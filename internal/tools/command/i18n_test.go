package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

func TestCommandDefaultLocalizerUsesCentralPluralMessages(t *testing.T) {
	localizer := commandLocalizer(nil)
	definition := fault.Definition{Code: "cli.shell_count", Kind: fault.Usage}
	for _, tc := range []struct {
		count int
		want  string
	}{
		{1, "Expected at most one shell name, received 1 argument"},
		{2, "Expected at most one shell name, received 2 arguments"},
	} {
		err := definition.New(fault.WithParams(fault.Params{"Count": tc.count}))
		require.Equal(t, tc.want, localizer.Render(err))
	}
}

func TestCommandBrokenDiagnosticsPreserveChildFailure(t *testing.T) {
	bundle, err := messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte("[\"command.started\"]\nother = \"partial {{.Missing}}\"\n")}})
	require.NoError(t, err)
	var sink bytes.Buffer
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "debug", Output: "stderr"}, Writer: &sink}, func(err error) { t.Error(err) })
	call, options := helper(t, "exit")
	options.Logger, options.Localizer = logger, bundle.Localizer("en")
	err = Run(t.Context(), call, options)
	var exit *exec.ExitError
	require.True(t, errors.As(err, &exit))
	require.Equal(t, 7, exit.ExitCode())
	lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	var start, finish map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &start))
	require.NoError(t, json.Unmarshal(lines[1], &finish))
	require.Equal(t, "Operation failed [command.started]", start["msg"])
	require.Equal(t, "Operation failed [command.finished]", finish["msg"])
	require.Equal(t, "failed", finish["outcome"])
	require.Equal(t, float64(7), finish["exit_code"])
	require.NotContains(t, sink.String(), "partial")
}

func TestCommandUninitializedLocalizerUsesEnglishFallback(t *testing.T) {
	var sink bytes.Buffer
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "debug", Output: "stderr"}, Writer: &sink}, func(err error) { t.Error(err) })
	call, options := helper(t, "exit")
	options.Logger, options.Localizer = logger, &messages.Localizer{}
	err := Run(t.Context(), call, options)
	var exit *exec.ExitError
	require.True(t, errors.As(err, &exit))
	require.Equal(t, 7, exit.ExitCode())
	require.Contains(t, sink.String(), "Operation failed [command.started]")
	require.Contains(t, sink.String(), "Operation failed [command.finished]")
}
