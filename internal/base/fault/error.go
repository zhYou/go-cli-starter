// Package fault 保存可组合的错误定义，不登记应用领域码。
package fault

import (
	"errors"
	"fmt"
	"maps"
)

type Kind string

const (
	Usage         Kind = "usage"
	Configuration Kind = "configuration"
	Runtime       Kind = "runtime"
)

type Code string
type Params map[string]any

// Definition 由拥有错误的模块定义；同一错误码不依赖全局注册表。
type Definition struct {
	Code Code
	Kind Kind
}
type Error struct {
	Code    Code
	Kind    Kind
	Params  Params
	Message string
	Cause   error
}
type Option func(*Error)

// WithParams 快照参数映射；嵌套值应为不可变值，每个错误拥有独立映射。
func WithParams(params Params) Option {
	snapshot := maps.Clone(params)
	return func(e *Error) { e.Params = maps.Clone(snapshot) }
}
func (d Definition) New(opts ...Option) *Error {
	e := &Error{Code: d.Code, Kind: d.Kind}
	for _, opt := range opts {
		opt(e)
	}
	return e
}
func (d Definition) Wrap(cause error, opts ...Option) error {
	if cause == nil {
		return nil
	}
	e := d.New(opts...)
	e.Cause = cause
	return e
}
func (e *Error) Error() string {
	text := e.Message
	if text == "" {
		text = string(e.Code)
		if len(e.Params) > 0 {
			text += fmt.Sprint(e.Params)
		}
	}
	if e.Cause != nil {
		if text == "" {
			return e.Cause.Error()
		}
		return text + "：" + e.Cause.Error()
	}
	return text
}
func (e *Error) Unwrap() error { return e.Cause }

// New 和 Wrap 保留无消息码的外部错误适配入口。
func New(kind Kind, message string) error { return &Error{Kind: kind, Message: message} }
func Wrap(kind Kind, message string, cause error) error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

// Details 展开聚合错误，保留每项的最外层分类及原始原因。
func Details(err error) []*Error {
	if err == nil {
		return nil
	}
	switch e := err.(type) {
	case *Error:
		return []*Error{e}
	case interface{ Unwrap() []error }:
		var result []*Error
		for _, child := range e.Unwrap() {
			result = append(result, Details(child)...)
		}
		return result
	default:
		if next := errors.Unwrap(err); next != nil {
			return Details(next)
		}
		return []*Error{{Kind: Runtime, Cause: err}}
	}
}

// KindOf 聚合时运行/配置错误优先于调用错误。
func KindOf(err error) Kind {
	kind := Usage
	details := Details(err)
	if len(details) == 0 {
		return Runtime
	}
	for _, e := range details {
		switch e.Kind {
		case Runtime:
			return Runtime
		case Configuration:
			kind = Configuration
		case Usage:
		default:
			return Runtime
		}
	}
	return kind
}
