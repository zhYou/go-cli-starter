// Package cli 构造独立的命令树，管理进程的输入输出约定。
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/session"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/iostreams"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
)

// Options 提供进程依赖；每次调用均创建新的命令树。
type Options struct {
	Name, Version, Commit, Date string
	In                          io.Reader
	Out, ErrOut                 io.Writer
	ConfigDir                   string
	// Directories 显式覆写应用目录，供嵌入调用及测试隔离；ConfigDir 仅选择配置目录。
	Directories  *paths.Directories
	Language     string
	LookupEnv    func(string) (string, bool)
	Dependencies *Dependencies
}

// Run 执行一次命令并返回约定的退出码，不直接退出进程。
func Run(ctx context.Context, args []string, opts Options) (code int) {
	code = 1
	started := time.Now()
	var executionErr error
	opts = defaults(opts)
	// Cobra、日志、诊断回调与最终错误输出共用调用级 stderr 写锁。
	opts.ErrOut = iostreams.Synchronize(opts.ErrOut, nil)
	sink := &recordingWriter{writer: opts.Out}
	opts.Out = sink
	//nolint:contextcheck // Cobra 构造阶段不执行请求，下面 ExecuteContextC 才注入调用者 ctx。
	root, call := buildRoot(opts)
	defer func() {
		if call.session != nil {
			call.session.Finish(ctx, logging.CommandResult{ExitCode: code, Duration: time.Since(started), Err: executionErr})
		}
	}()
	envLang := languageEnvironment(opts)
	if opts.Language != "" {
		envLang = opts.Language
	}
	lang, err := selectLanguage(root, args, envLang)
	call.localizer = newLocalizer(lang)
	localizeTree(root, call.localizer)
	if err == nil {
		root.SetArgs(args)
		_, err = root.ExecuteContextC(ctx)
	}
	if err == nil && sink.Err() != nil {
		err = (fault.Definition{Code: "cli.write", Kind: fault.Runtime}).Wrap(sink.Err())
	}
	err = classifyExecutionError(err)
	executionErr = err
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		message := call.localizer.Text("cli.cancelled", nil)
		if errors.Is(err, context.DeadlineExceeded) {
			message = call.localizer.Text("cli.deadline", nil)
		}
		_, _ = fmt.Fprintln(opts.ErrOut, call.localizer.Text("cli.error", map[string]any{paramMessage: message}))
		return 130
	}
	_, _ = fmt.Fprintln(opts.ErrOut, call.localizer.Text("cli.error", map[string]any{paramMessage: call.localizer.Render(err)}))
	return exitCode(err)
}

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
	streams := (iostreams.IOStreams{In: opts.In, Out: opts.Out, ErrOut: opts.ErrOut}).WithDefaults()
	opts.In, opts.Out, opts.ErrOut = streams.In, streams.Out, streams.ErrOut
	return opts
}

func buildRoot(opts Options) (*cobra.Command, *invocation) {
	call := newInvocation(opts.Dependencies)
	lang := opts.Language
	if lang == "" {
		lang = languageEnvironment(opts)
	}
	if normalized, ok := canonicalLanguage(lang); ok {
		lang = normalized
	} else {
		lang = languageChinese
	}
	call.localizer = newLocalizer(lang)
	root := &cobra.Command{Use: opts.Name, Short: "供用户和自动化工具使用的 CLI 模板", Version: opts.Version, Args: noArgs, SilenceErrors: true, SilenceUsage: true, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	root.SetIn(opts.In)
	root.SetOut(opts.Out)
	root.SetErr(opts.ErrOut)
	root.PersistentFlags().BoolVar(&call.asJSON, flagJSON, false, "以 JSON 格式输出结果")
	root.PersistentFlags().StringVar(&call.configPath, flagConfig, "", "从指定的 TOML 文件读取配置")
	root.PersistentFlags().StringVar(&call.logLevel, flagLogLevel, defaultLogLevel, "日志级别：debug、info、warn、error")
	root.PersistentFlags().Var(&languageValue{call: call, root: root}, flagLanguage, "显示语言")
	root.PersistentFlags().Lookup(flagLanguage).DefValue = languageChinese
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := validateFlags(cmd, call); err != nil {
			return err
		}
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if skipsConfig(cmd, root) {
			return nil
		}
		var override *string
		if cmd.Flags().Changed(flagLogLevel) {
			override = &call.logLevel
		}
		cfg, err := call.deps.LoadConfig(config.Options{AppName: opts.Name, Path: call.configPath, Explicit: cmd.Flags().Changed(flagConfig), UserConfigDir: opts.ConfigDir, Context: cmd.Context(), Validator: call.deps.Validator, LookupEnv: opts.LookupEnv, LogLevel: override})
		if err != nil {
			return (fault.Definition{Code: "cli.config_failed", Kind: fault.Configuration}).Wrap(err)
		}
		ctx, owned := session.Begin(cmd.Context(), cfg, logging.RunInfo{Component: opts.Name, Command: cmd.CommandPath(), Version: opts.Version, Commit: opts.Commit}, session.Options{
			Paths: paths.Options{AppName: opts.Name, LookupEnv: opts.LookupEnv}, Directories: opts.Directories, Stderr: opts.ErrOut, ExternalLogger: call.deps.Logger, NewLogger: call.deps.NewLogger, Localizer: call.localizer,
		}, func(err error) {
			_, _ = fmt.Fprintln(opts.ErrOut, call.localizer.Text("cli.warning", map[string]any{paramMessage: call.localizer.Render(err)}))
		})
		call.session = owned
		call.logger = owned.Logger
		cmd.SetContext(ctx)
		return nil
	}
	root.CompletionOptions.DisableDefaultCmd = true
	help := newHelpCommand(root)
	root.SetHelpCommand(help)
	root.AddCommand(newGreetCommand(opts, call), newVersionCommand(opts, call), newSchemaCommand(root, &call.results), newCompletionCommand(root))
	configureHelp(root, call)
	configureHelp(help, call)
	protectCommand(root)
	protectCommand(help)
	localizeTree(root, call.localizer)
	localizeTreeHelp(help, call)
	return root, call
}
func skipsConfig(cmd, root *cobra.Command) bool {
	if cmd == root || cmd.Annotations[annotationSkipConfig] == annotationEnabled {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == commandCompletion || c.Name() == cobra.ShellCompRequestCmd || c.Name() == cobra.ShellCompNoDescRequestCmd {
			return true
		}
	}
	return false
}

// validateFlags 在配置/资源入口前根据 Cobra 元数据检查必填和组合。
func validateFlags(cmd *cobra.Command, call *invocation) error {
	if f := cmd.Flags().Lookup(flagLogLevel); f != nil && f.Changed {
		switch f.Value.String() {
		case "debug", defaultLogLevel, "warn", "error":
		default:
			return usageProblem("invalid_flag", fault.Params{paramFlag: flagLogLevel, paramValue: strconv.Quote(f.Value.String()), "Type": "debug|info|warn|error"})
		}
	}

	var missing []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if len(f.Annotations[cobra.BashCompOneRequiredFlag]) > 0 && !f.Changed {
			missing = append(missing, "--"+f.Name)
		}
	})
	if len(missing) > 0 {
		return usageProblem("required", fault.Params{"Flags": strings.Join(missing, ", ")})
	}
	rules := []struct{ annotation, id string }{
		{"cobra_annotation_required_if_others_set", "together"},
		{"cobra_annotation_one_required", "one"},
		{"cobra_annotation_mutually_exclusive", "exclusive"},
	}
	for _, rule := range rules {
		var failure error
		seen := map[string]bool{}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			for _, group := range f.Annotations[rule.annotation] {
				if seen[group] {
					continue
				}
				seen[group] = true
				names := strings.Fields(group)
				count := 0
				present := true
				for _, name := range names {
					other := cmd.Flags().Lookup(name)
					if other == nil {
						present = false
						break
					}
					if other.Changed {
						count++
					}
				}
				if !present {
					continue
				}
				invalid := rule.id == "together" && count > 0 && count < len(names) || rule.id == "one" && count == 0 || rule.id == "exclusive" && count > 1
				if invalid && failure == nil {
					failure = usageProblem("group", fault.Params{"Flags": group, "Rule": call.localizer.Text("cli.group_"+rule.id, nil)})
				}
			}
		})
		if failure != nil {
			return failure
		}
	}
	return nil
}
func localizeTreeHelp(help *cobra.Command, call *invocation) {
	help.Short = call.localizer.Text("cli.help", nil)
	help.Use = call.localizer.Text("cli.help_use", nil)
}
