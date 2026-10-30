package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
)

func TestExitCodeAndUnwrap(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"成功", nil, 0},
		{"调用", fault.New(fault.Usage, "参数无效"), 2},
		{"配置", fault.New(fault.Configuration, "配置无效"), 1},
		{"运行", fault.New(fault.Runtime, "执行失败"), 1},
		{"普通业务错误", errors.New("执行失败"), 1},
		{"取消", fault.Wrap(fault.Runtime, "请求失败", context.Canceled), 130},
		{"超时", fmt.Errorf("请求失败：%w", context.DeadlineExceeded), 130},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, exitCode(tc.err)) })
	}
}

func TestBusinessErrorWithCustomPreRun(t *testing.T) {
	cause := errors.New("业务执行失败")
	root := metadataRoot(Options{Out: io.Discard, ErrOut: io.Discard})
	root.AddCommand(&cobra.Command{
		Use:               "fail",
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE:              wrapRun(func(*cobra.Command, []string) error { return cause }),
	})
	root.SetArgs([]string{"fail"})
	err := classifyExecutionError(root.Execute())
	require.ErrorIs(t, err, cause)
	require.Equal(t, 1, exitCode(err))
}

func TestCobraRequiredFlagIsUsageError(t *testing.T) {
	root := metadataRoot(Options{Out: io.Discard, ErrOut: io.Discard})
	command := &cobra.Command{Use: "required", RunE: wrapRun(func(*cobra.Command, []string) error { return nil })}
	command.Flags().String("token", "", "令牌")
	require.NoError(t, command.MarkFlagRequired("token"))
	root.AddCommand(command)
	root.SetArgs([]string{"required"})
	err := classifyExecutionError(root.Execute())
	require.Error(t, err)
	require.Equal(t, 2, exitCode(err))
}
