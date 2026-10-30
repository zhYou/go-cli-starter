package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestValidationChineseFields(t *testing.T) {
	type request struct {
		Name string `validate:"required,notblank" label:"--name"`
	}
	v := New(func(engine *validator.Validate) {
		engine.RegisterStructValidation(func(sl validator.StructLevel) {
			r := sl.Current().Interface().(request)
			if r.Name == "禁止" {
				sl.ReportError(r.Name, "--name", "Name", "reserved", "")
			}
		}, request{})
	})
	require.Contains(t, v.Struct(context.Background(), request{}).Error(), "--name")
	require.Error(t, v.Struct(context.Background(), request{"禁止"}))
	require.NoError(t, v.Struct(context.Background(), request{"Codex"}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.True(t, errors.Is(v.Struct(ctx, request{}), context.Canceled))
}
