package greeting

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/apperror"
	"github.com/zhYou/go-cli-starter/internal/validation"
	"testing"
)

func TestGreetingDirectCallRejectsWhitespace(t *testing.T) {
	for _, name := range []string{"", "  ", "\t\n"} {
		_, err := Run(context.Background(), Options{Name: name})
		require.Error(t, err)
		require.Equal(t, apperror.Usage, apperror.KindOf(err))
	}
	result, err := New(validation.New()).Run(context.Background(), Options{Name: " Codex "})
	require.NoError(t, err)
	require.Equal(t, "你好， Codex ！", result.Message)
}
