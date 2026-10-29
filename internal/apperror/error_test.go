package apperror_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/apperror"
)

func TestKindsAndCause(t *testing.T) {
	cause := errors.New("磁盘不可用")
	err := apperror.Wrap(apperror.Configuration, "读取配置失败", cause)
	require.ErrorIs(t, err, cause)
	var typed *apperror.Error
	require.ErrorAs(t, err, &typed)
	require.Equal(t, apperror.Configuration, apperror.KindOf(fmt.Errorf("外层：%w", err)))
	require.Contains(t, err.Error(), "读取配置失败")
	require.Contains(t, err.Error(), "磁盘不可用")
	require.Equal(t, apperror.Runtime, apperror.KindOf(cause))
	require.Equal(t, apperror.Usage, apperror.KindOf(apperror.New(apperror.Usage, "参数无效")))
}
