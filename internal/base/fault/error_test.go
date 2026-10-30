package fault_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
)

func TestKindsAndCause(t *testing.T) {
	cause := errors.New("磁盘不可用")
	err := fault.Wrap(fault.Configuration, "读取配置失败", cause)
	require.ErrorIs(t, err, cause)
	var typed *fault.Error
	require.ErrorAs(t, err, &typed)
	require.Equal(t, fault.Configuration, fault.KindOf(fmt.Errorf("外层：%w", err)))
	require.Contains(t, err.Error(), "读取配置失败")
	require.Contains(t, err.Error(), "磁盘不可用")
	require.Equal(t, fault.Runtime, fault.KindOf(cause))
	require.Equal(t, fault.Usage, fault.KindOf(fault.New(fault.Usage, "参数无效")))
}
