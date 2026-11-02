//go:build linux || darwin || windows

package command

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnedDescendantsStopOnEveryReturn(t *testing.T) {
	for _, outcome := range []string{"success", "fail", "wait"} {
		for _, streams := range []string{"pipes", "closed"} {
			t.Run(outcome+"/"+streams, func(t *testing.T) {
				call, options := helper(t, "spawn")
				heartbeat := filepath.Join(options.Dir, "heartbeat")
				call.Args = append(call.Args, heartbeat, streams, outcome)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				start := time.Now()
				done := startCapture(t, ctx, call, options)
				if outcome == "wait" {
					waitMarker(t, heartbeat)
					cancel()
				}
				r := awaitCapture(t, done)
				if time.Since(start) > 3*time.Second {
					t.Fatal("未有界收尾")
				}
				switch outcome {
				case "success":
					if r.Err == nil {
						if r.Status != Succeeded || r.ExitCode != 0 {
							t.Fatalf("成功结果不完整: %+v", r)
						}
					} else if streams == "pipes" && errors.Is(r.Err, exec.ErrWaitDelay) {
						// 根进程已成功退出，后代占用管道仍可能耗尽排空宽限。
						if r.Status != Failed || r.ExitCode != -1 {
							t.Fatalf("管道宽限结果不一致: %+v", r)
						}
					} else {
						t.Fatalf("成功场景返回非预期错误: %+v", r)
					}
				case "fail":
					var exit *exec.ExitError
					if !errors.As(r.Err, &exit) || exit.ExitCode() != 7 || r.ExitCode != 7 || r.Status != Failed {
						t.Fatalf("失败结果不完整: %+v", r)
					}
				case "wait":
					var exit *exec.ExitError
					if !errors.Is(r.Err, context.Canceled) || !errors.As(r.Err, &exit) || r.Status != Canceled || r.ExitCode != exit.ExitCode() {
						t.Fatalf("取消结果不完整: %+v", r)
					}
				}
				assertHeartbeatStopped(t, heartbeat, 60*time.Millisecond)
			})
		}
	}
}
func TestCleanupDoesNotKillSibling(t *testing.T) {
	sibling, opts := helper(t, "heartbeat")
	path := filepath.Join(opts.Dir, "sibling")
	sibling.Args = append(sibling.Args, path)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	//nolint:gosec // G204：只执行当前测试二进制，参数来自临时 fixture。
	process := exec.CommandContext(ctx, sibling.Program, sibling.Args...)
	process.Env = opts.Env
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = process.Process.Kill(); _ = process.Wait() })
	waitMarker(t, path)
	call, options := helper(t, "exit")
	err := Run(t.Context(), call, options)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("清理目标没有按预期失败: %v", err)
	}
	//nolint:gosec // G304：心跳路径只来自当前测试的临时目录。
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	waitHeartbeatUpdate(t, path, before)
}

func TestRootExitsBeforeGrandchildHoldingPipes(t *testing.T) {
	call, options := helper(t, "branch")
	heartbeat := filepath.Join(options.Dir, "grandchild")
	call.Args = append(call.Args, heartbeat, "pipes")
	r := awaitCapture(t, startCapture(t, t.Context(), call, options))
	if r.Err == nil {
		if r.Status != Succeeded || r.ExitCode != 0 {
			t.Fatalf("根成功结果不完整: %+v", r)
		}
	} else if errors.Is(r.Err, exec.ErrWaitDelay) {
		if r.Status != Failed || r.ExitCode != -1 {
			t.Fatalf("管道宽限结果不一致: %+v", r)
		}
	} else {
		t.Fatal(r.Err)
	}
	assertHeartbeatStopped(t, heartbeat, 60*time.Millisecond)
}
