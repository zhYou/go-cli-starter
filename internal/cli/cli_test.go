package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return invokeContext(t, context.Background(), args...)
}

func invokeContext(t *testing.T, ctx context.Context, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(ctx, args, Options{
		Name: "testcli", Version: "v1.2.3", Commit: "abc123", Date: "2026-10-28",
		Out: &out, ErrOut: &errOut, ConfigDir: t.TempDir(),
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	return code, out.String(), errOut.String()
}

func TestGreetOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"text", []string{"greet", "--name", "Codex"}, "Hello, Codex!\n"},
		{"json", []string{"greet", "--name", "Codex", "--json"}, "{\"message\":\"Hello, Codex!\"}\n"},
		{"unicode", []string{"--json", "greet", "--name", "周游"}, "{\"message\":\"Hello, 周游!\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := invoke(t, tc.args...)
			if code != 0 || out != tc.want || errOut != "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
			}
		})
	}
}

func TestInvalidArgumentsFailWithoutOutputOrPrompt(t *testing.T) {
	for _, args := range [][]string{
		{"greet"}, {"greet", "--name", ""}, {"greet", "--name", "x", "extra"},
		{"unknown"}, {"greet", "--unknown"}, {"--unknown", "--json"},
		{"completion", "bogus"}, {"completion", "bash", "extra"},
		{"help", "unknown"}, {"help", "greet", "unknown"},
	} {
		code, out, errOut := invoke(t, args...)
		if code != 2 || out != "" || strings.Count(errOut, "Error:") != 1 {
			t.Errorf("args=%v code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
}

func TestDebugLogsDoNotPolluteJSON(t *testing.T) {
	code, out, errOut := invoke(t, "greet", "--name", "Codex", "--json", "--log-level", "debug")
	if code != 0 || !json.Valid([]byte(out)) || !strings.Contains(errOut, "DEBUG") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestDiscoveryBypassesBrokenConfiguration(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("[broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{}, {"--help"}, {"greet", "--help"}, {"version", "--json"},
		{"--version"}, {"schema"}, {"help", "greet"},
		{"completion", "bash"}, {"completion", "zsh"},
		{"completion", "fish"}, {"completion", "powershell"},
	} {
		code, out, errOut := invoke(t, append([]string{"--config", bad}, args...)...)
		if code != 0 || out == "" || errOut != "" {
			t.Errorf("args=%v code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	code, out, errOut := invoke(t, "greet", "--name", "Codex", "--config", bad)
	if code != 1 || out != "" || errOut == "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestCancellationExitCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, out, errOut := invokeContext(t, ctx, "greet", "--name", "Codex")
	if code != 130 || out != "" || errOut == "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestVersionJSON(t *testing.T) {
	code, out, errOut := invoke(t, "version", "--json")
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || got["version"] != "v1.2.3" || got["commit"] != "abc123" || got["name"] != "testcli" || errOut != "" {
		t.Fatalf("code=%d result=%v stderr=%q", code, got, errOut)
	}
}

func TestSchemaReflectsRegisteredCommands(t *testing.T) {
	var out bytes.Buffer
	root := NewRoot(Options{Name: "testcli", Out: &out})
	extra := &cobra.Command{Use: "extra", Short: "An added command"}
	extra.Flags().String("token", "", "An example flag")
	if err := extra.MarkFlagRequired("token"); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(extra)
	root.SetArgs([]string{"schema", "extra"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		SchemaVersion int `json:"schema_version"`
		Commands      []struct {
			Path  string `json:"path"`
			Flags []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || len(got.Commands) != 1 || got.Commands[0].Path != "testcli extra" {
		t.Fatalf("schema=%s", out.String())
	}
	found := false
	for _, flag := range got.Commands[0].Flags {
		if flag.Name == "token" && flag.Required {
			found = true
		}
	}
	if !found {
		t.Fatalf("required flag missing: %s", out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestOutputFailureIsRuntimeError(t *testing.T) {
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"greet", "--name", "x"}, Options{
		Name: "testcli", Out: failingWriter{}, ErrOut: &errOut, ConfigDir: t.TempDir(),
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if code != 1 || !strings.Contains(errOut.String(), "output unavailable") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}
