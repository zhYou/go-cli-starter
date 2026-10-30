package cli

import (
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func resolveCommand(root *cobra.Command, args []string) (*cobra.Command, error) {
	target := root
	for _, name := range args {
		var next *cobra.Command
		for _, child := range target.Commands() {
			if !child.Hidden && (child.Name() == name || slices.Contains(child.Aliases, name)) {
				next = child
				break
			}
		}
		if next == nil {
			return nil, usageProblem("unknown_path", fault.Params{"Value": strconv.Quote(strings.Join(args, " "))})
		}
		target = next
	}
	return target, nil
}

func newHelpCommand(root *cobra.Command) *cobra.Command {
	var target *cobra.Command
	return &cobra.Command{
		Use: "help [命令路径...]", Short: "显示命令帮助",
		Annotations: map[string]string{"skip-config": "true"},
		Args: func(_ *cobra.Command, args []string) error {
			var err error
			target, err = resolveCommand(root, args)
			return err
		},
		RunE: func(_ *cobra.Command, _ []string) error { return target.Help() },
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	return &cobra.Command{
		Use: "completion [bash|zsh|fish|powershell]", Short: "生成 Shell 补全脚本",
		ValidArgs:   shells,
		Annotations: map[string]string{"skip-config": "true"},
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usageProblem("shell_count", fault.Params{"Count": len(args)})
			}
			if len(args) == 1 && !slices.Contains(shells, args[0]) {
				return usageProblem("shell", fault.Params{"Value": strconv.Quote(args[0])})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return writeText(cmd.OutOrStdout(), func(w io.Writer) error {
				switch args[0] {
				case "bash":
					return root.GenBashCompletionV2(w, true)
				case "zsh":
					return root.GenZshCompletion(w)
				case "fish":
					return root.GenFishCompletion(w, true)
				default:
					return root.GenPowerShellCompletionWithDesc(w)
				}
			})
		},
	}
}
