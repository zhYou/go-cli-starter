package command

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func observe(ctx context.Context, call Invocation, options Execution) (context.Context, func(error)) {
	start := time.Now()
	operation := call.Operation
	if options.Operation != "" {
		operation = options.Operation
	}
	if operation == "" {
		operation = "tool.run"
	}
	tool := filepath.Base(call.Program)
	var span trace.Span
	if options.Tracer != nil {
		ctx, span = options.Tracer.Start(ctx, operation, trace.WithAttributes(attribute.String("tool", tool)))
	}
	logger := options.Logger
	if logger != nil {
		logger = logger.WithContext(ctx)
		logger.Debugw("开始执行外部工具", "operation", operation, "tool", tool)
	}
	return ctx, func(err error) {
		outcome := "success"
		exit := 0
		if err != nil {
			outcome = "failed"
			exit = -1
			var process *exec.ExitError
			if errors.As(err, &process) {
				exit = process.ExitCode()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				outcome = "timeout"
			} else if errors.Is(err, context.Canceled) {
				outcome = "cancelled"
			}
		}
		if span != nil {
			span.SetAttributes(attribute.String("outcome", outcome), attribute.Int("exit_code", exit))
			if err != nil {
				span.SetStatus(codes.Error, outcome)
				span.RecordError(err)
			}
			span.End()
		}
		if logger != nil {
			logger.Record(logging.ToolCall{Message: "外部工具执行结束", Operation: operation, Tool: tool, Outcome: outcome, ExitCode: exit, Duration: time.Since(start), Err: err})
		}
	}
}
