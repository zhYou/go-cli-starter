package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"go.uber.org/zap"
	"io"
)

func newGreetCommand(opts Options, call *invocation) *cobra.Command {
	var name string
	greet := &cobra.Command{
		Use:     "greet --name 姓名",
		Short:   "输出问候语（可移除的示例）",
		Example: fmt.Sprintf("  %s greet --name Codex\n  %s greet --name Codex --json", opts.Name, opts.Name),
		Args: func(cmd *cobra.Command, args []string) error {
			if err := noArgs(cmd, args); err != nil {
				return err
			}
			return (greeting.Options{Name: name, Localizer: call.localizer}).Validate(cmd.Context(), call.deps.Validator)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := call.deps.Greet(cmd.Context(), greeting.Options{Name: name, Localizer: call.localizer})
			if err != nil {
				return err
			}
			call.logger.Debug(call.localizer.Text("cli.greet_log", nil), zap.String("name", name))
			if call.asJSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			return writeText(cmd.OutOrStdout(), func(w io.Writer) error { _, err := fmt.Fprintln(w, result.Message); return err })
		},
	}
	greet.Flags().StringVar(&name, "name", "", "问候对象的姓名")
	if err := greet.MarkFlagRequired("name"); err != nil {
		panic(err)
	}

	if err := registerResult[greeting.Result](&call.results, greet, nil); err != nil {
		panic(err)
	}
	return greet
}
