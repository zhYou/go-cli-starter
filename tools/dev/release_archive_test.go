package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type archiveMember struct {
	name string
	data []byte
	mode os.FileMode
}

func releaseArchiveBytes(t *testing.T, windows bool, binary, mode string) []byte {
	t.Helper()
	if mode == "corrupt" {
		return []byte("损坏归档")
	}
	members := []archiveMember{{binary, []byte("测试二进制"), 0755}, {"README.md", []byte("说明"), 0644}, {"config.example.yaml", []byte("log_level: warn\n"), 0644}}
	switch mode {
	case "binary":
		members = members[1:]
	case "readme":
		members = append(members[:1], members[2:]...)
	case "config":
		members = members[:2]
	case "empty":
		members[2].data = nil
	case "parent":
		members[2].name = "configs/config.example.yaml"
	case "duplicate":
		members = append(members, members[2])
	case "symlink":
		members[2].mode = os.ModeSymlink | 0644
	case "dot-alias":
		members = append(members, archiveMember{"./config.example.yaml", nil, 0644})
	case "parent-alias":
		members = append(members, archiveMember{"sub/../config.example.yaml", nil, 0644})
	case "windows-alias":
		members = append(members, archiveMember{`.\config.example.yaml`, nil, 0644})
	}
	var out bytes.Buffer
	if windows {
		writer := zip.NewWriter(&out)
		for _, member := range members {
			header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
			header.SetMode(member.mode)
			file, err := writer.CreateHeader(header)
			require.NoError(t, err)
			_, err = file.Write(member.data)
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
	} else {
		compressed := gzip.NewWriter(&out)
		writer := tar.NewWriter(compressed)
		for _, member := range members {
			header := &tar.Header{Name: member.name, Mode: int64(member.mode.Perm()), Size: int64(len(member.data)), Typeflag: tar.TypeReg}
			if member.mode&os.ModeSymlink != 0 {
				header.Typeflag = tar.TypeSymlink
				header.Linkname = "outside"
				header.Size = 0
			}
			require.NoError(t, writer.WriteHeader(header))
			if header.Typeflag == tar.TypeReg {
				_, err := writer.Write(member.data)
				require.NoError(t, err)
			}
		}
		require.NoError(t, writer.Close())
		require.NoError(t, compressed.Close())
	}
	return out.Bytes()
}

func TestReleaseArchiveContents(t *testing.T) {
	for _, target := range []string{"linux_amd64", "windows_amd64"} {
		for _, mode := range []string{"valid", "binary", "readme", "config", "empty", "parent", "duplicate", "corrupt", "symlink", "dot-alias", "parent-alias", "windows-alias"} {
			t.Run(target+"/"+mode, func(t *testing.T) {
				dir := releaseFixtureMode(t, target, mode)
				err := checkReleaseSnapshot(dir)
				if mode == "valid" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}
