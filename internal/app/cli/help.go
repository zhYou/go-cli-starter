package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/messages"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func noArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return usageProblem("no_args", fault.Params{paramValue: strconv.Quote(args[0])})
	}
	return nil
}

// 根据结构化错误生成中文提示，避免替换用户输入中的英文片段。
func localizedFlagError(_ *cobra.Command, err error) error {
	var structured *fault.Error
	if errors.As(err, &structured) {
		return structured
	}
	var missing *pflag.NotExistError
	var required *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &missing):
		return usageProblem("unknown_flag", fault.Params{paramFlag: strconv.Quote(missing.GetSpecifiedName())})
	case errors.As(err, &required):
		return usageProblem("required_flag", fault.Params{paramFlag: required.GetFlag().Name})
	case errors.As(err, &invalid):
		return usageProblem("invalid_flag", fault.Params{paramFlag: invalid.GetFlag().Name, paramValue: strconv.Quote(invalid.GetValue()), "Type": invalid.GetFlag().Value.Type()})
	case errors.As(err, &syntax):
		return usageProblem("syntax", fault.Params{paramValue: strconv.Quote(syntax.GetSpecifiedFlag())})
	default:
		return (fault.Definition{Code: "cli.parse", Kind: fault.Usage}).Wrap(err)
	}
}

func configureHelp(cmd *cobra.Command, call *invocation) {
	cmd.InitDefaultHelpFlag()
	cmd.Flags().Lookup(flagHelp).Usage = "显示帮助"
	if cmd.Version != "" {
		cmd.InitDefaultVersionFlag()
		cmd.Flags().Lookup(flagVersion).Usage = "显示版本信息"
	}
	cmd.SetVersionTemplate("{{.Name}} 版本 {{.Version}}\n")
	cmd.SetFlagErrorFunc(localizedFlagError)
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		_, _ = fmt.Fprint(c.OutOrStdout(), localizedHelp(c, true, call.localizer))
	})
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		_, err := fmt.Fprint(c.ErrOrStderr(), localizedHelp(c, false, call.localizer))
		return err
	})
	for _, child := range cmd.Commands() {
		configureHelp(child, call)
	}
}

func localizedHelp(cmd *cobra.Command, description bool, l *messages.Localizer) string {
	localizeTree(cmd.Root(), l)
	var out strings.Builder
	if description {
		if cmd.Long != "" {
			fmt.Fprintf(&out, "%s\n\n", cmd.Long)
		} else if cmd.Short != "" {
			fmt.Fprintf(&out, "%s\n\n", cmd.Short)
		}
	}
	fmt.Fprintln(&out, l.Text("help.usage", nil))
	// 直接按命令元数据生成中文用法，保留命令名和参数占位符。
	use := cmd.CommandPath()
	if _, args, ok := strings.Cut(cmd.Use, " "); ok {
		use += " " + args
	}
	if cmd.HasAvailableFlags() && !cmd.DisableFlagsInUseLine {
		use += l.Text("help.flags_suffix", nil)
	}
	fmt.Fprintf(&out, "  %s\n", use)
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&out, "  %s%s\n", cmd.CommandPath(), l.Text("help.command_suffix", nil))
	}
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&out, "\n%s\n  %s\n", l.Text("help.aliases", nil), strings.Join(cmd.Aliases, ", "))
	}
	if cmd.Example != "" {
		fmt.Fprintf(&out, "\n%s\n%s\n", l.Text("help.examples", nil), cmd.Example)
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(&out, "\n"+l.Text("help.commands", nil))
		columns := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
		for _, child := range cmd.Commands() {
			if child.IsAvailableCommand() || child.Name() == commandHelp {
				_, _ = fmt.Fprintf(columns, "  %s\t%s\n", child.Name(), child.Short)
			}
		}
		_ = columns.Flush()
	}
	writeLocalizedFlags(&out, l.Text("help.flags", nil), cmd.LocalNonPersistentFlags(), l)
	writeLocalizedFlags(&out, l.Text("help.global_flags", nil), cmd.InheritedFlags(), l)
	writeLocalizedFlags(&out, l.Text("help.global_flags", nil), cmd.PersistentFlags(), l)
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintln(&out, "\n"+l.Text("help.detail", map[string]any{"Command": strconv.Quote(cmd.CommandPath() + l.Text("help.command_suffix", nil) + " --help")}))
	}
	return out.String()
}

func writeLocalizedFlags(out *strings.Builder, title string, flags *pflag.FlagSet, l *messages.Localizer) {
	if !flags.HasAvailableFlags() {
		return
	}
	fmt.Fprintf(out, "\n%s:\n", title)
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
		if flag.Name == flagHelp {
			description = l.Text("cli.help_flag", nil)
		}
		if len(flag.Annotations[cobra.BashCompOneRequiredFlag]) > 0 {
			description += l.Text("help.required", nil)
		}
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "0" {
			description += l.Text("help.default", map[string]any{paramValue: strconv.Quote(flag.DefValue)})
		}
		_, _ = fmt.Fprintf(columns, "  %s\t%s\n", label, description)
	})
	_ = columns.Flush()
}
