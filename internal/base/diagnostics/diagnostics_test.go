package diagnostics

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRotationPreservesWholeRecordsAndBoundsBackups(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	w, err := newWriter(root, EventsFile, 12, 2)
	require.NoError(t, err)
	for _, s := range []string{"first\n", "second\n", "third\n", "fourth\n"} {
		_, err = w.Write([]byte(s))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
	entries, err := os.ReadDir(root.Name())
	require.NoError(t, err)
	require.Len(t, entries, 3)
	var all bytes.Buffer
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(root.Name(), e.Name()))
		require.NoError(t, err)
		all.Write(b)
		info, err := e.Info()
		require.NoError(t, err)
		if runtime.GOOS != "windows" {
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		}
	}
	require.Contains(t, all.String(), "second\n")
	require.Contains(t, all.String(), "third\n")
	require.Contains(t, all.String(), "fourth\n")
	require.NotContains(t, all.String(), "first\n")
	_, err = w.Write([]byte("closed"))
	require.ErrorIs(t, err, os.ErrClosed)
}
func TestOversizeRecordIsNotSplit(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	w, err := newWriter(root, TraceFile, 4, 0)
	require.NoError(t, err)
	_, err = w.Write([]byte("oversized\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("{}\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	b, err := root.ReadFile("trace.000001.jsonl")
	require.NoError(t, err)
	require.Equal(t, "oversized\n", string(b))
}
func TestRetentionProtectsActiveAndUnknownFiles(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 1
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	writer, err := run.Writer(EventsFile)
	require.NoError(t, err)
	_, err = writer.Write([]byte(strings.Repeat("x", 2*1024*1024)))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(base, "legacy.jsonl"), []byte("legacy"), 0600))
	require.NoError(t, Cleanup(base, cfg))
	_, err = os.Stat(run.Path)
	require.NoError(t, err)
	require.NoError(t, run.Close())
	_, err = os.Stat(run.Path)
	require.ErrorIs(t, err, os.ErrNotExist)
	// #nosec G304 -- 仅读取本测试创建的临时目录文件。
	legacy, err := os.ReadFile(filepath.Join(base, "legacy.jsonl"))
	require.NoError(t, err)
	require.Equal(t, "legacy", string(legacy))
}
func TestExpiredRunAndUnknownContent(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	require.NoError(t, run.Close())
	old := time.Now().Add(-48 * time.Hour)
	meta := metadata{Version: 1, ID: run.ID, Started: old, Finished: old}
	require.NoError(t, writeMetadataPath(run.Path, meta))
	cfg.RetentionDays = 1
	require.NoError(t, Cleanup(base, cfg))
	_, err = os.Stat(run.Path)
	require.ErrorIs(t, err, os.ErrNotExist)
	unknown, err := Open(base, Defaults(), nil)
	require.NoError(t, err)
	require.NoError(t, unknown.Close())
	require.NoError(t, os.WriteFile(filepath.Join(unknown.Path, "user.txt"), []byte("keep"), 0600))
	require.NoError(t, writeMetadataPath(unknown.Path, metadata{Version: 1, ID: unknown.ID, Started: old, Finished: old}))
	require.NoError(t, Cleanup(base, cfg))
	_, err = os.Stat(filepath.Join(unknown.Path, "user.txt"))
	require.NoError(t, err)
}
func writeMetadataPath(path string, value metadata) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeMetadata(root, value)
}
func TestRetentionReclaimsPublishedRunWithInterruptedMetadataUpdate(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	require.NoError(t, run.Close())
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, writeMetadataPath(run.Path, metadata{Version: 1, ID: run.ID, Started: old, Finished: old}))
	require.NoError(t, os.WriteFile(filepath.Join(run.Path, metadataTemporaryName), []byte("{"), 0600))
	cfg.RetentionDays = 1
	require.NoError(t, Cleanup(base, cfg))
	_, err = os.Stat(run.Path)
	require.ErrorIs(t, err, os.ErrNotExist)
}
