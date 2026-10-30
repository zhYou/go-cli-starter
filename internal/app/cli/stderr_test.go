package cli

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
)

type overlappingStderr struct {
	bytes.Buffer
	entered, release                chan struct{}
	active, overlaps, syncs, closes atomic.Int32
}

func (s *overlappingStderr) Write(p []byte) (int, error) {
	if s.active.Add(1) > 1 {
		s.overlaps.Add(1)
	}
	defer s.active.Add(-1)
	if bytes.Contains(p, []byte("测试告警")) {
		close(s.entered)
		<-s.release
	}
	return s.Buffer.Write(p)
}
func (s *overlappingStderr) Sync() error  { s.syncs.Add(1); return nil }
func (s *overlappingStderr) Close() error { s.closes.Add(1); return nil }

func TestCLIErrorAndDiagnosticWarningShareStderrSerialization(t *testing.T) {
	sink := &overlappingStderr{entered: make(chan struct{}), release: make(chan struct{})}
	var out bytes.Buffer
	var warn func(error)
	warningDone := make(chan struct{})
	deps := Dependencies{
		LoadConfig: func(config.Options) (config.Config, error) {
			cfg := config.Defaults()
			cfg.Logging.Output = "stderr"
			return cfg, nil
		},
		NewLogger: func(info logging.RunInfo, opts logging.Options, report func(error)) *logging.Logger {
			warn = report
			return logging.New(info, opts, report)
		},
		Greet: func(context.Context, greeting.Options) (greeting.Result, error) {
			go func() { defer close(warningDone); warn(errors.New("测试告警")) }()
			<-sink.entered
			// 告警保持进行时返回业务错误；独立定时器释放它，避免将错误写入成功作为同步条件。
			time.AfterFunc(25*time.Millisecond, func() { close(sink.release) })
			return greeting.Result{}, errors.New("测试业务失败")
		},
	}
	code := Run(t.Context(), []string{"greet", "--name", "x"}, Options{Out: &out, ErrOut: sink, Dependencies: &deps, Directories: &paths.Directories{State: t.TempDir()}, LookupEnv: func(string) (string, bool) { return "", false }})
	<-warningDone
	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	require.Zero(t, sink.overlaps.Load(), "业务错误输出与诊断告警并发调用底层 stderr")
	require.Zero(t, sink.syncs.Load())
	require.Zero(t, sink.closes.Load())
	require.Contains(t, sink.String(), "测试业务失败")
	require.Contains(t, sink.String(), "测试告警")
}
