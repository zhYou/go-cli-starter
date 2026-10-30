package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/diagnostics"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/base/telemetry"
)

func TestTraceWarningAndStderrLogsShareSerialization(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logging.Output = "stderr"
	cfg.Telemetry.Exporter = telemetry.File
	cfg.Telemetry.BatchSize = 1
	var sink bytes.Buffer
	var warnings atomic.Int32
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Directories: &paths.Directories{State: t.TempDir()}, Stderr: &sink}, func(err error) { _, _ = fmt.Fprintln(&sink, "测试警告", err); warnings.Add(1) })
	w, err := s.files.Writer(diagnostics.TraceFile)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	_, span := s.Tracer.Start(ctx, "failed-export")
	span.End()
	for range 10000 {
		s.Logger.Infow("业务日志")
		if warnings.Load() > 0 {
			break
		}
	}
	s.Finish(ctx, logging.CommandResult{})
	require.Equal(t, int32(1), warnings.Load())
	require.Equal(t, 1, strings.Count(sink.String(), "测试警告"))
	for line := range strings.SplitSeq(strings.TrimSpace(sink.String()), "\n") {
		if !strings.HasPrefix(line, "测试警告 ") {
			require.True(t, json.Valid([]byte(line)), "stderr 日志被并发写入破坏：%q", line)
		}
	}
}
