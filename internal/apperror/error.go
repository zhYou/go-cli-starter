// Package apperror 为调用、配置和业务失败保留可识别的错误分类。
package apperror

import "errors"

// Kind 描述错误所属边界。
type Kind string

const (
	Usage         Kind = "usage"
	Configuration Kind = "configuration"
	Runtime       Kind = "runtime"
)

// Error 保留中文上下文和底层原因。
type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Cause.Error()
	}
	return e.Message + "：" + e.Cause.Error()
}

func (e *Error) Unwrap() error { return e.Cause }

func New(kind Kind, message string) error { return &Error{Kind: kind, Message: message} }

func Wrap(kind Kind, message string, cause error) error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

// KindOf 将未分类的错误视为运行失败。
func KindOf(err error) Kind {
	var typed *Error
	if errors.As(err, &typed) {
		switch typed.Kind {
		case Usage, Configuration, Runtime:
			return typed.Kind
		}
	}
	return Runtime
}
