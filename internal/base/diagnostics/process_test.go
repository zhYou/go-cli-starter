package diagnostics

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const helperRootEnv = "GO_CLI_STARTER_DIAGNOSTICS_TEST_ROOT"

func TestDiagnosticLeaseHelper(t *testing.T) {
	base := os.Getenv(helperRootEnv)
	if base == "" {
		return
	}
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	w, err := run.Writer(EventsFile)
	require.NoError(t, err)
	_, err = w.Write([]byte(strings.Repeat("x", 2*1024*1024)))
	require.NoError(t, err)
	fmt.Println(run.Path)
	_, _ = io.Copy(io.Discard, os.Stdin)
	require.NoError(t, run.Close())
}
func TestDiagnosticCleanupHelper(t *testing.T) {
	base := os.Getenv(helperRootEnv)
	if base == "" {
		return
	}
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 1
	require.NoError(t, Cleanup(base, cfg))
}
func TestProcessesProtectLiveRunAndReclaimKilledWriter(t *testing.T) {
	base := t.TempDir()
	exe, err := os.Executable()
	require.NoError(t, err)
	// #nosec G204 -- 仅执行当前测试二进制的专用子进程。
	child := exec.CommandContext(t.Context(), exe, "-test.run=^TestDiagnosticLeaseHelper$")
	child.Env = append(os.Environ(), helperRootEnv+"="+base)
	input, err := child.StdinPipe()
	require.NoError(t, err)
	output, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = input.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	line, err := bufio.NewReader(output).ReadString('\n')
	require.NoError(t, err)
	path := strings.TrimSpace(line)
	require.Equal(t, base, filepath.Dir(path))
	var cleaners []*exec.Cmd
	for range 3 {
		// #nosec G204 -- 仅执行当前测试二进制的专用清理子进程。
		cleaner := exec.CommandContext(t.Context(), exe, "-test.run=^TestDiagnosticCleanupHelper$")
		cleaner.Env = append(os.Environ(), helperRootEnv+"="+base)
		require.NoError(t, cleaner.Start())
		cleaners = append(cleaners, cleaner)
	}
	for _, cleaner := range cleaners {
		require.NoError(t, cleaner.Wait())
	}
	_, err = os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 1
	require.NoError(t, Cleanup(base, cfg))
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}
func TestRetentionDoesNotFollowArtifactSymlinks(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	require.NoError(t, run.Close())
	victim := filepath.Join(t.TempDir(), "keep")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0600))
	if err := os.Symlink(victim, filepath.Join(run.Path, EventsFile)); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("当前 Windows 主机无法创建符号链接：%v", err)
		}
		require.NoError(t, err)
	}
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, writeMetadataPath(run.Path, metadata{Version: 1, ID: run.ID, Started: old, Finished: old}))
	cfg.RetentionDays = 1
	require.NoError(t, Cleanup(base, cfg))
	// #nosec G304 -- 仅读取本测试创建的链接目标，验证它未被清理器修改。
	b, err := os.ReadFile(victim)
	require.NoError(t, err)
	require.Equal(t, "keep", string(b))
	_, err = os.Lstat(filepath.Join(run.Path, EventsFile))
	require.NoError(t, err)
}
func TestInterruptedCleanupDoesNotOrphanData(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	run, err := Open(base, cfg, nil)
	require.NoError(t, err)
	require.NoError(t, run.Close())
	root, err := os.OpenRoot(run.Path)
	require.NoError(t, err)
	for i := 1; i <= 4096; i++ {
		name := fmt.Sprintf("events.%06d.jsonl", i)
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		require.NoError(t, err)
		_, err = file.Write([]byte(strings.Repeat("x", 512)))
		require.NoError(t, err)
		require.NoError(t, file.Close())
	}
	require.NoError(t, root.Close())
	exe, err := os.Executable()
	require.NoError(t, err)
	// #nosec G204 -- 仅执行当前测试二进制的清理 helper。
	child := exec.CommandContext(t.Context(), exe, "-test.run=^TestDiagnosticCleanupHelper$")
	child.Env = append(os.Environ(), helperRootEnv+"="+base)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err = os.Stat(filepath.Join(run.Path, metadataName))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("清理未移除元数据")
		}
		time.Sleep(100 * time.Microsecond)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	require.NoError(t, Cleanup(base, Defaults()))
	entries, err := os.ReadDir(run.Path)
	if os.IsNotExist(err) {
		return
	}
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, artifactPattern.MatchString(entry.Name()), "遗留诊断数据："+entry.Name())
	}
}
