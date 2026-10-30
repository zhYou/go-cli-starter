package command

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// 文件门闩确定先后次序；TERM 标记证明失败已触发 fail-fast 且另一项尚在收尾。
func TestBatchFirstCauseSurvivesCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TERM 宽限时序属于 Unix 后端")
	}
	for _, failureFirst := range []bool{true, false} {
		name := "parent-first"
		if failureFirst {
			name = "failure-first"
		}
		t.Run(name, func(t *testing.T) {
			call, options := helper(t, "term-marker")
			options.WaitDelay = time.Second
			ready := filepath.Join(options.Dir, "ready")
			term := filepath.Join(options.Dir, "term")
			failReady := filepath.Join(options.Dir, "fail-ready")
			gate := filepath.Join(options.Dir, "gate")
			call.Args = append(call.Args, ready, term)
			failure := call
			failure.Args = []string{"-test.run=^TestHelperProcess$", "--", "gate-fail", failReady, gate}
			ctx, cancel := context.WithCancelCause(t.Context())
			parentCause := errors.New("父调用停止")
			type outcome struct {
				items []Result
				err   error
			}
			done := make(chan outcome, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				items, err := RunBatch(ctx, []Invocation{call, failure, call}, options, BatchOptions{Concurrency: 2, FailFast: true})
				done <- outcome{items, err}
			}()
			t.Cleanup(func() {
				cancel(parentCause)
				select {
				case <-finished:
				case <-time.After(4 * time.Second):
					t.Error("真实 helper 收尾未完成")
				}
			})
			waitMarker(t, ready)
			waitMarker(t, failReady)
			if failureFirst {
				if err := os.WriteFile(gate, nil, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				cancel(parentCause)
			}
			waitMarker(t, term)
			if failureFirst {
				cancel(parentCause)
			} else {
				if err := os.WriteFile(gate, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var result outcome
			select {
			case result = <-done:
			case <-time.After(4 * time.Second):
				t.Fatal("批量没有完成收尾")
			}
			var exit *exec.ExitError
			if !errors.As(result.items[1].Err, &exit) || exit.ExitCode() != 7 {
				t.Fatalf("未保留真实失败: %+v", result.items)
			}
			if result.items[0].Status != Canceled || !errors.Is(result.items[0].Err, context.Canceled) || result.items[2].Status != Skipped {
				t.Fatalf("单项状态/错误链: %+v", result.items)
			}
			if failureFirst {
				if result.err == nil || errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
					t.Fatalf("后到父取消覆盖普通首因: %v", result.err)
				}
				if !errors.As(result.err, &exit) || exit.ExitCode() != 7 {
					t.Fatalf("整体丢失首个退出错误: %v", result.err)
				}
			} else if !errors.Is(result.err, context.Canceled) || !errors.Is(result.err, parentCause) || !errors.Is(result.items[0].Err, parentCause) {
				t.Fatalf("父取消首因未保留: %v %+v", result.err, result.items)
			}
		})
	}
}
func waitMarker(t *testing.T, path string) {
	t.Helper()
	limit := time.Now().Add(3 * time.Second)
	for time.Now().Before(limit) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("未出现进程同步标记 %s", path)
}

// 一纳秒期限在构造子 context 时已到期，直接覆盖拿到名额但尚未进入 Run 的路径。
func TestBatchImmediateItemDeadlineIsNotSkipped(t *testing.T) {
	for _, failFast := range []bool{false, true} {
		name := "collect-all"
		if failFast {
			name = "fail-fast"
		}
		t.Run(name, func(t *testing.T) {
			call, options := helper(t, "bytes")
			results, err := RunBatch(t.Context(), []Invocation{call, call, call}, options, BatchOptions{Concurrency: 1, ItemTimeout: time.Nanosecond, FailFast: failFast})
			if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("自身期限应汇总为普通失败: %v", err)
			}
			for i, result := range results {
				if failFast && i > 0 {
					if result.Status != Skipped || !errors.Is(result.Err, context.Canceled) {
						t.Fatalf("未取名额项应 skipped: %+v", result)
					}
					continue
				}
				if result.Status != Canceled || !errors.Is(result.Err, context.DeadlineExceeded) || len(result.Stdout) != 0 {
					t.Fatalf("获名额后的自身期限被吞掉: %+v", result)
				}
			}
		})
	}
	for _, parentDeadline := range []bool{false, true} {
		parent, cancel := context.WithCancel(t.Context())
		cancel()
		if parentDeadline {
			var stop context.CancelFunc
			parent, stop = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			defer stop()
		}
		call, options := helper(t, "bytes")
		results, err := RunBatch(parent, []Invocation{call}, options, BatchOptions{ItemTimeout: time.Nanosecond})
		if results[0].Status != Skipped || !errors.Is(results[0].Err, parent.Err()) || !errors.Is(err, parent.Err()) {
			t.Fatalf("父先取消应 skipped: %v %+v", err, results)
		}
	}
}
