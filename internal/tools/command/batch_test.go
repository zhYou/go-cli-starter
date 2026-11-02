package command

import (
	"context"
	"encoding/json"
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

func TestBatchOrderCollectAllAndTimeoutClassification(t *testing.T) {
	first, options := helper(t, "sleep", "140ms", "first")
	second := first
	second.Args = append([]string(nil), first.Args...)
	second.Args[len(second.Args)-1] = "second"
	failed := first
	failed.Args = []string{"-test.run=^TestHelperProcess$", "--", "exit"}
	items := []Invocation{first, failed, second}
	results, err := RunBatch(t.Context(), items, options, BatchOptions{Concurrency: 1, ItemTimeout: 200 * time.Millisecond})
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("整体分类 %v", err)
	}
	if string(results[0].Stdout) != "first" || results[1].ExitCode != 7 || string(results[2].Stdout) != "second" {
		t.Fatalf("顺序 %+v", results)
	}
	results, err = RunBatch(t.Context(), []Invocation{first}, options, BatchOptions{ItemTimeout: 10 * time.Millisecond})
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !errors.Is(results[0].Err, context.DeadlineExceeded) {
		t.Fatalf("单项超时: %v %+v", err, results)
	}
}

func TestBatchFailFastAndParentCancellation(t *testing.T) {
	fail, options := helper(t, "exit")
	sleep := fail
	sleep.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "1m", "never"}
	results, err := RunBatch(t.Context(), []Invocation{fail, sleep, sleep}, options, BatchOptions{Concurrency: 1, FailFast: true})
	if err == nil || errors.Is(err, context.Canceled) || results[1].Status != Skipped || results[2].Status != Skipped {
		t.Fatalf("fail-fast %v %+v", err, results)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	results, err = RunBatch(ctx, []Invocation{sleep, sleep}, options, BatchOptions{})
	if !errors.Is(err, context.Canceled) || results[0].Status != Skipped || results[1].Status != Skipped {
		t.Fatalf("父取消 %v %+v", err, results)
	}
}

func TestBatchConcurrencyBound(t *testing.T) {
	call, options := helper(t, "interval")
	items := make([]Invocation, 6)
	for i := range items {
		items[i] = call
	}
	results, err := RunBatch(t.Context(), items, options, BatchOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	intervals := make([][2]int64, len(results))
	for i, r := range results {
		if err := json.Unmarshal(r.Stdout, &intervals[i]); err != nil {
			t.Fatal(err)
		}
	}
	maxActive := 0
	for _, period := range intervals {
		active := 0
		for _, other := range intervals {
			if other[0] <= period[0] && period[0] < other[1] {
				active++
			}
		}
		if active > maxActive {
			maxActive = active
		}
	}
	if maxActive != 2 {
		t.Fatalf("实际最大并发 %d", maxActive)
	}
}

func TestConcurrentFailFastKeepsItemCauseOnly(t *testing.T) {
	call, options := helper(t, "exit")
	sleep := call
	sleep.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "1m", "never"}
	results, err := RunBatch(t.Context(), []Invocation{sleep, call, sleep}, options, BatchOptions{Concurrency: 2, FailFast: true})
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("整体错误 %v", err)
	}
	if results[0].Status != Canceled || !errors.Is(results[0].Err, context.Canceled) || results[2].Status != Skipped {
		t.Fatalf("派生取消 %+v", results)
	}
}

func TestActiveParentCancelStopsQueueAndRetainsCause(t *testing.T) {
	call, options := helper(t, "sleep", "1m", "never")
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("父调用终止")
	timer := time.AfterFunc(100*time.Millisecond, func() { cancel(cause) })
	defer timer.Stop()
	results, err := RunBatch(ctx, []Invocation{call, call, call}, options, BatchOptions{Concurrency: 1})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || results[0].Status != Canceled || results[1].Status != Skipped || results[2].Status != Skipped {
		t.Fatalf("父取消 %v %+v", err, results)
	}
}
