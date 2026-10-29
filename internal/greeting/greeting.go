// Package greeting is a removable example of business logic independent of Cobra.
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
	return Result{Message: fmt.Sprintf("Hello, %s!", opts.Name)}, nil
}
