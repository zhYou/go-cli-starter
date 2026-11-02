// Package catalog 校验自维护的 TOML 消息目录。
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
)

// Parse 解析完整消息 ID 表并校验字段、非空内容及模板语法。
func Parse(data []byte) (map[string]map[string]string, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 TOML 消息目录：%w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("消息目录为空")
	}
	result := make(map[string]map[string]string, len(raw))
	for id, value := range raw {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("消息 ID 为空")
		}
		fields, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("消息 %q 必须是完整 ID 表", id)
		}
		entry := make(map[string]string, len(fields))
		for field, value := range fields {
			switch field {
			case "description", "zero", "one", "two", "few", "many", "other":
			default:
				return nil, fmt.Errorf("消息 %q 含未知字段 %q", id, field)
			}
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("消息 %q 的 %s 必须是字符串", id, field)
			}
			if field != "description" {
				if strings.TrimSpace(text) == "" {
					return nil, fmt.Errorf("消息 %q 的 %s 为空", id, field)
				}
				if _, err := template.New(id).Option("missingkey=error").Parse(text); err != nil {
					return nil, fmt.Errorf("消息 %q 的 %s 模板无效：%w", id, field, err)
				}
			}
			entry[field] = text
		}
		if _, ok := entry["other"]; !ok {
			return nil, fmt.Errorf("消息 %q 缺少 other", id)
		}
		result[id] = entry
	}
	return result, nil
}

// Files 查找目录根或兼容旧模块的 locales 子目录中的消息文件。
func Files(source fs.FS) ([]string, error) {
	if source == nil {
		return nil, fmt.Errorf("消息目录为 nil")
	}
	var names []string
	for _, dir := range []string{".", "locales"} {
		entries, err := fs.ReadDir(source, dir)
		if err != nil {
			if dir == "locales" && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("读取消息目录 %s：%w", dir, err)
		}
		for _, entry := range entries {
			ext := path.Ext(entry.Name())
			if !entry.IsDir() && (ext == ".json" || ext == ".toml") {
				names = append(names, path.Join(dir, entry.Name()))
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("消息目录没有支持的文件")
	}
	return names, nil
}
