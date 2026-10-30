// Package greeting 是独立于 Cobra 的业务逻辑示例，可按需移除。
package greeting

import (
	"context"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
	"sync"
)

type Options struct {
	Localizer *messages.Localizer `validate:"-"`
	Name      string              `validate:"required,notblank" label:"--name"`
}

func (o Options) Validate(ctx context.Context, v *validation.Validator) error {
	return v.Struct(ctx, o)
}

type Service struct{ validator *validation.Validator }

func New(v *validation.Validator) *Service {
	if v == nil {
		v = validation.New()
	}
	return &Service{validator: v}
}

type Result struct {
	Message string `json:"message"`
}

func Run(ctx context.Context, opts Options) (Result, error) {
	return New(validation.New()).Run(ctx, opts)
}

func (s *Service) Run(ctx context.Context, opts Options) (Result, error) {
	if err := opts.Validate(ctx, s.validator); err != nil {
		return Result{}, err
	}
	l := opts.Localizer
	if l == nil {
		b, err := defaultBundle()
		if err != nil {
			return Result{}, err
		}
		l = b.Localizer("zh-CN")
	}
	return Result{Message: l.Text("greeting.message", map[string]any{"Name": opts.Name})}, nil
}

var defaultBundle = sync.OnceValues(func() (*messages.Bundle, error) { return messages.NewBundle(Catalog()) })
