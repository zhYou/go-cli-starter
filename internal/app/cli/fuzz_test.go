package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
)

// isolatedOptions 显式注入配置、目录与环境；任何 argv 都不读取或写入用户状态。
func isolatedOptions(home string, out, stderr io.Writer) Options {
	dirs := paths.Directories{Config: home, Data: filepath.Join(home, "data"), State: filepath.Join(home, "state"), Cache: filepath.Join(home, "cache")}
	return Options{Directories: &dirs, Out: out, ErrOut: stderr, LookupEnv: func(key string) (string, bool) {
		if key == "GO_CLI_STARTER_HOME" {
			return home, true
		}
		return "", false
	}, Dependencies: &Dependencies{LoadConfig: func(config.Options) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Logging.Output = "none"
		return cfg, nil
	}}}
}
func FuzzArgv(f *testing.F) {
	for _, seed := range []struct {
		args   []string
		cancel bool
	}{
		{nil, false}, {[]string{""}, false}, {[]string{" "}, false},
		{[]string{"greet", "--name", ""}, false}, {[]string{"greet", "--name", " "}, false},
		{[]string{"greet", "--name", "你好 世界", "--json"}, false},
		{[]string{"greet", "--", "--name", "x"}, false},
		{[]string{"greet", "--name", "--lang"}, false},
		{[]string{"greet", "--name", "x"}, true}, {[]string{"schema"}, true},
		{[]string{"completion", "bash"}, false},
	} {
		encoded := ""
		if len(seed.args) > 0 {
			encoded = strings.Join(seed.args, "\x00") + "\x00"
		}
		f.Add(encoded, seed.cancel)
	}
	home := f.TempDir()
	f.Setenv("GO_CLI_STARTER_HOME", home)
	f.Fuzz(func(t *testing.T, raw string, cancelled bool) {
		if len(raw) > 4096 {
			t.Skip()
		}
		var args []string
		if raw != "" {
			args = strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
		}
		if len(args) > 64 {
			t.Skip()
		}
		var out, stderr bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if cancelled {
			cancel()
		}
		code := Run(ctx, args, isolatedOptions(home, &out, &stderr))
		switch code {
		case 0, 1, 2, 130:
		default:
			t.Fatalf("非契约退出码 %d，argv=%q", code, args)
		}
		if code != 0 && out.Len() != 0 {
			t.Fatalf("失败时 stdout 非空：code=%d argv=%q output=%q", code, args, out.String())
		}
		if code != 0 && stderr.Len() == 0 {
			t.Fatalf("失败时缺少诊断：code=%d argv=%q", code, args)
		}
		entries, err := os.ReadDir(home)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("隔离注入仍产生状态：%v", entries)
		}
	})
}
func TestArgvCancellationReturns130(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out, stderr bytes.Buffer
	if code := Run(ctx, []string{"greet", "--name", "你好"}, isolatedOptions(home, &out, &stderr)); code != 130 {
		t.Fatalf("取消退出码=%d", code)
	}
	if out.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("取消输出不满足约定：stdout=%q stderr=%q", out.String(), stderr.String())
	}
}
func BenchmarkBuildRoot(b *testing.B) {
	opts := isolatedOptions(b.TempDir(), io.Discard, io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buildRoot(opts)
	}
}
func BenchmarkRunGreet(b *testing.B) {
	opts := isolatedOptions(b.TempDir(), io.Discard, io.Discard)
	args := []string{"greet", "--name", "Codex", "--json"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if code := Run(b.Context(), args, opts); code != 0 {
			b.Fatalf("退出码=%d", code)
		}
	}
}
