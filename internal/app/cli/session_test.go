package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBusinessCreatesPersistentEvents(t *testing.T) {
	home := t.TempDir()
	var out, stderr bytes.Buffer
	code := Run(context.Background(), []string{"greet", "--name", "Codex", "--json"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(key string) (string, bool) {
		if key == "GO_CLI_STARTER_HOME" {
			return home, true
		}
		return "", false
	}})
	require.Zero(t, code, stderr.String())
	require.JSONEq(t, `{"message":"你好，Codex！"}`, out.String())
	files, err := filepath.Glob(filepath.Join(home, "state", "logs", "*", "events.jsonl"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "command_started")
	require.Contains(t, string(data), "command_finished")
}

func TestDiscoveryAndInvalidCallsCreateNoState(t *testing.T) {
	for _, args := range [][]string{{"greet"}, {"greet", "--name", " "}, {"greet", "--name", "x", "--log-level", "bad"}, {"help"}, {"version"}, {"schema"}, {"completion", "bash"}, {"__complete", "greet", ""}} {
		t.Run(args[0]+"-"+args[len(args)-1], func(t *testing.T) {
			home := t.TempDir()
			var out, stderr bytes.Buffer
			Run(context.Background(), args, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(key string) (string, bool) {
				if key == "GO_CLI_STARTER_HOME" {
					return home, true
				}
				return "", false
			}})
			_, err := os.Stat(filepath.Join(home, "state"))
			require.True(t, os.IsNotExist(err), "%v", err)
		})
	}
}

func TestFileTraceCorrelatesActualCommandAndBusinessLogs(t *testing.T) {
	home := t.TempDir()
	var out, stderr bytes.Buffer
	env := map[string]string{"GO_CLI_STARTER_HOME": home, "GO_CLI_STARTER_TELEMETRY_EXPORTER": "file", "GO_CLI_STARTER_LOG_LEVEL": "debug"}
	code := Run(t.Context(), []string{"greet", "--name", "private-name", "--json", "--lang", "en"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok }})
	require.Zero(t, code, stderr.String())
	require.Empty(t, stderr.String())
	files, err := filepath.Glob(filepath.Join(home, "state", "logs", "*", "events.jsonl"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	logs, err := os.ReadFile(files[0])
	require.NoError(t, err)
	traces, err := os.ReadFile(filepath.Join(filepath.Dir(files[0]), "trace.jsonl"))
	require.NoError(t, err)
	require.NotContains(t, string(logs), "private-name")
	require.NotContains(t, string(traces), "private-name")
	spans := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(traces), []byte("\n")) {
		var row map[string]any
		require.NoError(t, json.Unmarshal(line, &row))
		spans[row["Name"].(string)] = row
	}
	require.Len(t, spans, 2)
	business := spans["greeting.run"]
	require.NotNil(t, business)
	businessContext := business["SpanContext"].(map[string]any)
	var rootContext map[string]any
	for name, row := range spans {
		if name != "greeting.run" {
			rootContext = row["SpanContext"].(map[string]any)
		}
	}
	require.Equal(t, rootContext["TraceID"], businessContext["TraceID"])
	require.Equal(t, rootContext["SpanID"], business["Parent"].(map[string]any)["SpanID"])
	events := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte("\n")) {
		var row map[string]any
		require.NoError(t, json.Unmarshal(line, &row))
		events[row["event"].(string)] = row
	}
	require.Equal(t, "Command started", events["command_started"]["msg"])
	require.Equal(t, rootContext["TraceID"], events["command_started"]["trace_id"])
	require.Equal(t, rootContext["SpanID"], events["command_finished"]["span_id"])
	require.Equal(t, businessContext["SpanID"], events["greeting_finished"]["span_id"])
}

func TestLogAndTraceOutputsAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		log, trace string
		files      []string
	}{{"none", "none", nil}, {"stderr", "none", nil}, {"none", "file", []string{"trace.jsonl"}}, {"file", "none", []string{"events.jsonl"}}} {
		t.Run(tc.log+"-"+tc.trace, func(t *testing.T) {
			home := t.TempDir()
			var out, stderr bytes.Buffer
			env := map[string]string{"GO_CLI_STARTER_HOME": home, "GO_CLI_STARTER_LOGGING_OUTPUT": tc.log, "GO_CLI_STARTER_TELEMETRY_EXPORTER": tc.trace}
			code := Run(t.Context(), []string{"greet", "--name", "x"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok }})
			require.Zero(t, code, stderr.String())
			files, err := filepath.Glob(filepath.Join(home, "state", "logs", "*", "*.jsonl"))
			require.NoError(t, err)
			require.Len(t, files, len(tc.files))
			for i, file := range files {
				require.Equal(t, tc.files[i], filepath.Base(file))
			}
			if tc.log == "stderr" {
				require.Contains(t, stderr.String(), "command_started")
			} else {
				require.Empty(t, stderr.String())
			}
		})
	}
}

func TestConfigFailureCreatesNoDiagnostics(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.toml"), []byte("[logging]\noutput='invalid'\n"), 0600))
	var out, stderr bytes.Buffer
	code := Run(t.Context(), []string{"greet", "--name", "x"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(k string) (string, bool) { return home, k == "GO_CLI_STARTER_HOME" }})
	require.Equal(t, 1, code)
	require.Empty(t, out.String())
	_, err := os.Stat(filepath.Join(home, "state"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDiagnosticFailurePreservesBusinessResultAndWarnsOncePerSubsystem(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "state"), []byte("blocked"), 0600))
	var out, stderr bytes.Buffer
	code := Run(t.Context(), []string{"greet", "--name", "x", "--json", "--lang", "en"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: func(k string) (string, bool) { return home, k == "GO_CLI_STARTER_HOME" }})
	require.Zero(t, code)
	require.JSONEq(t, `{"message":"Hello, x!"}`, out.String())
	require.Equal(t, 1, strings.Count(stderr.String(), "Diagnostic files unavailable"))
	require.Equal(t, 1, strings.Count(stderr.String(), "Logging diagnostics unavailable"))
}

func TestCanceledBusinessStillFlushesTrace(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var out, stderr bytes.Buffer
	env := map[string]string{"GO_CLI_STARTER_HOME": home, "GO_CLI_STARTER_TELEMETRY_EXPORTER": "file"}
	deps := Dependencies{Greet: func(ctx context.Context, _ greeting.Options) (greeting.Result, error) {
		cancel()
		return greeting.Result{}, ctx.Err()
	}}
	code := Run(ctx, []string{"greet", "--name", "x", "--json"}, Options{Out: &out, ErrOut: &stderr, Dependencies: &deps, LookupEnv: func(k string) (string, bool) { v, ok := env[k]; return v, ok }})
	require.Equal(t, 130, code)
	require.Empty(t, out.String())
	files, err := filepath.Glob(filepath.Join(home, "state", "logs", "*", "trace.jsonl"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Contains(t, string(data), "greeting.run")
	require.Contains(t, string(data), "Error")
	logs, err := os.ReadFile(filepath.Join(filepath.Dir(files[0]), "events.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(logs), `"exit_code":130`)
}
