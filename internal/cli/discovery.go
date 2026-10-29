package cli

import (
	"fmt"
	"slices"
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
			return nil, fmt.Errorf("unknown command path %q", strings.Join(args, " "))
		}
		target = next
	}
	return target, nil
}

func newHelpCommand(root *cobra.Command) *cobra.Command {
	var target *cobra.Command
	return &cobra.Command{
		Use: "help [command path...]", Short: "Show help for a command",
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
		Use: "completion [bash|zsh|fish|powershell]", Short: "Generate a shell completion script",
		ValidArgs:   shells,
		Annotations: map[string]string{"skip-config": "true"},
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return err
			}
			if len(args) == 1 && !slices.Contains(shells, args[0]) {
				return fmt.Errorf("unsupported shell %q: use bash, zsh, fish, or powershell", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
		},
	}
}
