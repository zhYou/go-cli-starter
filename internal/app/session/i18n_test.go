package session

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"github.com/zhYou/go-cli-starter/internal/locales"
)

func TestSessionUsesDefaultEnglishAndExplicitChinese(t *testing.T) {
	bundle, err := locales.LoadBundle()
	require.NoError(t, err)
	cases := []struct {
		name          string
		localizer     *messages.Localizer
		start, finish string
	}{
		{"default", nil, "Command started", "Command finished"},
		{"chinese", bundle.Localizer("zh-CN"), "命令开始", "命令结束"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.Logging.Output = "stderr"
			var sink bytes.Buffer
			ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink, Localizer: tc.localizer}, func(err error) { t.Error(err) })
			s.Finish(ctx, logging.CommandResult{})
			lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
			require.Len(t, lines, 2)
			var start, finish map[string]any
			require.NoError(t, json.Unmarshal(lines[0], &start))
			require.NoError(t, json.Unmarshal(lines[1], &finish))
			require.Equal(t, tc.start, start["msg"])
			require.Equal(t, tc.finish, finish["msg"])
			require.Equal(t, "command_finished", finish["event"])
		})
	}
}

func TestSessionBrokenDiagnosticsPreserveBusinessFailure(t *testing.T) {
	bundle, err := messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte("[\"session.started\"]\nother = \"partial {{.Missing}}\"\n")}})
	require.NoError(t, err)
	cfg := defaultConfig()
	cfg.Logging.Output = "stderr"
	var sink bytes.Buffer
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink, Localizer: bundle.Localizer("en")}, func(err error) { t.Error(err) })
	businessErr := (fault.Definition{Code: "example.business", Kind: fault.Runtime}).New()
	s.Finish(ctx, logging.CommandResult{ExitCode: 1, Err: businessErr})
	lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	var start, finish map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &start))
	require.NoError(t, json.Unmarshal(lines[1], &finish))
	require.Equal(t, "Operation failed [session.started]", start["msg"])
	require.Equal(t, "Operation failed [session.finished]", finish["msg"])
	require.Equal(t, float64(1), finish["exit_code"])
	require.Equal(t, "example.business", finish["error_code"])
	require.NotContains(t, sink.String(), "partial")
}

func TestSessionConcurrentLocalizersRemainIsolated(t *testing.T) {
	bundle, err := locales.LoadBundle()
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 20 {
		for _, tc := range []struct{ lang, want string }{{"en", "Command finished"}, {"zh-CN", "命令结束"}} {
			wg.Go(func() {
				cfg := defaultConfig()
				cfg.Logging.Output = "stderr"
				var sink bytes.Buffer
				ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink, Localizer: bundle.Localizer(tc.lang)}, func(err error) { t.Error(err) })
				s.Finish(ctx, logging.CommandResult{})
				if !bytes.Contains(sink.Bytes(), []byte(tc.want)) {
					t.Errorf("%s: %s", tc.lang, sink.Bytes())
				}
			})
		}
	}
	wg.Wait()
}

func TestSessionUninitializedLocalizerUsesEnglishFallback(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logging.Output = "stderr"
	var sink bytes.Buffer
	ctx, s := Begin(t.Context(), cfg, logging.RunInfo{}, Options{Stderr: &sink, Localizer: &messages.Localizer{}}, func(err error) { t.Error(err) })
	s.Finish(ctx, logging.CommandResult{})
	require.Contains(t, sink.String(), "Operation failed [session.started]")
	require.Contains(t, sink.String(), "Operation failed [session.finished]")
}
