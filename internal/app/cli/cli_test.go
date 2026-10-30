package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
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
		Out: &out, ErrOut: &errOut, ConfigDir: t.TempDir(), Directories: &paths.Directories{State: t.TempDir()},
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
		{"text", []string{"greet", "--name", "Codex"}, "你好，Codex！\n"},
		{"json", []string{"greet", "--name", "Codex", "--json"}, "{\"message\":\"你好，Codex！\"}\n"},
		{"unicode", []string{"--json", "greet", "--name", "周游"}, "{\"message\":\"你好，周游！\"}\n"},
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
		if code != 2 || out != "" || strings.Count(errOut, "错误：") != 1 {
			t.Errorf("args=%v code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
}

func TestChineseHelpRegardlessOfLocale(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	for _, args := range [][]string{{}, {"--help"}, {"greet", "--help"}, {"help", "greet"}, {"version", "--help"}, {"schema", "--help"}, {"completion", "--help"}, {"help", "--help"}} {
		code, out, errOut := invoke(t, args...)
		if code != 0 || errOut != "" || !strings.Contains(out, "用法：") || !strings.Contains(out, "显示帮助") || !strings.Contains(out, "日志级别") || !strings.Contains(out, "默认值") {
			t.Errorf("参数=%v 退出码=%d 标准输出=%q 标准错误=%q", args, code, out, errOut)
		}
		for _, english := range []string{"Usage:", "Available Commands:", "Flags:", "Global Flags:", "help for", "(default", "[command]", "NAME"} {
			if strings.Contains(out, english) {
				t.Errorf("参数=%v 帮助仍包含 %q：%s", args, english, out)
			}
		}
	}
}

func TestChineseErrorsPreserveUserValues(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"unknown"}, "unknown"},
		{[]string{"greet"}, "--name"},
		{[]string{"greet", "--name", "x", "extra"}, "extra"},
		{[]string{"greet", "--unknown"}, "unknown"},
		{[]string{"greet", "-x"}, "x"},
		{[]string{"greet", "--name"}, "name"},
		{[]string{"--json=invalid"}, "invalid"},
		{[]string{"completion", "bogus"}, "bogus"},
		{[]string{"help", "bogus"}, "bogus"},
		{[]string{"schema", "bogus"}, "bogus"},
	} {
		code, out, errOut := invoke(t, tc.args...)
		if code != 2 || out != "" || !strings.HasPrefix(errOut, "错误：") || !strings.Contains(errOut, tc.want) {
			t.Errorf("参数=%v 退出码=%d 标准输出=%q 标准错误=%q", tc.args, code, out, errOut)
		}
		for _, english := range []string{"unknown", "required", "unsupported", "invalid", "accepts", "Error:"} {
			// 用户输入保留原文，错误模板必须使用中文。
			if strings.Contains(strings.ReplaceAll(errOut, tc.want, ""), english) {
				t.Errorf("参数=%v 错误仍包含英文模板：%s", tc.args, errOut)
			}
		}
	}
}

func TestChineseVersionAndSchema(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		code, out, errOut := invoke(t, args...)
		if code != 0 || !strings.Contains(out, "版本") || !strings.Contains(out, "v1.2.3") || errOut != "" {
			t.Fatalf("参数=%v 退出码=%d 标准输出=%q 标准错误=%q", args, code, out, errOut)
		}
	}
	code, out, errOut := invoke(t, "schema", "greet")
	var got struct {
		Commands []commandSchema `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || errOut != "" || len(got.Commands) != 1 || !strings.Contains(got.Commands[0].Description, "问候") {
		t.Fatalf("退出码=%d 标准输出=%q 标准错误=%q", code, out, errOut)
	}
}

func TestDebugLogsDoNotPolluteJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[logging]\noutput = \"stderr\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, "greet", "--name", "Codex", "--json", "--log-level", "debug", "--config", path)
	if code != 0 || !json.Valid([]byte(out)) || !strings.Contains(errOut, `"level":"debug"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestDiscoveryBypassesBrokenConfiguration(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.toml")
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
	if code != 130 || out != "" || !strings.Contains(errOut, "操作已取消") {
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
	root := metadataRoot(Options{Name: "testcli", Out: &out})
	extra := &cobra.Command{Use: "extra", Short: "新增的命令"}
	extra.Flags().String("token", "", "示例选项")
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
		t.Fatalf("缺少必填选项标记：%s", out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestOutputFailureIsRuntimeError(t *testing.T) {
	var errOut bytes.Buffer
	code := Run(context.Background(), []string{"greet", "--name", "x"}, Options{
		Name: "testcli", Out: failingWriter{}, ErrOut: &errOut, ConfigDir: t.TempDir(), Directories: &paths.Directories{State: t.TempDir()},
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	if code != 1 || !strings.Contains(errOut.String(), "output unavailable") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}

// metadataRoot 仅用于无业务执行的命令元数据测试。业务测试统一通过 Run 托管资源。
func metadataRoot(opts Options) *cobra.Command { root, _ := buildRoot(defaults(opts)); return root }
