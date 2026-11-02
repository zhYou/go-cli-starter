package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

var errTestWrite = errors.New("diagnostic write failed")

type failedDiagnosticWriter struct{}

func (failedDiagnosticWriter) Write([]byte) (int, error) { return 0, errTestWrite }

func TestCorruptCatalogProcess(t *testing.T) {
	if os.Getenv("TEST_CORRUPT_I18N_CATALOG") != "1" {
		t.Skip("仅由损坏目录子进程执行")
	}
	cfg := defaultConfig()
	cfg.Logging.Output = "stderr"
	var sink bytes.Buffer
	var warnings []error
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink}, func(err error) { warnings = append(warnings, err) })
	businessErr := (fault.Definition{Code: "example.business", Kind: fault.Runtime}).New()
	s.Finish(ctx, logging.CommandResult{ExitCode: 1, Err: businessErr})
	require.Len(t, warnings, 1)
	require.Error(t, warnings[0])
	lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	var start, finish map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &start))
	require.NoError(t, json.Unmarshal(lines[1], &finish))
	require.Equal(t, "Operation failed [session.started]", start["msg"])
	require.Equal(t, "Operation failed [session.finished]", finish["msg"])
	require.Equal(t, float64(1), finish["exit_code"])
	require.Equal(t, "example.business", finish["error_code"])

	warnings = nil
	ctx, s = Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: failedDiagnosticWriter{}}, func(err error) { warnings = append(warnings, err) })
	s.Finish(ctx, logging.CommandResult{})
	require.Len(t, warnings, 2, "目录加载失败不得吞掉后续日志写入故障")
	require.Contains(t, warnings[1].Error(), errTestWrite.Error())

	bundle, err := messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte("[\"session.started\"]\nother = \"Injected start\"\n[\"session.finished\"]\nother = \"Injected finish\"\n")}})
	require.NoError(t, err)
	sink.Reset()
	warnings = nil
	ctx, s = Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink, Localizer: bundle.Localizer("en")}, func(err error) { warnings = append(warnings, err) })
	s.Finish(ctx, logging.CommandResult{})
	require.Empty(t, warnings)
	require.Contains(t, sink.String(), "Injected start")
	require.Contains(t, sink.String(), "Injected finish")
}
