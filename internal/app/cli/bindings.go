package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

type textMessage struct {
	id     string
	params map[string]any
}

type commandTexts struct {
	short, long, use, example textMessage
}

type commandTextBinding struct {
	command *cobra.Command
	name    string
	texts   commandTexts
}

type flagTextBinding struct {
	flag *pflag.Flag
	text textMessage
}

func message(id string) textMessage { return textMessage{id: id} }

func (call *invocation) bindCommand(cmd *cobra.Command, texts commandTexts) {
	call.commandTexts = append(call.commandTexts, commandTextBinding{command: cmd, name: cmd.Name(), texts: texts})
}

func (call *invocation) bindFlag(flag *pflag.Flag, text textMessage) {
	call.flagTexts = append(call.flagTexts, flagTextBinding{flag: flag, text: text})
}

// localizeTree 先完整渲染，再将文案应用到本次调用的具体命令和选项。
func (call *invocation) localizeTree(root *cobra.Command, language string) error {
	localizer := call.bundle.Localizer(language)
	format := func(text textMessage) (string, error) {
		if text.id == "" {
			return "", nil
		}
		return localizer.Format(text.id, messages.FormatOptions{Params: text.params})
	}
	type renderedCommand struct{ short, long, use, example string }
	commands := make([]renderedCommand, len(call.commandTexts))
	for i, binding := range call.commandTexts {
		fields := []struct {
			text textMessage
			dest *string
		}{
			{binding.texts.short, &commands[i].short},
			{binding.texts.long, &commands[i].long},
			{binding.texts.use, &commands[i].use},
			{binding.texts.example, &commands[i].example},
		}
		for _, field := range fields {
			text, err := format(field.text)
			if err != nil {
				return err
			}
			*field.dest = text
		}
		if binding.texts.use.id != "" {
			parts := strings.Fields(commands[i].use)
			if len(parts) == 0 || parts[0] != binding.name {
				return fmt.Errorf("message %q changes command name %q", binding.texts.use.id, binding.name)
			}
		}
	}
	flags := make([]string, len(call.flagTexts))
	for i, binding := range call.flagTexts {
		text, err := format(binding.text)
		if err != nil {
			return err
		}
		flags[i] = text
	}
	template, err := localizer.Format("cli.version_template", messages.FormatOptions{Params: map[string]any{"Name": "{{.Name}}", "Version": "{{.Version}}"}})
	if err != nil {
		return err
	}
	for i, binding := range call.commandTexts {
		if binding.texts.short.id != "" {
			binding.command.Short = commands[i].short
		}
		if binding.texts.long.id != "" {
			binding.command.Long = commands[i].long
		}
		if binding.texts.use.id != "" {
			binding.command.Use = commands[i].use
		}
		if binding.texts.example.id != "" {
			binding.command.Example = commands[i].example
		}
	}
	for i, binding := range call.flagTexts {
		binding.flag.Usage = flags[i]
	}
	root.SetVersionTemplate(template + "\n")
	call.localizer = localizer
	return nil
}
