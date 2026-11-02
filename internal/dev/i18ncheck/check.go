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
		enParams, err := parameters(en[id])
		if err != nil {
			return fmt.Errorf("消息 %q 的 en 模板：%w", id, err)
		}
		zhParams, err := parameters(other)
		if err != nil {
			return fmt.Errorf("消息 %q 的 zh-CN 模板：%w", id, err)
		}
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

func parameters(entry map[string]string) ([]string, error) {
	set := make(map[string]bool)
	for form, text := range entry {
		if form == "description" {
			continue
		}
		t, err := template.New("").Parse(text)
		if err != nil {
			return nil, err
		}
		a := templateAnalyzer{template: t, params: set, active: make(map[string]bool)}
		a.collect(t.Root, true, map[string]bool{"$": true})
		if a.err != nil {
			return nil, a.err
		}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	slices.Sort(result)
	return result, nil
}

type templateAnalyzer struct {
	template *template.Template
	params   map[string]bool
	active   map[string]bool
	err      error
}

func (a *templateAnalyzer) collect(node parse.Node, rootDot bool, aliases map[string]bool) {
	if a.err != nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				a.collect(child, rootDot, aliases)
			}
		}
	case *parse.ActionNode:
		a.collect(n.Pipe, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
	case *parse.PipeNode:
		if n != nil {
			for _, command := range n.Cmds {
				a.collect(command, rootDot, aliases)
			}
		}
	case *parse.CommandNode:
		if len(n.Args) >= 3 {
			if ident, ok := n.Args[0].(*parse.IdentifierNode); ok && ident.Ident == "index" {
				switch root := n.Args[1].(type) {
				case *parse.DotNode:
					if key, ok := n.Args[2].(*parse.StringNode); ok && rootDot {
						a.params[key.Text] = true
					}
				case *parse.VariableNode:
					if len(root.Ident) == 1 && aliases[root.Ident[0]] {
						if key, ok := n.Args[2].(*parse.StringNode); ok {
							a.params[key.Text] = true
						}
					}
				}
			}
		}
		for _, arg := range n.Args {
			a.collect(arg, rootDot, aliases)
		}
	case *parse.FieldNode:
		if rootDot && len(n.Ident) > 0 {
			a.params[n.Ident[0]] = true
		}
	case *parse.VariableNode:
		if len(n.Ident) > 1 && aliases[n.Ident[0]] {
			a.params[n.Ident[1]] = true
		}
	case *parse.ChainNode:
		a.collect(n.Node, rootDot, aliases)
	case *parse.IfNode:
		aliases = maps.Clone(aliases)
		a.collect(n.Pipe, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		a.collect(n.List, rootDot, maps.Clone(aliases))
		a.collect(n.ElseList, rootDot, maps.Clone(aliases))
	case *parse.RangeNode:
		aliases = maps.Clone(aliases)
		a.collect(n.Pipe, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		a.collect(n.List, false, maps.Clone(aliases))
		a.collect(n.ElseList, rootDot, maps.Clone(aliases))
	case *parse.WithNode:
		aliases = maps.Clone(aliases)
		a.collect(n.Pipe, rootDot, aliases)
		bindAliases(n.Pipe, rootDot, aliases)
		bodyRoot, err := templateContext(n.Pipe, rootDot, aliases)
		if err != nil {
			a.err = err
			return
		}
		a.collect(n.List, bodyRoot, maps.Clone(aliases))
		a.collect(n.ElseList, rootDot, maps.Clone(aliases))
	case *parse.TemplateNode:
		a.collect(n.Pipe, rootDot, aliases)
		bodyRoot, err := templateContext(n.Pipe, rootDot, aliases)
		if err != nil {
			a.err = err
			return
		}
		called := a.template.Lookup(n.Name)
		if called == nil || called.Tree == nil {
			a.err = fmt.Errorf("未定义模板 %q", n.Name)
			return
		}
		if a.active[n.Name] {
			a.err = fmt.Errorf("不支持递归模板 %q", n.Name)
			return
		}
		a.active[n.Name] = true
		a.collect(called.Root, bodyRoot, map[string]bool{"$": bodyRoot})
		delete(a.active, n.Name)
	}
}

// templateContext 识别未经计算的根对象或子值；无法判断的上下文直接拒绝。
func templateContext(pipe *parse.PipeNode, rootDot bool, aliases map[string]bool) (bool, error) {
	if pipe == nil {
		return false, nil
	}
	if len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		return false, fmt.Errorf("无法静态确定模板上下文")
	}
	switch value := pipe.Cmds[0].Args[0].(type) {
	case *parse.DotNode:
		return rootDot, nil
	case *parse.VariableNode:
		isRoot, known := aliases[value.Ident[0]]
		if !known {
			return false, fmt.Errorf("无法静态确定模板变量 %q", value.Ident[0])
		}
		return len(value.Ident) == 1 && isRoot, nil
	case *parse.FieldNode, *parse.ChainNode, *parse.StringNode, *parse.NumberNode, *parse.BoolNode, *parse.NilNode:
		return false, nil
	case *parse.PipeNode:
		return templateContext(value, rootDot, aliases)
	default:
		return false, fmt.Errorf("无法静态确定模板上下文")
	}
}

// bindAliases 跟踪直接引用根数据的模板变量，分支中使用独立作用域。
func bindAliases(pipe *parse.PipeNode, rootDot bool, aliases map[string]bool) {
	if pipe == nil || len(pipe.Decl) != 1 {
		return
	}
	name := pipe.Decl[0].Ident[0]
	if len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		delete(aliases, name)
		return
	}
	isRoot := false
	switch value := pipe.Cmds[0].Args[0].(type) {
	case *parse.DotNode:
		isRoot = rootDot
	case *parse.VariableNode:
		var known bool
		isRoot, known = aliases[value.Ident[0]]
		if !known {
			delete(aliases, name)
			return
		}
		isRoot = len(value.Ident) == 1 && isRoot
	}
	aliases[name] = isRoot
}
