package cli

import (
	"bytes"
	"context"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEnglishInvocation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--lang", "en", "greet", "--name", "Ada"}, 0, "Hello, Ada!"},
		{[]string{"--lang=en", "greet", "--name", "Ada", "--json"}, 0, "Hello, Ada!"},
		{[]string{"greet", "--lang=en", "--help"}, 0, "Usage:"},
		{[]string{"--lang=en", "schema", "greet"}, 0, "Greeting"},
		{[]string{"--lang=en", "version"}, 0, "version"},
		{[]string{"--lang=en", "greet"}, 2, "--name"},
		{[]string{"--lang=en", "greet", "--unknown"}, 2, "Unknown option"},
	} {
		code, out, errOut := invoke(t, tc.args...)
		require.Equal(t, tc.code, code, "%v: %s", tc.args, errOut)
		require.Contains(t, out+errOut, tc.want)
		if tc.code != 0 {
			require.True(t, strings.HasPrefix(errOut, "Error:"), errOut)
			require.Empty(t, out)
		}
	}
}
func TestLanguageScanRespectsFlagValuesAndSeparator(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"greet", "--name", "--lang", "--json"}, 0, "你好，--lang！"},
		{[]string{"greet", "--name", "--lang=en"}, 0, "你好，--lang=en！"},
		{[]string{"greet", "--name=x", "--", "--lang=en"}, 2, "错误："},
		{[]string{"--lang", "zh", "greet", "--name=x"}, 0, "你好，x！"},
		{[]string{"__complete", "greet", "--lang", ""}, 0, ":0"},
	} {
		code, out, errOut := invoke(t, tc.args...)
		require.Equal(t, tc.code, code, "%v: %s", tc.args, errOut)
		require.Contains(t, out+errOut, tc.want)
	}
}
func TestRequiredAndGroupsBeforeConfiguration(t *testing.T) {
	for _, group := range []bool{false, true} {
		count := 0
		root := NewRoot(Options{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}, Dependencies: &Dependencies{
			LoadConfig: func(config.Options) (config.Config, error) { count++; return config.Config{}, nil },
		}})
		cmd := &cobra.Command{Use: "extra", RunE: func(*cobra.Command, []string) error { count++; return nil }}
		cmd.Flags().String("a", "", "a")
		cmd.Flags().String("b", "", "b")
		if group {
			cmd.MarkFlagsRequiredTogether("a", "b")
		} else {
			require.NoError(t, cmd.MarkFlagRequired("a"))
		}
		root.AddCommand(cmd)
		args := []string{"extra"}
		if group {
			args = append(args, "--a=x")
		}
		root.SetArgs(args)
		require.Error(t, root.ExecuteContext(context.Background()))
		require.Zero(t, count)
	}
}

func TestLanguagePrecedenceAndIsolation(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			var out, stderr bytes.Buffer
			args := []string{"greet", "--name", "Ada"}
			lang := "en"
			want := "Hello, Ada!"
			if i%2 == 0 {
				args = append(args, "--lang", "zh")
				want = "你好，Ada！"
			}
			code := Run(context.Background(), args, Options{Name: "sample", ConfigDir: t.TempDir(), Out: &out, ErrOut: &stderr, LookupEnv: func(k string) (string, bool) { return lang, k == "SAMPLE_LANG" }})
			require.Zero(t, code, stderr.String())
			require.Contains(t, out.String(), want)
		})
	}
	wg.Wait()
	code, out, errOut := invoke(t, "--lang=de", "version")
	require.Equal(t, 2, code)
	require.Empty(t, out)
	require.Contains(t, errOut, "de")
}
func TestMetadataLanguageScanForNewFlags(t *testing.T) {
	root := NewRoot(Options{})
	cmd := &cobra.Command{Use: "extra"}
	cmd.Flags().StringP("source", "s", "", "来源")
	root.AddCommand(cmd)
	for _, args := range [][]string{
		{"extra", "--source", "--lang=en"},
		{"extra", "-s", "--lang=en"},
		{"extra", "-s--lang=en"},
		{"greet", "--name", "__complete", "--lang=en"},
	} {
		lang, err := selectLanguage(root, args, "zh-CN")
		require.NoError(t, err)
		want := "zh-CN"
		if args[0] == "greet" {
			want = "en"
		}
		require.Equal(t, want, lang, "%v", args)
	}
}
func TestInvalidExplicitValuesNeverLoadConfig(t *testing.T) {
	count := 0
	deps := Dependencies{LoadConfig: func(config.Options) (config.Config, error) { count++; return config.Defaults(), nil }}
	var out, stderr bytes.Buffer
	code := Run(context.Background(), []string{"greet", "--name", "Ada", "--log-level", "noisy"}, Options{Dependencies: &deps, Out: &out, ErrOut: &stderr})
	require.Equal(t, 2, code)
	require.Zero(t, count)
	require.Empty(t, out.String())
}
func TestNoneLoggingAndEnglishDiagnostics(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[logging]\nlevel = \"debug\"\noutput = \"none\"\n"), 0600))
	code, out, stderr := invoke(t, "greet", "--name", "Ada", "--lang=en", "--config", configPath)
	require.Zero(t, code)
	require.Contains(t, out, "Hello, Ada!")
	require.Empty(t, stderr)
	require.NoError(t, os.WriteFile(configPath, []byte("[logging]\noutput = \"stdout\"\n"), 0600))
	code, out, stderr = invoke(t, "greet", "--name", "Ada", "--lang=en", "--config", configPath)
	require.Equal(t, 1, code)
	require.Empty(t, out)
	require.Contains(t, stderr, "Invalid configuration")
	require.NotContains(t, stderr, "日志")
}
