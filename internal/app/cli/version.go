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
				return writeJSON(cmd.OutOrStdout(), versionResult{opts.Name, opts.Version, opts.Commit, opts.Date})
			}
			return writeText(cmd.OutOrStdout(), func(w io.Writer) error {
				_, err := fmt.Fprintln(w, call.localizer.Text("cli.version_text", map[string]any{"Name": opts.Name, "Version": opts.Version, "Commit": opts.Commit, "Date": opts.Date}))
				return err
			})
		},
	}

	if err := registerResult[versionResult](&call.results, version, nil); err != nil {
		panic(err)
	}
	return version
}

type versionResult struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}
