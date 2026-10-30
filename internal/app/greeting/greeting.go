// Package greeting 是独立于 Cobra 的业务逻辑示例，可按需移除。
package greeting

import (
	"context"
	"sync"

	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
)

// Options 声明本次调用的显式依赖与覆盖选项。
type Options struct {
	Localizer *messages.Localizer `validate:"-"`
	Name      string              `validate:"required,notblank" label:"--name"`
}

// Validate 使用调用者的校验器检查业务输入。
func (o Options) Validate(ctx context.Context, v *validation.Validator) error {
	return v.Struct(ctx, o)
}

// Service 持有可复用的业务校验依赖。
type Service struct{ validator *validation.Validator }

// New 创建使用给定校验器的问候服务；nil 使用默认规则。
func New(v *validation.Validator) *Service {
	if v == nil {
		v = validation.New()
	}
	return &Service{validator: v}
}

// Result 保存命令成功时的有类型结果。
type Result struct {
	Message string `json:"message"`
}

// Run 执行一次业务调用，先校验输入再生成结果。
func Run(ctx context.Context, opts Options) (Result, error) {
	return New(validation.New()).Run(ctx, opts)
}

// Run 执行一次业务调用，先校验输入再生成结果。
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
