//go:build linux || darwin

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
				if outcome == "wait" {
					go func() {
						for i := 0; i < 200; i++ {
							if _, err := os.Stat(heartbeat); err == nil {
								cancel()
								return
							}
							time.Sleep(5 * time.Millisecond)
						}
						cancel()
					}()
				}
				start := time.Now()
				r := Capture(ctx, call, options, CaptureOptions{})
				if time.Since(start) > 3*time.Second {
					t.Fatal("未有界收尾")
				}
				if outcome == "fail" {
					var exit *exec.ExitError
					if !errors.As(r.Err, &exit) {
						t.Fatalf("丢失退出错误 %v", r.Err)
					}
				}
				if outcome == "wait" && !errors.Is(r.Err, context.Canceled) {
					t.Fatal(r.Err)
				}
				before, err := os.ReadFile(heartbeat)
				if err != nil {
					t.Fatal(err)
				}
				time.Sleep(60 * time.Millisecond)
				after, err := os.ReadFile(heartbeat)
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(after) {
					t.Fatal("返回后后代仍在运行")
				}
			})
		}
	}
}
func TestCleanupDoesNotKillSibling(t *testing.T) {
	sibling, opts := helper(t, "heartbeat")
	path := filepath.Join(opts.Dir, "sibling")
	sibling.Args = append(sibling.Args, path)
	process := exec.Command(sibling.Program, sibling.Args...)
	process.Env = opts.Env
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	call, options := helper(t, "exit")
	_ = Run(t.Context(), call, options)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) {
		t.Fatal("误伤兄弟进程")
	}
}

func TestRootExitsBeforeGrandchildHoldingPipes(t *testing.T) {
	call, options := helper(t, "branch")
	heartbeat := filepath.Join(options.Dir, "grandchild")
	call.Args = append(call.Args, heartbeat, "pipes")
	r := Capture(t.Context(), call, options, CaptureOptions{})
	if r.Err != nil && !errors.Is(r.Err, exec.ErrWaitDelay) {
		t.Fatal(r.Err)
	}
	before, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	after, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("持有管道的孙进程仍在工作")
	}
}
