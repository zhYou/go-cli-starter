// Package cli constructs independent command trees and owns the process IO contract.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zhYou/go-cli-starter/internal/config"
	"github.com/zhYou/go-cli-starter/internal/greeting"
)

// Options supplies process dependencies; every invocation gets a fresh command tree.
type Options struct {
	Name, Version, Commit, Date string
	In                          io.Reader
	Out, ErrOut                 io.Writer
	ConfigDir                   string
	LookupEnv                   func(string) (string, bool)
}

type execution struct{ started bool }

// Run executes once and returns a documented process exit code. It never exits.
func Run(ctx context.Context, args []string, opts Options) int {
	opts = defaults(opts)
	state := &execution{}
	root := newRoot(opts, state)
	root.SetArgs(args)
	_, err := root.ExecuteContextC(ctx)
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintf(opts.ErrOut, "Error: %v\n", err)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 130
	}
	if state.started {
		return 1
	}
	return 2
}

// NewRoot exposes command construction for extension and embedded command tests.
func NewRoot(opts Options) *cobra.Command { return newRoot(defaults(opts), &execution{}) }

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

func newRoot(opts Options, state *execution) *cobra.Command {
	var asJSON bool
	var configPath, logLevel string
	logger := slog.New(slog.NewTextHandler(opts.ErrOut, &slog.HandlerOptions{Level: slog.LevelWarn}))
	root := &cobra.Command{
		Use:           opts.Name,
		Short:         "A reusable CLI for people and automation",
		Version:       opts.Version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetIn(opts.In)
	root.SetOut(opts.Out)
	root.SetErr(opts.ErrOut)
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "Write the result as JSON")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Read configuration from this YAML file")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "Log level: debug, info, warn, error")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		state.started = true
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if skipsConfig(cmd, root) {
			return nil
		}
		var override *string
		if cmd.Flags().Changed("log-level") {
			override = &logLevel
		}
		cfg, err := config.Load(config.Options{
			AppName: opts.Name, Path: configPath, UserConfigDir: opts.ConfigDir,
			LookupEnv: opts.LookupEnv, LogLevel: override,
		})
		if err != nil {
			return err
		}
		var level slog.Level
		if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
			return err
		}
		logger = slog.New(slog.NewTextHandler(opts.ErrOut, &slog.HandlerOptions{Level: level}))
		return nil
	}
	var name string
	greet := &cobra.Command{
		Use:     "greet --name NAME",
		Short:   "Print a greeting (removable example)",
		Example: fmt.Sprintf("  %s greet --name Codex\n  %s greet --name Codex --json", opts.Name, opts.Name),
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return errors.New("--name is required and must not be blank")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := greeting.Run(cmd.Context(), greeting.Options{Name: name})
			if err != nil {
				return err
			}
			logger.DebugContext(cmd.Context(), "greeting created", "name", name)
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return err
		},
	}
	greet.Flags().StringVar(&name, "name", "", "Name to greet")
	if err := greet.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	version := &cobra.Command{
		Use: "version", Short: "Print version and build information", Args: cobra.NoArgs,
		Annotations: map[string]string{"skip-config": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Commit  string `json:"commit"`
					Date    string `json:"date"`
				}{opts.Name, opts.Version, opts.Commit, opts.Date})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s (commit %s, built %s)\n", opts.Name, opts.Version, opts.Commit, opts.Date)
			return err
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(newHelpCommand(root))
	root.AddCommand(greet, version, newSchemaCommand(root), newCompletionCommand(root))
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
