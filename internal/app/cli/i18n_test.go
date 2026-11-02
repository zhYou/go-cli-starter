package cli

import (
	"bytes"
	"context"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"github.com/zhYou/go-cli-starter/internal/locales"

	"github.com/stretchr/testify/require"
)

func TestDefaultLanguageIsEnglishAndIgnoresSystemLocale(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	code, out, stderr := invoke(t, "greet", "--name", "Ada")
	require.Zero(t, code, stderr)
	require.Equal(t, "Hello, Ada!\n", out)
	require.Empty(t, stderr)
}

func TestCorruptCatalogStopsBeforeHelpOutput(t *testing.T) {
	for name, data := range map[string]string{
		"invalid TOML":     "[broken",
		"invalid template": "[\"fixture.message\"]\nother = \"{{\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			loader := func() (*messages.Bundle, error) {
				return messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte(data)}})
			}
			_, _, err := buildRoot(defaults(Options{LookupEnv: noFixtureEnv}), loader)
			require.ErrorContains(t, err, "active.en.toml")
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"--help"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
			require.Equal(t, 1, code, stderr.String())
			require.Empty(t, out.String())
			require.Contains(t, stderr.String(), "active.en.toml")
		})
	}
}

func TestUnboundFlagWithSameNameKeepsItsDescription(t *testing.T) {
	var out bytes.Buffer
	root, call, err := buildRoot(defaults(Options{Out: &out, LookupEnv: noFixtureEnv}), locales.LoadBundle)
	require.NoError(t, err)
	foreign := &cobra.Command{Use: "extra", Short: "extra", RunE: func(*cobra.Command, []string) error { return nil }}
	foreign.Flags().String("name", "", "unrelated name")
	root.AddCommand(foreign)
	require.NoError(t, call.localizeTree(root, call.localizer.Language()))
	root.SetArgs([]string{"extra", "--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "unrelated name")
}

func TestHelpRenderingFailureIsRuntimeAndLeavesStdoutEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, language, replacement string
	}{
		{"missing translation", "en", ""},
		{"template execution error", "en", "other = \"prefix {{index . 1}} suffix\"\n"},
		{"missing template parameter", "en", "other = \"prefix {{.Missing}} suffix\"\n"},
		{"missing Chinese translation", "zh-CN", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle := catalogFixture(t, tc.language, "help.usage", tc.replacement)
			loader := func() (*messages.Bundle, error) { return bundle, nil }
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"--lang", tc.language, "--help"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
			require.Equal(t, 1, code, stderr.String())
			require.Empty(t, out.String())
			require.Contains(t, stderr.String(), "help.usage")
		})
	}
}

func TestVersionRenderingFailureLeavesStdoutEmpty(t *testing.T) {
	bundle := catalogFixture(t, "en", "cli.version_text", "other = \"prefix {{.Missing}} suffix\"\n")
	loader := func() (*messages.Bundle, error) { return bundle, nil }
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{"version"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
	require.Equal(t, 1, code, stderr.String())
	require.Empty(t, out.String())
	require.Contains(t, stderr.String(), "cli.version_text")
}

func catalogFixture(t *testing.T, language, id, replacement string) *messages.Bundle {
	t.Helper()
	files := fstest.MapFS{}
	for _, lang := range []string{"en", "zh-CN"} {
		name := "active." + lang + ".toml"
		data, err := fs.ReadFile(locales.Catalog(), name)
		require.NoError(t, err)
		if lang == language {
			content := string(data)
			header := "[\"" + id + "\"]\n"
			start := strings.Index(content, header)
			require.NotEqual(t, -1, start)
			end := strings.Index(content[start+len(header):], "\n[")
			require.NotEqual(t, -1, end)
			end += start + len(header) + 1
			section := ""
			if replacement != "" {
				section = header + replacement + "\n"
			}
			data = []byte(content[:start] + section + content[end:])
		}
		files[name] = &fstest.MapFile{Data: data}
	}
	bundle, err := messages.NewBundle(files)
	require.NoError(t, err)
	return bundle
}

func TestLanguageOverrideDoesNotRenderUnselectedMetadata(t *testing.T) {
	for _, tc := range []struct {
		missing, selected, want string
	}{
		{"en", "zh", "用法："},
		{"zh-CN", "en", "Usage:"},
	} {
		t.Run(tc.selected, func(t *testing.T) {
			bundle := catalogFixture(t, tc.missing, "cli.greet", "")
			loader := func() (*messages.Bundle, error) { return bundle, nil }
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"--lang", tc.missing, "--lang", tc.selected, "greet", "--help"}, Options{
				Out: &out, ErrOut: &stderr, Language: tc.missing, LookupEnv: noFixtureEnv,
			}, loader)
			require.Zero(t, code, stderr.String())
			require.Contains(t, out.String(), tc.want)
			require.Empty(t, stderr.String())
		})
	}
}

func TestMetadataRenderingFailureStopsBeforeOutput(t *testing.T) {
	for _, id := range []string{"cli.greet", "cli.greet_use", "cli.greet_example", "cli.name", "cli.version_template"} {
		t.Run(id, func(t *testing.T) {
			bundle := catalogFixture(t, "zh-CN", id, "other = \"prefix {{.Missing}} suffix\"\n")
			loader := func() (*messages.Bundle, error) { return bundle, nil }
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"--lang", "zh", "greet", "--help"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
			require.Equal(t, 1, code, stderr.String())
			require.Empty(t, out.String())
			require.Contains(t, stderr.String(), id)
		})
	}
}

func TestAllHelpCallbacksPropagateRenderingFailure(t *testing.T) {
	bundle := catalogFixture(t, "en", "help.usage", "other = \"prefix {{.Missing}} suffix\"\n")
	loader := func() (*messages.Bundle, error) { return bundle, nil }
	for _, args := range [][]string{{}, {"--help"}, {"greet", "--help"}, {"help", "greet"}, {"completion"}} {
		var out, stderr bytes.Buffer
		code := run(context.Background(), args, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
		require.Equal(t, 1, code, "%v: %s", args, stderr.String())
		require.Empty(t, out.String(), "%v", args)
		require.Contains(t, stderr.String(), "help.usage")
	}
}

func TestCommandAndFlagBindingsRenderIndependently(t *testing.T) {
	bundle := catalogWithCommandFixtures(t)
	for _, tc := range []struct {
		language, command, description, usage, example, flag, otherFlag string
	}{
		{"en", "alpha", "Alpha long description", "alpha <item>", "Example alpha", "Alpha name", "Beta name"},
		{"en", "beta", "Beta long description", "beta <item>", "Example beta", "Beta name", "Alpha name"},
		{"zh", "alpha", "甲命令长说明", "alpha <项目>", "甲命令示例", "甲名称", "乙名称"},
		{"zh", "beta", "乙命令长说明", "beta <项目>", "乙命令示例", "乙名称", "甲名称"},
	} {
		t.Run(tc.language+"/"+tc.command, func(t *testing.T) {
			var out bytes.Buffer
			loader := func() (*messages.Bundle, error) { return bundle, nil }
			root, call, err := buildRoot(defaults(Options{Name: "testcli", Language: tc.language, Out: &out, LookupEnv: noFixtureEnv}), loader)
			require.NoError(t, err)
			for _, name := range []string{"alpha", "beta"} {
				cmd := &cobra.Command{Use: name, Short: name, RunE: func(*cobra.Command, []string) error { return nil }}
				cmd.Flags().String("name", "", "original")
				root.AddCommand(cmd)
				call.bindCommand(cmd, commandTexts{short: message("fixture." + name + ".short"), long: message("fixture." + name + ".long"), use: message("fixture." + name + ".use"), example: message("fixture." + name + ".example")})
				call.bindFlag(cmd.Flags().Lookup("name"), message("fixture."+name+".flag"))
				configureHelp(cmd, call)
			}
			require.NoError(t, call.localizeTree(root, tc.language))
			root.SetArgs([]string{tc.command, "--help"})
			require.NoError(t, root.Execute())
			require.NoError(t, call.helpErr)
			require.Contains(t, out.String(), tc.description)
			require.Contains(t, out.String(), tc.usage)
			require.Contains(t, out.String(), tc.example)
			require.Contains(t, out.String(), tc.flag)
			require.NotContains(t, out.String(), tc.otherFlag)
		})
	}
}

func catalogWithCommandFixtures(t *testing.T) *messages.Bundle {
	t.Helper()
	extra := map[string]string{
		"en": `
["fixture.alpha.short"]
other = "Alpha short"
["fixture.alpha.long"]
other = "Alpha long description"
["fixture.alpha.use"]
other = "alpha <item>"
["fixture.alpha.example"]
other = "Example alpha"
["fixture.alpha.flag"]
other = "Alpha name"
["fixture.beta.short"]
other = "Beta short"
["fixture.beta.long"]
other = "Beta long description"
["fixture.beta.use"]
other = "beta <item>"
["fixture.beta.example"]
other = "Example beta"
["fixture.beta.flag"]
other = "Beta name"
`,
		"zh-CN": `
["fixture.alpha.short"]
other = "甲命令"
["fixture.alpha.long"]
other = "甲命令长说明"
["fixture.alpha.use"]
other = "alpha <项目>"
["fixture.alpha.example"]
other = "甲命令示例"
["fixture.alpha.flag"]
other = "甲名称"
["fixture.beta.short"]
other = "乙命令"
["fixture.beta.long"]
other = "乙命令长说明"
["fixture.beta.use"]
other = "beta <项目>"
["fixture.beta.example"]
other = "乙命令示例"
["fixture.beta.flag"]
other = "乙名称"
`,
	}
	files := fstest.MapFS{}
	for lang, addition := range extra {
		name := "active." + lang + ".toml"
		data, err := fs.ReadFile(locales.Catalog(), name)
		require.NoError(t, err)
		files[name] = &fstest.MapFile{Data: append(data, []byte(addition)...)}
	}
	bundle, err := messages.NewBundle(files)
	require.NoError(t, err)
	return bundle
}

func TestTranslatedUsageCannotChangeCommandName(t *testing.T) {
	for _, tc := range []struct{ name, use string }{
		{"different name", "salute --name NAME"},
		{"leading space", " greet --name NAME"},
		{"tab in name", "greet\t--name NAME"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle := catalogFixture(t, "en", "cli.greet_use", "other = "+strconv.Quote(tc.use)+"\n")
			loader := func() (*messages.Bundle, error) { return bundle, nil }
			t.Run("construct", func(t *testing.T) {
				root, _, err := buildRoot(defaults(Options{LookupEnv: noFixtureEnv}), loader)
				require.ErrorContains(t, err, "cli.greet_use")
				require.Equal(t, "供用户和自动化工具使用的 CLI 模板", root.Short)
				greet, _, findErr := root.Find([]string{"greet"})
				require.NoError(t, findErr)
				require.Equal(t, "greet --name 姓名", greet.Use)
			})
			t.Run("run", func(t *testing.T) {
				var out, stderr bytes.Buffer
				code := run(context.Background(), []string{"greet", "--help"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, loader)
				require.Equal(t, 1, code, stderr.String())
				require.Empty(t, out.String())
				require.Contains(t, stderr.String(), "cli.greet_use")
			})
		})
	}
}

func TestExistingHelpAndVersionFlagsKeepTheirDescriptions(t *testing.T) {
	for _, flag := range []string{"help", "version"} {
		for _, scope := range []string{"local", "persistent", "inherited"} {
			t.Run(flag+"/"+scope, func(t *testing.T) {
				var out bytes.Buffer
				root, call, err := buildRoot(defaults(Options{Out: &out, LookupEnv: noFixtureEnv}), locales.LoadBundle)
				require.NoError(t, err)
				cmd := &cobra.Command{Use: "extra", RunE: func(*cobra.Command, []string) error { return nil }}
				if flag == "version" {
					cmd.Version = "dev"
					cmd.Flags().Bool("help", false, "custom help description")
				}
				description := "custom " + flag + " description"
				var registered *pflag.Flag
				args := []string{"extra", "--help"}
				switch scope {
				case "local":
					cmd.Flags().Bool(flag, false, description)
					registered = cmd.Flags().Lookup(flag)
					root.AddCommand(cmd)
				case "persistent":
					cmd.PersistentFlags().Bool(flag, false, description)
					registered = cmd.PersistentFlags().Lookup(flag)
					root.AddCommand(cmd)
				case "inherited":
					parent := &cobra.Command{Use: "parent"}
					parent.PersistentFlags().Bool(flag, false, description)
					registered = parent.PersistentFlags().Lookup(flag)
					parent.AddCommand(cmd)
					root.AddCommand(parent)
					args = []string{"parent", "extra", "--help"}
				}
				configureHelp(cmd, call)
				require.NoError(t, call.localizeTree(root, "en"))
				root.SetArgs(args)
				require.NoError(t, root.Execute())
				require.Contains(t, out.String(), description)
				require.Equal(t, description, registered.Usage)
			})
		}
	}
}

func TestDefaultHelpAndVersionFlagsAreLocalized(t *testing.T) {
	for _, tc := range []struct{ lang, help, version string }{
		{"en", "Show help", "Show version information"},
		{"zh", "显示帮助", "显示版本信息"},
	} {
		code, out, stderr := invoke(t, "--lang", tc.lang, "--help")
		require.Zero(t, code, stderr)
		require.Contains(t, out, tc.help)
		require.Contains(t, out, tc.version)
	}
}

func TestLongDescriptionRenderingFailureIsReturned(t *testing.T) {
	var out bytes.Buffer
	root, call, err := buildRoot(defaults(Options{Out: &out, LookupEnv: noFixtureEnv}), locales.LoadBundle)
	require.NoError(t, err)
	cmd := &cobra.Command{Use: "extra", Long: "original description"}
	root.AddCommand(cmd)
	call.bindCommand(cmd, commandTexts{long: message("cli.version_text")})
	err = call.localizeTree(root, "en")
	require.ErrorContains(t, err, "cli.version_text")
	require.Equal(t, "original description", cmd.Long)
	require.Empty(t, out.String())
}

func TestCompletionArityUsesPluralCatalog(t *testing.T) {
	code, out, stderr := invoke(t, "completion", "bash", "extra")
	require.Equal(t, 2, code)
	require.Empty(t, out)
	require.Contains(t, stderr, "received 2 arguments")
	require.NotContains(t, stderr, "Operation failed")
}

func noFixtureEnv(string) (string, bool) { return "", false }

func TestGreetExampleUsesInvocationName(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		code, out, stderr := invoke(t, "--lang", lang, "greet", "--help")
		require.Zero(t, code, stderr)
		require.Contains(t, out, "testcli greet --name Codex")
		require.Contains(t, out, "testcli greet --name Codex --json")
	}
}

// 翻译中的合法字面模板分隔符及元数据必须只渲染一次。
func TestVersionFlagRendersLiteralTextOnce(t *testing.T) {
	for _, tc := range []struct{ name, template, nameValue, version, want string }{
		{"opening delimiter", `prefix {{"{{"}}`, "app", "v1", "prefix {{\n"},
		{"literal field", `prefix {{"{{.Missing}}"}}`, "app", "v1", "prefix {{.Missing}}\n"},
		{"actual metadata", `{{.Name}} {{.Version}}`, "app{{.Name}}", "v{{.Missing}}", "app{{.Name}} v{{.Missing}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replacement := "other = " + strconv.Quote(tc.template) + "\n"
			bundle := catalogFixture(t, "en", "cli.version_template", replacement)
			var out, stderr bytes.Buffer
			var code int
			require.NotPanics(t, func() {
				code = run(context.Background(), []string{"--version"}, Options{Name: tc.nameValue, Version: tc.version, Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, func() (*messages.Bundle, error) { return bundle, nil })
			})
			require.Zero(t, code, stderr.String())
			require.Equal(t, tc.want, out.String())
			require.Empty(t, stderr.String())
		})
	}
}

func TestVersionFlagRenderingFailureLeavesStdoutEmpty(t *testing.T) {
	bundle := catalogFixture(t, "en", "cli.version_template", "other = \"prefix {{.Missing}}\"\n")
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{"--version"}, Options{Out: &out, ErrOut: &stderr, LookupEnv: noFixtureEnv}, func() (*messages.Bundle, error) { return bundle, nil })
	require.Equal(t, 1, code, stderr.String())
	require.Empty(t, out.String())
	require.Contains(t, stderr.String(), "cli.version_template")
}

// 独立版本的每个命令使用自身身份，并保留翻译和元数据的字面模板标记。
func TestVersionUsesExecutingCommandMetadata(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--version"}, "prefix {{ parent parent-v1\n"},
		{[]string{"child{{.Name}}", "--version"}, "prefix {{ child{{.Name}} v{{.Missing}}\n"},
		{[]string{"child{{.Name}}", "grandchild", "--version"}, "prefix {{ grandchild grand-v3\n"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			bundle := catalogFixture(t, "en", "cli.version_template", "other = "+strconv.Quote(`prefix {{"{{"}} {{.Name}} {{.Version}}`)+"\n")
			var out bytes.Buffer
			root, call, err := buildRoot(defaults(Options{Name: "parent", Version: "parent-v1", Out: &out, LookupEnv: noFixtureEnv}), func() (*messages.Bundle, error) { return bundle, nil })
			require.NoError(t, err)
			child := &cobra.Command{Use: "child{{.Name}}", Version: "v{{.Missing}}", RunE: func(*cobra.Command, []string) error { return nil }}
			grandchild := &cobra.Command{Use: "grandchild", Version: "grand-v3", RunE: func(*cobra.Command, []string) error { return nil }}
			child.AddCommand(grandchild)
			root.AddCommand(child)
			configureHelp(child, call)
			require.NoError(t, call.localizeTree(root, "en"))
			root.SetArgs(tc.args)
			require.NotPanics(t, func() { require.NoError(t, root.Execute()) })
			require.Equal(t, tc.want, out.String())
		})
	}
}

// 子命令版本格式化失败时，命令树和输出均不应被部分提交。
func TestChildVersionRenderFailureDoesNotMutateTree(t *testing.T) {
	bundle := catalogFixture(t, "en", "cli.version_template", `other = '{{index .Name 1}}'`+"\n")
	var out bytes.Buffer
	root, call, err := buildRoot(defaults(Options{Name: "parent", Version: "v1", Out: &out, LookupEnv: noFixtureEnv}), func() (*messages.Bundle, error) { return bundle, nil })
	require.NoError(t, err)
	before := root.VersionTemplate()
	child := &cobra.Command{Use: "x", Version: "v2"}
	root.AddCommand(child)
	require.ErrorContains(t, call.localizeTree(root, "en"), "cli.version_template")
	require.Equal(t, before, root.VersionTemplate())
	require.Empty(t, out.String())
}
