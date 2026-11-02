package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletionCandidatesAndQuietProtocol(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"language", []string{"__complete", "--lang", ""}, []string{"en\n", "zh-CN\n", "zh\n", ":4\n"}},
		{"level", []string{"__complete", "--log-level", ""}, []string{"debug\n", "info\n", "warn\n", "error\n", ":4\n"}},
		{"help", []string{"__complete", "help", ""}, []string{"completion\t", "greet\t", "version\t", ":4\n"}},
		{"nested", []string{"__complete", "help", "completion", ""}, []string{"bash\t", "zsh\t", "fish\t", "powershell\t", ":4\n"}},
		{"partial", []string{"__complete", "--lang", "e", "--log-level", ""}, []string{"debug\n", ":4\n"}},
		{"partial-equals", []string{"__complete", "--lang=e", "--log-level", ""}, []string{"debug\n", ":4\n"}},
		{"no-description", []string{"__completeNoDesc", "help", "completion", ""}, []string{"bash\n", "powershell\n", ":4\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, stderr := invoke(t, tc.args...)
			require.Zero(t, code, stderr)
			require.Empty(t, stderr)
			for _, want := range tc.want {
				require.Contains(t, out, want)
			}
			if tc.name == "no-description" {
				require.NotContains(t, out, "\t")
			}
		})
	}
}

func TestCompletionLanguageFallbackAndOrdinaryValidation(t *testing.T) {
	for _, tc := range []struct {
		env  string
		args []string
		want string
	}{
		{"invalid", []string{"__complete", ""}, "version\tShow"},
		{"zh-CN", []string{"__complete", "--lang=e", ""}, "version\t显示"},
		{"en", []string{"__complete", "--lang", "zh", "--log-level", ""}, "info\n"},
		{"en", []string{"__complete", "--lang", "zh", ""}, "version\t显示"},
	} {
		var out, stderr bytes.Buffer
		code := Run(context.Background(), tc.args, Options{Name: "sample", Out: &out, ErrOut: &stderr, LookupEnv: func(key string) (string, bool) { return tc.env, key == "SAMPLE_LANG" }})
		require.Zero(t, code, stderr.String())
		require.Empty(t, stderr.String())
		require.Contains(t, out.String(), tc.want)
	}
	for _, args := range [][]string{{"--lang=e", "version"}, {"--config", "__complete", "--lang=e", "version"}, {"greet", "--name", "__complete", "--lang=e"}, {"--", "__complete", "--lang=e"}} {
		code, out, stderr := invoke(t, args...)
		require.Equal(t, 2, code)
		require.Empty(t, out)
		require.NotEmpty(t, stderr)
	}
}

func TestShellSubcommandsHaveHelpSchemaAndScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			for _, args := range [][]string{{"help", "completion", shell}, {"completion", shell, "--help"}} {
				code, out, stderr := invoke(t, args...)
				require.Zero(t, code, stderr)
				require.Empty(t, stderr)
				require.Contains(t, out, "testcli completion "+shell)
				require.Contains(t, out, "Examples:")
			}
			code, out, stderr := invoke(t, "schema", "completion", shell)
			require.Zero(t, code, stderr)
			var doc struct {
				Commands []commandSchema `json:"commands"`
			}
			require.NoError(t, json.Unmarshal([]byte(out), &doc))
			require.Len(t, doc.Commands, 1)
			require.Equal(t, "testcli completion "+shell, doc.Commands[0].Path)
			code, out, stderr = invoke(t, "completion", shell)
			require.Zero(t, code, stderr)
			require.Empty(t, stderr)
			require.Contains(t, out, "testcli")
			legacyCode, legacyOut, legacyErr := invoke(t, "completion", "--", shell)
			require.Zero(t, legacyCode, legacyErr)
			require.Empty(t, legacyErr)
			require.Equal(t, out, legacyOut)

			code, out, stderr = invoke(t, "completion", shell, "extra")
			require.Equal(t, 2, code)
			require.Empty(t, out)
			require.NotEmpty(t, stderr)
		})
	}
}

func TestAllCommandsHaveLocalizedExamplesWithInvocationName(t *testing.T) {
	for _, lang := range []string{"en", "zh-CN"} {
		for _, path := range [][]string{{}, {"help"}, {"version"}, {"schema"}, {"completion"}, {"greet"}} {
			args := append([]string{"--lang", lang}, path...)
			args = append(args, "--help")
			code, out, stderr := invoke(t, args...)
			require.Zero(t, code, stderr)
			require.Empty(t, stderr)
			heading := "Examples:"
			if lang == "zh-CN" {
				heading = "示例："
			}
			require.Contains(t, out, heading)
			require.Contains(t, out, "testcli ")
		}
	}
}

func TestCompletionNeverLoadsConfigurationOrCreatesState(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{{"completion", "bash"}, {"help", "completion", "bash"}, {"__complete", "help", ""}, {"__complete", "--lang=e", "--log-level", ""}} {
		var out, stderr bytes.Buffer
		code := Run(context.Background(), append([]string{"--config", filepath.Join(home, "missing.toml")}, args...), Options{Name: "sample", Out: &out, ErrOut: &stderr, LookupEnv: func(key string) (string, bool) { return home, key == "SAMPLE_HOME" }})
		require.Zero(t, code, stderr.String())
		require.Empty(t, stderr.String())
		_, err := os.Stat(home)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestConcurrentCompletionLanguageIsolation(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		for _, lang := range []string{"en", "zh-CN"} {
			wg.Go(func() {
				var out, stderr bytes.Buffer
				code := Run(context.Background(), []string{"__complete", "--lang", lang, ""}, Options{Name: "sample", Out: &out, ErrOut: &stderr, LookupEnv: func(string) (string, bool) { return "", false }})
				if code != 0 || stderr.Len() != 0 {
					t.Errorf("%s: code=%d stderr=%s", lang, code, stderr.String())
					return
				}
				want := "version\tShow"
				if lang == "zh-CN" {
					want = "version\t显示"
				}
				if !strings.Contains(out.String(), want) {
					t.Errorf("%s: %s", lang, out.String())
				}
			})
		}
	}
	wg.Wait()
}

func TestCompletionRecognizesInitializedHelpAndVersionFlags(t *testing.T) {
	for _, flag := range []string{"--help=false", "--version=false", "-h=false", "-v=false"} {
		code, out, stderr := invoke(t, flag, "__complete", "--lang", "")
		require.Zero(t, code, stderr)
		require.Empty(t, stderr)
		require.Contains(t, out, ":4\n")
	}
}

func TestCanceledCompletionDoesNotCommitErrorDirective(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, out, stderr := invokeContext(t, ctx, "__complete", "--unknown", "")
	require.Equal(t, 130, code)
	require.Empty(t, out)
	require.NotEmpty(t, stderr)
}
