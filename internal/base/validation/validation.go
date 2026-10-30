// Package validation 提供实例级的业务校验与中文字段提示。
package validation

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"reflect"
	"strings"
)

type Validator struct{ engine *validator.Validate }
type fieldError struct {
	message string
	cause   error
}

func (e fieldError) Error() string { return e.message }
func (e fieldError) Unwrap() error { return e.cause }

// New 在并发使用前完成规则注册；可通过 callback 组合条件和业务规则。
func New(register ...func(*validator.Validate)) *Validator {
	engine := validator.New(validator.WithRequiredStructEnabled())
	engine.RegisterTagNameFunc(func(f reflect.StructField) string {
		if name := f.Tag.Get("label"); name != "" {
			return name
		}
		return f.Name
	})
	if err := engine.RegisterValidation("notblank", func(f validator.FieldLevel) bool { return strings.TrimSpace(f.Field().String()) != "" }); err != nil {
		panic(err)
	}
	for _, configure := range register {
		configure(engine)
	}
	return &Validator{engine: engine}
}
func (v *Validator) Struct(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := v.engine.StructCtx(ctx, value); err != nil {
		if fields, ok := err.(validator.ValidationErrors); ok {
			messages := make([]string, 0, len(fields))
			for _, field := range fields {
				reason := "不满足校验规则"
				switch field.Tag() {
				case "required":
					reason = "必须提供"
				case "notblank":
					reason = "不能为空白"
				case "oneof":
					reason = "必须为 " + field.Param()
				}
				messages = append(messages, fmt.Sprintf("%s %s", field.Field(), reason))
			}
			return fault.Wrap(fault.Usage, "", fieldError{strings.Join(messages, "；"), err})
		}
		return fault.Wrap(fault.Runtime, "校验定义无效", err)
	}
	return ctx.Err()
}
