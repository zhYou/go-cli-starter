package cli

import (
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
)

func canonicalLanguage(value string) (string, bool) {
	switch value {
	case "zh", languageChinese:
		return languageChinese, true
	case languageEnglish:
		return languageEnglish, true
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
		selected = languageEnglish
	}
	// 与 Execute 使用相同的命令查找，子命令选项也可能位于命令名之前。
	// Find 不执行选项 Set；查找错误留给实际执行路径处理。
	current, scan, _ := root.Find(args)
	completion := false
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
			if name == flagLanguage {
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
		} else if current == root && (arg == cobra.ShellCompRequestCmd || arg == cobra.ShellCompNoDescRequestCmd) {
			completion = true
			remaining := scan[i+1:]
			if len(remaining) > 0 {
				remaining = remaining[:len(remaining)-1]
			}
			// 隐藏补全尚未由 Cobra 创建；单独查找其真正要补全的命令。
			current, scan, _ = root.Find(remaining)
			i = -1
		}
	}
	if lang, ok := canonicalLanguage(selected); ok {
		return lang, nil
	}
	fallback := languageEnglish
	if lang, ok := canonicalLanguage(env); ok {
		fallback = lang
	}
	return fallback, usageProblem("language", fault.Params{paramValue: strconv.Quote(selected)})
}

type languageValue struct {
	call        *invocation
	root        *cobra.Command
	preselected bool
}

func (v *languageValue) String() string { return v.call.localizer.Language() }
func (v *languageValue) Type() string   { return "string" }
func (v *languageValue) Set(s string) error {
	lang, ok := canonicalLanguage(s)
	if !ok {
		return usageProblem("language", fault.Params{paramValue: strconv.Quote(s)})
	}
	// Run 已按完整 argv 选择最后生效的语言；解析中间值时只校验。
	if v.preselected {
		return nil
	}
	if err := v.call.localizeTree(v.root, lang); err != nil {
		return fault.Wrap(fault.Runtime, "", err)
	}
	return nil
}
