package command

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCancellationRetainsExitAndCause(t *testing.T) {
	call, options := helper(t, "sleep", "1m", "never")
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("调用者取消")
	timer := time.AfterFunc(100*time.Millisecond, func() { cancel(cause) })
	defer timer.Stop()
	r := Capture(ctx, call, options, CaptureOptions{})
	var exit *exec.ExitError
	if !errors.Is(r.Err, context.Canceled) || !errors.Is(r.Err, cause) || !errors.As(r.Err, &exit) || r.Status != Canceled {
		t.Fatalf("取消错误链 %+v", r)
	}
}

func TestInvalidExecutionDoesNotStart(t *testing.T) {
	if err := Run(t.Context(), Invocation{}, Execution{}); err == nil {
		t.Fatal("空程序应失败")
	}
	if _, err := RunBatch(t.Context(), nil, Execution{}, BatchOptions{Concurrency: -1}); err == nil || strings.Contains(err.Error(), "panic") {
		t.Fatalf("无效并发数: %v", err)
	}
}

func TestNoImplicitExecutionDeadline(t *testing.T) {
	call, options := helper(t, "sleep", "400ms", "complete")
	r := Capture(t.Context(), call, options, CaptureOptions{})
	if r.Err != nil || string(r.Stdout) != "complete" {
		t.Fatalf("默认宽限错误地限制执行时间 %+v", r)
	}
}
