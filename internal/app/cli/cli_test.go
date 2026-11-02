package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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

func TestCancellationExitCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, out, errOut := invokeContext(t, ctx, "greet", "--name", "Codex")
	if code != 130 || out != "" || !strings.Contains(errOut, "Operation cancelled") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
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
