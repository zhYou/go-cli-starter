package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/base/validation"
	"os"
	"strconv"
	"strings"
	"sync"
)

var loadBundle = sync.OnceValues(func() (*messages.Bundle, error) {
	return messages.NewBundle(Catalog(), config.Catalog(), greeting.Catalog(), paths.Catalog(), validation.Catalog())
})

func newLocalizer(lang string) *messages.Localizer {
	b, err := loadBundle()
	if err != nil {
		b, _ = messages.NewBundle()
	}
	return b.Localizer(lang)
}
func canonicalLanguage(value string) (string, bool) {
	switch value {
	case "zh", "zh-CN":
		return "zh-CN", true
	case "en":
		return "en", true
	default:
		return "", false
	}
}
func usageProblem(code string, params fault.Params) error {
	return (fault.Definition{Code: fault.Code("cli." + code), Kind: fault.Usage}).New(fault.WithParams(params))
}
func languageEnvironment(opts Options) string {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, ok := lookup(paths.EnvPrefix(opts.Name) + "_LANG")
	if !ok {
		return ""
	}
	return value
}

// selectLanguage 依据命令树选项元数据跳过值，不把值中的 --lang 当选项。
func selectLanguage(root *cobra.Command, args []string, env string) (string, error) {
	selected := env
	if selected == "" {
		selected = "zh-CN"
	}
	current := root
	completion := false
	scan := args
	findFlag := func(name string, short bool) *pflag.Flag {
		for cmd := current; cmd != nil; cmd = cmd.Parent() {
			for _, set := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
				var f *pflag.Flag
				if short {
					f = set.ShorthandLookup(name)
				} else {
					f = set.Lookup(name)
				}
				if f != nil {
					return f
				}
			}
		}
		return nil
	}
	for i := 0; i < len(scan); i++ {
		arg := scan[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--") {
			name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			flag := findFlag(name, false)
			if flag == nil {
				continue
			}
			if !hasValue && flag.NoOptDefVal == "" {
				if i+1 == len(scan) {
					if completion {
						break
					}
					continue
				}
				i++
				value = scan[i]
			}
			if name == "lang" {
				selected = value
			}
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			short := strings.TrimPrefix(arg, "-")
			for j := 0; j < len(short); j++ {
				f := findFlag(short[j:j+1], true)
				if f != nil && f.NoOptDefVal == "" {
					if j+1 == len(short) {
						i++
					}
					break
				}
			}
		} else {
			if arg == cobra.ShellCompRequestCmd || arg == cobra.ShellCompNoDescRequestCmd {
				completion = true
				if len(scan) > i+1 {
					scan = scan[:len(scan)-1]
				}
				continue
			}
			for _, child := range current.Commands() {
				if child.Name() == arg || child.HasAlias(arg) {
					current = child
					break
				}
			}
		}
	}
	if lang, ok := canonicalLanguage(selected); ok {
		return lang, nil
	}
	fallback := "zh-CN"
	if lang, ok := canonicalLanguage(env); ok {
		fallback = lang
	}
	return fallback, usageProblem("language", fault.Params{"Value": strconv.Quote(selected)})
}

type languageValue struct {
	call *invocation
	root *cobra.Command
}

func (v *languageValue) String() string { return v.call.localizer.Language() }
func (v *languageValue) Type() string   { return "string" }
func (v *languageValue) Set(s string) error {
	lang, ok := canonicalLanguage(s)
	if !ok {
		return usageProblem("language", fault.Params{"Value": strconv.Quote(s)})
	}
	v.call.localizer = newLocalizer(lang)
	localizeTree(v.root, v.call.localizer)
	return nil
}
func localizeTree(root *cobra.Command, l *messages.Localizer) {
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		key := cmd.Name()
		if cmd == root {
			key = "root"
		}
		ids := map[string]string{"root": "root", "greet": "greet", "help": "help", "schema": "schema", "completion": "completion", "version": "version"}
		if id, ok := ids[key]; ok {
			cmd.Short = l.Text("cli."+id, nil)
		}
		if key == "greet" || key == "help" || key == "schema" {
			cmd.Use = l.Text("cli."+key+"_use", nil)
		}
		flagIDs := map[string]string{"json": "json", "config": "config", "log-level": "log_level", "lang": "lang", "name": "name", "help": "help_flag", "version": "version_flag"}
		for _, set := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
			set.VisitAll(func(f *pflag.Flag) {
				if id, ok := flagIDs[f.Name]; ok {
					f.Usage = l.Text("cli."+id, nil)
				}
			})
		}
		cmd.SetVersionTemplate(l.Text("cli.version_template", map[string]any{"Name": "{{.Name}}", "Version": "{{.Version}}"}) + "\n")
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
}
