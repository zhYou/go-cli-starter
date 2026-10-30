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

// TestDiagnosticInterruptedCleanupHelper 在首次真实数据删除后握手，持锁等待父进程终止。
func TestDiagnosticInterruptedCleanupHelper(t *testing.T) {
	base := os.Getenv(helperRootEnv)
	if base == "" {
		return
	}
	cfg := Defaults()
	cfg.RetentionDays = 1
	cfg.MaxTotalSizeMiB = 0
	require.NoError(t, cleanup(base, cfg, func(root *os.Root, name string) error {
		if err := root.Remove(name); err != nil {
			return err
		}
		if artifactPattern.MatchString(name) {
			fmt.Println(name)
			_, _ = io.Copy(io.Discard, os.Stdin)
		}
		return nil
	}))
}

func TestInterruptedCleanupDoesNotOrphanData(t *testing.T) {
	base := t.TempDir()
	cfg := Defaults()
	cfg.RetentionDays = 0
	cfg.MaxTotalSizeMiB = 0
	create := func() *Run {
		run, err := Open(base, cfg, nil)
		require.NoError(t, err)
		for _, name := range []string{EventsFile, "events.000001.jsonl", "trace.000001.jsonl"} {
			require.NoError(t, os.WriteFile(filepath.Join(run.Path, name), []byte("{}\n"), 0600))
		}
		return run
	}
	expired := create()
	require.NoError(t, expired.Close())
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, writeMetadataPath(expired.Path, metadata{Version: metadataVersion, ID: expired.ID, Started: old, Finished: old}))
	active := create()
	defer func() { require.NoError(t, active.Close()) }()
	unknown := create()
	require.NoError(t, unknown.Close())
	marker := filepath.Join(unknown.Path, "user.txt")
	require.NoError(t, os.WriteFile(marker, []byte("保留"), 0600))
	require.NoError(t, writeMetadataPath(unknown.Path, metadata{Version: metadataVersion, ID: unknown.ID, Started: old, Finished: old}))
	// 活动目录也过期，确保保留依赖租约而非时间筛选。
	require.NoError(t, writeMetadataPath(active.Path, metadata{Version: metadataVersion, ID: active.ID, Started: old, Finished: old}))
	exe, err := os.Executable()
	require.NoError(t, err)
	// #nosec G204 -- 仅执行当前测试二进制的清理 helper。
	child := exec.CommandContext(t.Context(), exe, "-test.run=^TestDiagnosticInterruptedCleanupHelper$")
	child.Env = append(os.Environ(), helperRootEnv+"="+base)
	input, err := child.StdinPipe()
	require.NoError(t, err)
	output, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = input.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- strings.TrimSpace(line) }()
	var removed string
	select {
	case removed = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("清理子进程未完成首次数据删除握手")
	}
	require.True(t, artifactPattern.MatchString(removed), "握手必须指明已删除的数据文件")
	require.NoFileExists(t, filepath.Join(expired.Path, removed))
	require.FileExists(t, filepath.Join(expired.Path, metadataName))
	entries, err := os.ReadDir(expired.Path)
	require.NoError(t, err)
	remaining := 0
	for _, entry := range entries {
		if artifactPattern.MatchString(entry.Name()) {
			remaining++
		}
	}
	require.Equal(t, 2, remaining, "中断前必须还剩两个真实数据文件")
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	cfg.RetentionDays = 1
	require.NoError(t, Cleanup(base, cfg))
	require.NoDirExists(t, expired.Path)
	for _, name := range []string{EventsFile, "events.000001.jsonl", "trace.000001.jsonl", metadataName} {
		require.FileExists(t, filepath.Join(active.Path, name))
		require.FileExists(t, filepath.Join(unknown.Path, name))
	}
	//nolint:gosec // G304：仅读取本测试临时目录的未知文件。
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "保留", string(data))
}
