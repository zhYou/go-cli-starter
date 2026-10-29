package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
)

func newVersionCommand(opts Options, call *invocation) *cobra.Command {
	version := &cobra.Command{
		Use: "version", Short: "显示版本和构建信息", Args: noArgs,
		Annotations: map[string]string{"skip-config": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if call.asJSON {
				return writeJSON(cmd.OutOrStdout(), struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Commit  string `json:"commit"`
					Date    string `json:"date"`
				}{opts.Name, opts.Version, opts.Commit, opts.Date})
			}
			return writeText(cmd.OutOrStdout(), func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s 版本 %s（提交 %s，构建时间 %s）\n", opts.Name, opts.Version, opts.Commit, opts.Date)
				return err
			})
		},
	}

	return version
}
