package command

import (
	"context"
	"errors"
	"os/exec"
)

// Status 区分完成、失败、已取消和未获得执行机会的调用。
type Status string

const (
	Succeeded Status = "succeeded"
	Failed    Status = "failed"
	Canceled  Status = "canceled"
	Skipped   Status = "skipped"
)
const DefaultCaptureLimit = 1 << 20

// CaptureOptions.Limit 是每路输出的字节上限，0 使用 1 MiB；达到上限后持续排空。
type CaptureOptions struct{ Limit int }

// Result 保留原始输出及错误，供调用方先判断结果，再向 CLI stdout 提交。
type Result struct {
	Stdout, Stderr                   []byte
	StdoutTruncated, StderrTruncated bool
	ExitCode                         int
	Err                              error
	Status                           Status
}

// Capture 使用独立输出缓冲，保留 Execution.IO.In，不写入借用的输出流。
func Capture(ctx context.Context, call Invocation, options Execution, capture CaptureOptions) Result {
	if ctx.Err() != nil {
		return Result{ExitCode: -1, Err: contextError(ctx), Status: Skipped}
	}
	if capture.Limit < 0 {
		return Result{ExitCode: -1, Err: captureLimitInvalid.New(), Status: Failed}
	}
	if capture.Limit == 0 {
		capture.Limit = DefaultCaptureLimit
	}
	stdout, stderr := limitedBuffer{limit: capture.Limit}, limitedBuffer{limit: capture.Limit}
	options.IO.Out, options.IO.ErrOut = &stdout, &stderr
	err := Run(ctx, call, options)
	r := Result{Stdout: stdout.data, Stderr: stderr.data, StdoutTruncated: stdout.truncated, StderrTruncated: stderr.truncated, Err: err, Status: Succeeded}
	if err != nil {
		r.Status = Failed
		r.ExitCode = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			r.ExitCode = exit.ExitCode()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			r.Status = Canceled
		}
	}
	return r
}

type limitedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	n := min(len(data), b.limit-len(b.data))
	b.data = append(b.data, data[:n]...)
	b.truncated = b.truncated || n < len(data)
	return len(data), nil
}
