package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func noArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("不接受位置参数或未知命令：%q", args[0])
	}
	return nil
}

// 根据结构化错误生成中文提示，避免替换用户输入中的英文片段。
func chineseFlagError(_ *cobra.Command, err error) error {
	var missing *pflag.NotExistError
	var required *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &missing):
		return fmt.Errorf("未知选项 %q", missing.GetSpecifiedName())
	case errors.As(err, &required):
		return fmt.Errorf("选项 --%s 需要一个值", required.GetFlag().Name)
	case errors.As(err, &invalid):
		return fmt.Errorf("选项 --%s 的值 %q 无效，要求类型为 %s", invalid.GetFlag().Name, invalid.GetValue(), invalid.GetFlag().Value.Type())
	case errors.As(err, &syntax):
		return fmt.Errorf("选项语法错误：%q", syntax.GetSpecifiedFlag())
	default:
		return fmt.Errorf("解析选项失败：%w", err)
	}
}

func configureChineseHelp(cmd *cobra.Command) {
	cmd.InitDefaultHelpFlag()
	cmd.Flags().Lookup("help").Usage = "显示帮助"
	if cmd.Version != "" {
		cmd.InitDefaultVersionFlag()
		cmd.Flags().Lookup("version").Usage = "显示版本信息"
	}
	cmd.SetVersionTemplate("{{.Name}} 版本 {{.Version}}\n")
	cmd.SetFlagErrorFunc(chineseFlagError)
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		_, _ = fmt.Fprint(c.OutOrStdout(), chineseHelp(c, true))
	})
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		_, err := fmt.Fprint(c.ErrOrStderr(), chineseHelp(c, false))
		return err
	})
	for _, child := range cmd.Commands() {
		configureChineseHelp(child)
	}
}

func chineseHelp(cmd *cobra.Command, description bool) string {
	var out strings.Builder
	if description {
		if cmd.Long != "" {
			fmt.Fprintf(&out, "%s\n\n", cmd.Long)
		} else if cmd.Short != "" {
			fmt.Fprintf(&out, "%s\n\n", cmd.Short)
		}
	}
	fmt.Fprintln(&out, "用法：")
	// 直接按命令元数据生成中文用法，保留命令名和参数占位符。
	use := cmd.CommandPath()
	if _, args, ok := strings.Cut(cmd.Use, " "); ok {
		use += " " + args
	}
	if cmd.HasAvailableFlags() && !cmd.DisableFlagsInUseLine {
		use += " [选项]"
	}
	fmt.Fprintf(&out, "  %s\n", use)
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&out, "  %s [命令]\n", cmd.CommandPath())
	}
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&out, "\n别名：\n  %s\n", strings.Join(cmd.Aliases, "、"))
	}
	if cmd.Example != "" {
		fmt.Fprintf(&out, "\n示例：\n%s\n", cmd.Example)
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(&out, "\n可用命令：")
		columns := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
		for _, child := range cmd.Commands() {
			if child.IsAvailableCommand() || child.Name() == "help" {
				_, _ = fmt.Fprintf(columns, "  %s\t%s\n", child.Name(), child.Short)
			}
		}
		_ = columns.Flush()
	}
	writeChineseFlags(&out, "选项", cmd.LocalNonPersistentFlags())
	writeChineseFlags(&out, "全局选项", cmd.InheritedFlags())
	writeChineseFlags(&out, "全局选项", cmd.PersistentFlags())
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&out, "\n使用 %q 查看命令的详细帮助。\n", cmd.CommandPath()+" [命令] --help")
	}
	return out.String()
}

func writeChineseFlags(out *strings.Builder, title string, flags *pflag.FlagSet) {
	if !flags.HasAvailableFlags() {
		return
	}
	fmt.Fprintf(out, "\n%s：\n", title)
	columns := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		label := "    --" + flag.Name
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			label = "-" + flag.Shorthand + ", --" + flag.Name
		}
		if flag.NoOptDefVal == "" {
			label += " <" + flag.Value.Type() + ">"
		}
		description := flag.Usage
		if flag.Name == "help" {
			description = "显示帮助"
		}
		if len(flag.Annotations[cobra.BashCompOneRequiredFlag]) > 0 {
			description += "（必填）"
		}
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "0" {
			description += fmt.Sprintf("（默认值：%q）", flag.DefValue)
		}
		_, _ = fmt.Fprintf(columns, "  %s\t%s\n", label, description)
	})
	_ = columns.Flush()
}
