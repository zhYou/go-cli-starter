package scaffold

import "strings"

// templateSourceReference 只登记精确文件中的完整历史来源引用，不豁免其他 URL。
// 本登记属于初始化器的原模板身份，与其实现和 fixture 一样不加入 managedFiles。
func templateSourceReference(file string) string {
	if file == "docs/backlog.md" {
		return "[PR #5](https://github.com/zhYou/go-cli-starter/pull/5)"
	}
	return ""
}

func replaceManagedIdentity(file, text string, replacer *strings.Replacer) string {
	reference := templateSourceReference(file)
	if reference == "" {
		return replacer.Replace(text)
	}
	// 只分开已登记的完整引用，片段仍使用原身份规则；不使用占位符或 URL 编码。
	parts := strings.Split(text, reference)
	for index := range parts {
		parts[index] = replacer.Replace(parts[index])
	}
	return strings.Join(parts, reference)
}
