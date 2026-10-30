package session

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/require"
	appconfig "github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/base/diagnostics"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/base/telemetry"
)

func defaultConfig() appconfig.Config {
	return appconfig.Config{Logging: logging.Defaults(), Diagnostics: diagnostics.Defaults(), Telemetry: telemetry.Defaults()}
}

type countingSink struct {
	bytes.Buffer
	syncs int
}

func (s *countingSink) Sync() error { s.syncs++; return nil }

func TestBorrowedLoggerRemainsUsableWithoutSyncOrDirectories(t *testing.T) {
	home := filepath.Join(t.TempDir(), "unused")
	t.Setenv("GO_CLI_STARTER_HOME", home)
	sink := &countingSink{}
	external := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Defaults(), Writer: sink, RunID: "external"}, nil)
	ctx, s := Begin(t.Context(), defaultConfig(), logging.RunInfo{Component: "go-cli-starter", Command: "doctor"}, Options{ExternalLogger: external}, func(err error) { t.Error(err) })
	s.Finish(ctx, logging.CommandResult{})
	s.Finish(ctx, logging.CommandResult{})
	require.Zero(t, sink.syncs)
	_, err := os.Stat(home)
	require.ErrorIs(t, err, os.ErrNotExist)
	sink.Reset()
	external.Infow("still_usable")
	require.Contains(t, sink.String(), "still_usable")
	require.NotContains(t, sink.String(), "doctor")
}

func TestConcurrentSessionsOwnPrivateFilesAndFinishOnce(t *testing.T) {
	t.Setenv("GO_CLI_STARTER_HOME", t.TempDir())
	var runPaths sync.Map
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			ctx, s := Begin(t.Context(), defaultConfig(), logging.RunInfo{Component: "go-cli-starter", Command: "doctor"}, Options{Paths: paths.Options{AppName: "go-cli-starter"}}, func(err error) { t.Error(err) })
			path := s.files.Path
			if _, duplicate := runPaths.LoadOrStore(path, true); duplicate {
				t.Error("运行目录重复")
			}
			s.Finish(ctx, logging.CommandResult{})
			s.Finish(ctx, logging.CommandResult{})
		})
	}
	group.Wait()
	runPaths.Range(func(key, _ any) bool {
		path := key.(string)
		info, err := os.Stat(path)
		require.NoError(t, err)
		if runtime.GOOS != "windows" {
			require.Equal(t, os.FileMode(0700), info.Mode().Perm())
		}
		file := filepath.Join(path, diagnostics.EventsFile)
		info, err = os.Stat(file)
		require.NoError(t, err)
		if runtime.GOOS != "windows" {
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		}
		// #nosec G304 -- 路径仅来自本测试创建并登记的私有诊断目录。
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Equal(t, 1, bytes.Count(data, []byte("command_finished")))
		// #nosec G304 -- 仅回读本测试诊断目录中的固定元数据文件。
		data, err = os.ReadFile(filepath.Join(path, ".run.json"))
		require.NoError(t, err)
		var metadata map[string]any
		require.NoError(t, json.Unmarshal(data, &metadata))
		require.Contains(t, metadata, "finished")
		require.NotEqual(t, "0001-01-01T00:00:00Z", metadata["finished"])
		lock := flock.New(filepath.Join(path, ".lock"))
		locked, err := lock.TryLock()
		require.NoError(t, err)
		require.True(t, locked)
		require.NoError(t, lock.Close())
		return true
	})
}

func TestWriteAndFinishFailuresWarnOnce(t *testing.T) {
	t.Setenv("GO_CLI_STARTER_HOME", t.TempDir())
	var warnings atomic.Int32
	ctx, s := Begin(t.Context(), defaultConfig(), logging.RunInfo{}, Options{Paths: paths.Options{AppName: "go-cli-starter"}}, func(error) { warnings.Add(1) })
	writer, err := s.files.Writer(diagnostics.EventsFile)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	s.Logger.Infow("cannot write")
	s.Logger.Errorw("still cannot write")
	s.Finish(ctx, logging.CommandResult{})
	require.Equal(t, int32(1), warnings.Load())
}

func TestUnavailableWriterDegradesWithoutTypedNil(t *testing.T) {
	files, err := diagnostics.Open(t.TempDir(), diagnostics.Defaults(), nil)
	require.NoError(t, err)
	require.NoError(t, files.Close())
	s := &Session{files: files}
	warnings := 0
	warn := reporter(loggingFailed, func(error) { warnings++ })
	writer := s.writer(diagnostics.EventsFile, warn)
	require.Nil(t, writer)
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Defaults(), Writer: writer, RunID: files.ID}, warn)
	logger.Started()
	logger.Record(logging.CommandResult{ExitCode: 0})
	require.NoError(t, logger.Sync())
	require.Equal(t, 1, warnings)
}

func TestFinishWaitsForRegisteredWorkBeforeEndingSpanAndClosingFiles(t *testing.T) {
	cfg := defaultConfig()
	cfg.Telemetry.Exporter = telemetry.File
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{Component: "test", Command: "work"}, Options{Directories: &paths.Directories{State: t.TempDir()}}, func(err error) { t.Error(err) })
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	require.True(t, s.Go(func(ctx context.Context) {
		_, span := s.Tracer.Start(ctx, "worker")
		defer span.End()
		close(started)
		<-release
		s.Logger.WithContext(ctx).Infow("worker_completed")
	}))
	<-started
	go func() { s.Finish(ctx, logging.CommandResult{}); close(finished) }()
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.finishing }, time.Second, time.Millisecond)
	require.False(t, s.Go(func(context.Context) { t.Error("关闭后不应接受新工作") }))
	select {
	case <-finished:
		t.Fatal("业务尚未结束就关闭资源")
	default:
	}
	require.True(t, trace.SpanFromContext(ctx).IsRecording())
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("工作结束后未完成清理")
	}
	require.False(t, trace.SpanFromContext(ctx).IsRecording())
	data, err := os.ReadFile(filepath.Join(s.files.Path, diagnostics.EventsFile))
	require.NoError(t, err)
	require.Less(t, bytes.Index(data, []byte("worker_completed")), bytes.Index(data, []byte("command_finished")))
	spans, err := os.ReadFile(filepath.Join(s.files.Path, diagnostics.TraceFile))
	require.NoError(t, err)
	require.Contains(t, string(spans), "worker")
}

func TestStderrIsBorrowedAndNeverSyncedOrClosed(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logging.Output = "stderr"
	sink := &countingSink{}
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: sink}, func(err error) { t.Error(err) })
	s.Finish(ctx, logging.CommandResult{})
	require.Zero(t, sink.syncs)
	require.Contains(t, sink.String(), "command_finished")
	require.Nil(t, s.files)
}
