// Package cli 构造独立的命令树，管理进程的输入输出约定。
package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhYou/go-cli-starter/internal/apperror"
	"github.com/zhYou/go-cli-starter/internal/config"
	"io"
	"strings"
)

// Options 提供进程依赖；每次调用均创建新的命令树。
type Options struct {
	Name, Version, Commit, Date string
	In                          io.Reader
	Out, ErrOut                 io.Writer
	ConfigDir                   string
	LookupEnv                   func(string) (string, bool)
	Dependencies                *Dependencies
}

// Run 执行一次命令并返回约定的退出码，不直接退出进程。
func Run(ctx context.Context, args []string, opts Options) int {
	opts = defaults(opts)
	sink := &recordingWriter{writer: opts.Out}
	opts.Out = sink
	root := newRoot(opts)
	root.SetArgs(args)
	_, err := root.ExecuteContextC(ctx)
	if err == nil && sink.Err() != nil {
		err = apperror.Wrap(apperror.Runtime, "写入结果失败", sink.Err())
	}
	err = classifyExecutionError(err)
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		message := "操作已取消"
		if errors.Is(err, context.DeadlineExceeded) {
			message = "操作已超时"
		}
		_, _ = fmt.Fprintf(opts.ErrOut, "错误：%s\n", message)
		return 130
	}
	_, _ = fmt.Fprintf(opts.ErrOut, "错误：%v\n", err)
	return exitCode(err)
}

// NewRoot 提供命令构造入口，供扩展和嵌入式测试使用。
func NewRoot(opts Options) *cobra.Command { return newRoot(defaults(opts)) }

func defaults(opts Options) Options {
	if opts.Name == "" {
		opts.Name = "go-cli-starter"
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.Commit == "" {
		opts.Commit = "none"
	}
	if opts.Date == "" {
		opts.Date = "unknown"
	}
	if opts.In == nil {
		opts.In = strings.NewReader("")
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.ErrOut == nil {
		opts.ErrOut = io.Discard
	}
	return opts
}

func newRoot(opts Options) *cobra.Command {
	call := newInvocation(opts.Dependencies)
	root := &cobra.Command{Use: opts.Name, Short: "供用户和自动化工具使用的 CLI 模板", Version: opts.Version, Args: noArgs, SilenceErrors: true, SilenceUsage: true, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	root.SetIn(opts.In)
	root.SetOut(opts.Out)
	root.SetErr(opts.ErrOut)
	root.PersistentFlags().BoolVar(&call.asJSON, "json", false, "以 JSON 格式输出结果")
	root.PersistentFlags().StringVar(&call.configPath, "config", "", "从指定的 YAML 文件读取配置")
	root.PersistentFlags().StringVar(&call.logLevel, "log-level", "warn", "日志级别：debug、info、warn、error")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if skipsConfig(cmd, root) {
			return nil
		}
		var override *string
		if cmd.Flags().Changed("log-level") {
			override = &call.logLevel
		}
		cfg, err := call.deps.LoadConfig(config.Options{AppName: opts.Name, Path: call.configPath, UserConfigDir: opts.ConfigDir, Context: cmd.Context(), Validator: call.deps.Validator, LookupEnv: opts.LookupEnv, LogLevel: override})
		if err != nil {
			return apperror.Wrap(apperror.Configuration, "读取配置失败", err)
		}
		call.logger, err = call.deps.NewLogger(opts.ErrOut, cfg.LogLevel)
		if err != nil {
			return apperror.Wrap(apperror.Configuration, "日志配置无效", err)
		}
		return nil
	}
	root.CompletionOptions.DisableDefaultCmd = true
	help := newHelpCommand(root)
	root.SetHelpCommand(help)
	root.AddCommand(newGreetCommand(opts, call), newVersionCommand(opts, call), newSchemaCommand(root, &call.results), newCompletionCommand(root))
	configureChineseHelp(root)
	configureChineseHelp(help)
	protectCommand(root)
	protectCommand(help)
	return root
}
func skipsConfig(cmd, root *cobra.Command) bool {
	if cmd == root || cmd.Annotations["skip-config"] == "true" {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" || c.Name() == cobra.ShellCompRequestCmd || c.Name() == cobra.ShellCompNoDescRequestCmd {
			return true
		}
	}
	return false
}
