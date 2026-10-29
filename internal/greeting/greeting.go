// Package greeting 是独立于 Cobra 的业务逻辑示例，可按需移除。
package greeting

import (
	"context"
	"fmt"
)

type Options struct{ Name string }

type Result struct {
	Message string `json:"message"`
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return Result{Message: fmt.Sprintf("你好，%s！", opts.Name)}, nil
}
