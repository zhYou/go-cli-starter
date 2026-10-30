package command

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// BatchOptions 缺省顺序执行并收集全部结果；单项超时从取得名额后开始计算。
// 并发共享的输入流应由调用方提供并发安全实现，输出始终逐项独立捕获。
type BatchOptions struct {
	Concurrency int
	ItemTimeout time.Duration
	FailFast    bool
	Capture     CaptureOptions
}

// RunBatch 按输入顺序返回结果。派生取消留在单项错误，整体仅保留真实失败及父取消。
func RunBatch(parent context.Context, calls []Invocation, execution Execution, options BatchOptions) ([]Result, error) {
	if options.Concurrency < 0 || options.ItemTimeout < 0 || options.Capture.Limit < 0 {
		return nil, errors.New("批量并发数、超时和捕获上限不能为负数")
	}
	if options.Concurrency == 0 {
		options.Concurrency = 1
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	results := make([]Result, len(calls))
	for i := range results {
		results[i] = Result{Status: Skipped, ExitCode: -1}
	}
	sem := semaphore.NewWeighted(int64(options.Concurrency))
	var group errgroup.Group
	var mu sync.Mutex
	var first error
	for i, call := range calls {
		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}
		if ctx.Err() != nil {
			sem.Release(1)
			break
		}
		group.Go(func() error {
			defer sem.Release(1)
			itemCtx := ctx
			stop := func() {}
			if options.ItemTimeout > 0 {
				itemCtx, stop = context.WithTimeout(ctx, options.ItemTimeout)
			}
			defer stop()
			results[i] = Capture(itemCtx, call, execution, options.Capture)
			if results[i].Err != nil {
				mu.Lock()
				if first == nil {
					first = batchFailure(i, results[i].Err)
					if options.FailFast {
						cancel(first)
					}
				}
				mu.Unlock()
			}
			return nil
		})
	}
	_ = group.Wait()
	var failures []error
	for i := range results {
		if results[i].Status == Skipped {
			results[i].Err = contextError(ctx)
		}
		if !options.FailFast && results[i].Err != nil && results[i].Status != Skipped {
			failures = append(failures, batchFailure(i, results[i].Err))
		}
	}
	if options.FailFast && first != nil {
		failures = append(failures, first)
	}
	if parent.Err() != nil {
		failures = append(failures, contextError(parent))
	}
	return results, errors.Join(failures...)
}

func batchFailure(index int, err error) error {
	// 标准进程错误保留 errors.As；派生 deadline/cancel 不使 CLI 将普通批量失败映射为 130。
	cause := withoutCancellation(err)
	if cause == nil {
		return fmt.Errorf("第 %d 项执行未完成：%v", index+1, err)
	}
	return fmt.Errorf("第 %d 项执行失败：%w", index+1, cause)
}
func withoutCancellation(err error) error {
	if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var kept []error
		for _, child := range joined.Unwrap() {
			kept = append(kept, withoutCancellation(child))
		}
		return errors.Join(kept...)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return withoutCancellation(errors.Unwrap(err))
	}
	return err
}
