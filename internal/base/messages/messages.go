// Package messages 组合只读目录，创建每次调用独立的语言视图。
package messages

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/nicksnyder/go-i18n/v2/i18n/template"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"golang.org/x/text/language"
)

// Bundle 构造后不再暴露修改接口，可并发创建 Localizer。
type Bundle struct{ bundle *i18n.Bundle }

// Localizer 提供本次调用的只读语言视图。
type Localizer struct {
	language  string
	localizer *i18n.Localizer
}

// NewBundle 加载并冻结各模块的消息目录。
func NewBundle(catalogs ...fs.FS) (*Bundle, error) {
	b := i18n.NewBundle(language.SimplifiedChinese)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, catalog := range catalogs {
		names, err := fs.Glob(catalog, "locales/*.json")
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if _, err := b.LoadMessageFileFS(catalog, name); err != nil {
				return nil, err
			}
		}
	}
	return &Bundle{bundle: b}, nil
}

// Localizer 创建指定语言的独立只读视图。
func (b *Bundle) Localizer(lang string) *Localizer {
	if lang == "zh" {
		lang = "zh-CN"
	}
	return &Localizer{language: lang, localizer: i18n.NewLocalizer(b.bundle, lang)}
}

// Language 返回当前语言标识。
func (l *Localizer) Language() string { return l.language }

// Text 渲染目录消息；缺失消息返回可诊断的默认文本。
func (l *Localizer) Text(id string, params map[string]any) string {
	if l != nil && l.localizer != nil {
		text, err := l.localizer.Localize(&i18n.LocalizeConfig{MessageID: id, TemplateData: params, TemplateParser: &template.TextParser{Option: "missingkey=error"}})
		if err == nil {
			return text
		}
	}
	if l != nil && l.language == "en" {
		return "Operation failed [" + id + "]"
	}
	return "操作失败 [" + id + "]"
}

// Render 按错误结构组合领域消息与外部原因。
func (l *Localizer) Render(err error) string {
	var lines []string
	for _, detail := range fault.Details(err) {
		var text string
		if detail.Code != "" {
			text = l.Text(string(detail.Code), detail.Params)
		} else {
			text = detail.Message
		}
		if detail.Cause != nil && !strings.HasPrefix(string(detail.Code), "validation.") {
			cause := ""
			// 未分类叶子错误保留系统或第三方诊断。
			var typed *fault.Error
			if errors.As(detail.Cause, &typed) {
				cause = l.Render(detail.Cause)
			} else {
				cause = fmt.Sprint(detail.Cause)
			}
			if text != "" {
				text += ": "
			}
			text += cause
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "; ")
}
