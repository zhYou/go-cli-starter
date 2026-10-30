package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"github.com/zhYou/go-cli-starter/internal/tools/command"
)

func TestCommandEmbeddingHelper(t *testing.T) {
	if os.Getenv("COMMAND_EMBED_HELPER") == "1" {
		os.Exit(7)
	}
}
func TestEmbeddedCommandErrorsFollowCLILanguage(t *testing.T) {
	for language, want := range map[string]string{"zh-CN": "第 1 项执行失败", "en": "Batch item 1 failed"} {
		t.Run(language, func(t *testing.T) {
			home := t.TempDir()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			deps := &Dependencies{Greet: func(ctx context.Context, opts greeting.Options) (greeting.Result, error) {
				_, err := command.RunBatch(ctx, []command.Invocation{{Program: executable, Args: []string{"-test.run=^TestCommandEmbeddingHelper$"}}}, command.Execution{Dir: home, Env: []string{"COMMAND_EMBED_HELPER=1", "GORACE=atexit_sleep_ms=0", "GO_CLI_STARTER_HOME=" + home, "HOME=" + home, "USERPROFILE=" + home, "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}, Localizer: opts.Localizer}, command.BatchOptions{})
				return greeting.Result{}, err
			}}
			code := Run(t.Context(), []string{"greet", "--name", "fixture", "--lang", language, "--json"}, Options{Out: &stdout, ErrOut: &stderr, Dependencies: deps, LookupEnv: func(key string) (string, bool) {
				if key == "GO_CLI_STARTER_HOME" {
					return home, true
				}
				return "", false
			}})
			if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), want) || !strings.Contains(stderr.String(), "exit status 7") {
				t.Fatalf("嵌入工具诊断 code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
