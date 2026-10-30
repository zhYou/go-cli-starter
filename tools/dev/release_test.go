package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func releaseFixture(t *testing.T) string {
	t.Helper()
	return releaseFixtureMode(t, "", "")
}

func releaseFixtureMode(t *testing.T, selected, mode string) string {
	t.Helper()
	dir := t.TempDir()
	var sums strings.Builder
	for _, target := range []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64", "windows_arm64"} {
		ext := ".tar.gz"
		if strings.HasPrefix(target, "windows") {
			ext = ".zip"
		}
		name := fmt.Sprintf("%s_0.1.0_%s%s", appName, target, ext)
		doc := map[string]any{"spdxVersion": "SPDX-2.3", "SPDXID": "SPDXRef-DOCUMENT", "name": name, "documentNamespace": "https://example.com/" + name, "creationInfo": map[string]any{"creators": []string{"Tool: syft"}, "created": "2026-10-29T00:00:00Z"}, "packages": []any{map[string]any{"SPDXID": "SPDXRef-root", "name": name}}}
		sbom, err := json.Marshal(doc)
		require.NoError(t, err)
		binary := appName
		windows := strings.HasPrefix(target, "windows")
		if windows {
			binary += ".exe"
		}
		archiveMode := "valid"
		if target == selected {
			archiveMode = mode
		}
		archive := releaseArchiveBytes(t, windows, binary, archiveMode)
		for file, data := range map[string][]byte{name: archive, name + ".spdx.json": sbom} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, file), data, 0600))
			sum := sha256.Sum256(data)
			fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), file)
		}
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sums.String()), 0600))
	return dir
}
func TestReleaseSnapshotRejectsMissingOrMismatchedSBOM(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "hash", "duplicate", "escape", "wrong-document", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			dir := releaseFixture(t)
			sumPath := filepath.Join(dir, "checksums.txt")
			//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			data, err := os.ReadFile(sumPath)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			var sbom string
			for _, line := range lines {
				if strings.HasSuffix(line, ".spdx.json") {
					sbom = strings.SplitN(line, "  ", 2)[1]
					break
				}
			}
			switch mode {
			case "missing":
				//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
				require.NoError(t, os.Remove(filepath.Join(dir, sbom)))
			case "hash":
				//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
				require.NoError(t, os.WriteFile(filepath.Join(dir, sbom), []byte("{}"), 0600))
			case "duplicate":
				data = append(data, []byte(lines[0]+"\n")...)
			case "escape":
				data = append(data, []byte(strings.Repeat("0", 64)+"  ../outside\n")...)
			case "wrong-document":
				raw, _ := json.Marshal(map[string]any{"spdxVersion": "SPDX-2.3", "name": "wrong"})
				//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
				require.NoError(t, os.WriteFile(filepath.Join(dir, sbom), raw, 0600))
				sum := sha256.Sum256(raw)
				for i, line := range lines {
					if strings.HasSuffix(line, "  "+sbom) {
						lines[i] = hex.EncodeToString(sum[:]) + "  " + sbom
					}
				}
				data = []byte(strings.Join(lines, "\n") + "\n")
			case "symlink":
				outside := filepath.Join(t.TempDir(), "outside")
				//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
				raw, err := os.ReadFile(filepath.Join(dir, sbom))
				require.NoError(t, err)
				//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
				require.NoError(t, os.WriteFile(outside, raw, 0600))
				//nolint:gosec // G703：测试路径和权限只来自当前临时 fixture，不读取用户文件。
				require.NoError(t, os.Remove(filepath.Join(dir, sbom)))
				if err := os.Symlink(outside, filepath.Join(dir, sbom)); err != nil {
					t.Skipf("符号链接不可用：%v", err)
				}
			}
			//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			require.NoError(t, os.WriteFile(sumPath, data, 0600))
			err = checkReleaseSnapshot(dir)
			if mode == "valid" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
