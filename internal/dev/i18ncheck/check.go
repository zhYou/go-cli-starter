// Package i18ncheck 检查集中消息目录的一致性。
package i18ncheck

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"text/template"
	"text/template/parse"

	"github.com/zhYou/go-cli-starter/internal/base/messages/catalog"
)

// Check 校验集中目录的语法、双语 ID 和各复数形式参数并集。
func Check(source fs.FS) error {
	names, err := catalog.Files(source)
	if err != nil {
		return err
	}
	files := make(map[string]string)
	for _, name := range names {
		if path.Ext(name) != ".toml" {
			continue
		}
		base := path.Base(name)
		if base != "active.en.toml" && base != "active.zh-CN.toml" {
			return fmt.Errorf("不支持的消息文件 %q", name)
		}
		if files[base] != "" {
			return fmt.Errorf("消息文件 %q 重复", base)
		}
		files[base] = name
	}
	read := func(lang string) (map[string]map[string]string, error) {
		base := "active." + lang + ".toml"
		name := files[base]
		if name == "" {
			return nil, fmt.Errorf("缺少消息文件 %s", base)
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("读取 %s：%w", name, err)
		}
		result, err := catalog.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", name, err)
		}
		return result, nil
	}
	en, err := read("en")
	if err != nil {
		return err
	}
	zh, err := read("zh-CN")
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(en))
	for id := range en {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		other, ok := zh[id]
		if !ok {
			return fmt.Errorf("zh-CN 缺少消息 %q", id)
		}
		enParams := parameters(en[id])
		zhParams := parameters(other)
		if !slices.Equal(enParams, zhParams) {
			return fmt.Errorf("消息 %q 的参数不一致：en=%v，zh-CN=%v", id, enParams, zhParams)
		}
	}
	for id := range zh {
		if _, ok := en[id]; !ok {
			return fmt.Errorf("en 缺少消息 %q", id)
		}
	}
	return nil
}

func parameters(entry map[string]string) []string {
	set := make(map[string]bool)
	for form, text := range entry {
		if form == "description" {
			continue
		}
		// Parse 已验证模板语法，使用相同的内置函数集再次解析以提取参数。
		t, _ := template.New("").Parse(text)
		collect(t.Root, set, true, map[string]bool{"$": true})
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}

func collect(node parse.Node, set map[string]bool, rootDot bool, aliases map[string]bool) {
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				collect(child, set, rootDot, aliases)
			}
		}
	case *parse.ActionNode:
		collect(n.Pipe, set, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
	case *parse.PipeNode:
		if n != nil {
			for _, command := range n.Cmds {
				collect(command, set, rootDot, aliases)
			}
		}
	case *parse.CommandNode:
		if len(n.Args) >= 3 {
			if ident, ok := n.Args[0].(*parse.IdentifierNode); ok && ident.Ident == "index" {
				switch root := n.Args[1].(type) {
				case *parse.DotNode:
					if key, ok := n.Args[2].(*parse.StringNode); ok && rootDot {
						set[key.Text] = true
					}
				case *parse.VariableNode:
					if len(root.Ident) == 1 && aliases[root.Ident[0]] {
						if key, ok := n.Args[2].(*parse.StringNode); ok {
							set[key.Text] = true
						}
					}
				}
			}
		}
		for _, arg := range n.Args {
			collect(arg, set, rootDot, aliases)
		}
	case *parse.FieldNode:
		if rootDot && len(n.Ident) > 0 {
			set[n.Ident[0]] = true
		}
	case *parse.VariableNode:
		if len(n.Ident) > 1 && aliases[n.Ident[0]] {
			set[n.Ident[1]] = true
		}
	case *parse.ChainNode:
		collect(n.Node, set, rootDot, aliases)
	case *parse.IfNode:
		aliases = maps.Clone(aliases)
		collect(n.Pipe, set, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		collect(n.List, set, rootDot, aliases)
		collect(n.ElseList, set, rootDot, aliases)
	case *parse.RangeNode:
		aliases = maps.Clone(aliases)
		collect(n.Pipe, set, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		collect(n.List, set, false, aliases)
		collect(n.ElseList, set, rootDot, aliases)
	case *parse.WithNode:
		aliases = maps.Clone(aliases)
		collect(n.Pipe, set, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		collect(n.List, set, false, aliases)
		collect(n.ElseList, set, rootDot, aliases)
	case *parse.TemplateNode:
		collect(n.Pipe, set, rootDot, aliases)
	}
}

// bindAliases 跟踪直接引用根数据的模板变量，分支中使用独立作用域。
func bindAliases(pipe *parse.PipeNode, rootDot bool, aliases map[string]bool) {
	if pipe == nil || len(pipe.Decl) != 1 || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		return
	}
	isRoot := false
	switch value := pipe.Cmds[0].Args[0].(type) {
	case *parse.DotNode:
		isRoot = rootDot
	case *parse.VariableNode:
		isRoot = len(value.Ident) == 1 && aliases[value.Ident[0]]
	}
	aliases[pipe.Decl[0].Ident[0]] = isRoot
}
