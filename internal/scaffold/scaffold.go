// Package scaffold 重命名模板，不遍历或修改用户文件。
// 此包仅依赖标准库。
package scaffold

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const TemplateModule = "github.com/zhYou/go-cli-starter"
const TemplateName = "go-cli-starter"

// 模板只管理这些文件中的应用标识。
// 初始化项目时保留本实现及其测试夹具，避免改动初始化基准。
var managedFiles = []string{
	"go.mod", "main.go", "internal/cli/cli.go",
	"internal/cli/errors.go",
	"internal/cli/output_test.go",
	"internal/cli/errors_test.go",
	"internal/apperror/error_test.go", "tools/init/main.go",
	"tools/dev/main.go", "tools/dev/template.go", "mise.toml", ".goreleaser.yaml", "README.md", "AGENTS.md",
	"config.example.yaml", "docs/development.md",
}

var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var validModule = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

type change struct {
	path string
	data []byte
	mode os.FileMode
}

// Initialize 修改原始模板的标识；标识已匹配时不做修改。
func Initialize(root, module, name string) ([]string, error) {
	if !validName.MatchString(name) {
		return nil, errors.New("名称必须以小写字母开头，且只能包含小写字母、数字或连字符")
	}
	if !validModule.MatchString(module) || strings.HasSuffix(module, "/") {
		return nil, errors.New("无效的 Go 模块路径")
	}
	for _, part := range strings.Split(module, "/") {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("无效的 Go 模块路径片段")
		}
	}
	if err := validateModulePath(module); err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	currentModule, currentName, err := Identity(root)
	if err != nil {
		return nil, err
	}
	if currentModule == module && currentName == name {
		return []string{}, nil
	}
	if currentModule != TemplateModule || currentName != TemplateName {
		return nil, errors.New("项目已初始化，拒绝替换现有标识")
	}
	replacer := strings.NewReplacer(TemplateModule, module, TemplateName, name, "GO_CLI_STARTER", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	var changes []change
	// 写入文件前先检查所有目标路径。
	for _, relative := range managedFiles {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || resolved != path {
			return nil, fmt.Errorf("拒绝修改非常规文件或包含符号链接的模板路径 %q", relative)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		updated := replacer.Replace(string(data))
		if updated != string(data) {
			updatedData := []byte(updated)
			if filepath.Ext(path) == ".go" {
				updatedData, err = format.Source(updatedData)
				if err != nil {
					return nil, fmt.Errorf("格式化重命名后的文件 %q 失败：%w", relative, err)
				}
			}
			changes = append(changes, change{path, updatedData, info.Mode().Perm()})
		}
	}
	changed := make([]string, 0, len(changes))
	for _, item := range changes {
		if err := atomicWrite(item); err != nil {
			return changed, fmt.Errorf("初始化 %s 失败：%w", item.path, err)
		}
		relative, err := filepath.Rel(root, item.path)
		if err != nil {
			return changed, err
		}
		changed = append(changed, filepath.ToSlash(relative))
	}
	return changed, nil
}

// Identity 读取模块路径和可执行文件名，不执行项目代码。
func Identity(root string) (string, string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", "", err
	}
	var module string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			module = strings.Trim(fields[1], `"`)
			break
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "main.go"), nil, 0)
	if err != nil {
		return "", "", err
	}
	var name string
	ast.Inspect(file, func(node ast.Node) bool {
		value, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, ident := range value.Names {
			if ident.Name != "appName" || i >= len(value.Values) {
				continue
			}
			if literal, ok := value.Values[i].(*ast.BasicLit); ok && literal.Kind == token.STRING {
				name, _ = strconv.Unquote(literal.Value)
			}
		}
		return true
	})
	if module == "" || name == "" {
		return "", "", errors.New("无法从模板中找到模块路径或 appName")
	}
	return module, name, nil
}

func atomicWrite(item change) error {
	file, err := os.CreateTemp(filepath.Dir(item.path), ".template-init-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if err := file.Chmod(item.mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(item.data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), item.path)
}

// 通过已安装的 Go 工具检查版本后缀和各平台的路径规则。
// 修改项目文件前即删除用于检查的临时模块。
func validateModulePath(module string) error {
	dir, err := os.MkdirTemp("", "cli-module-check-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	cmd := exec.Command("go", "mod", "init", module)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("无效的 Go 模块路径：%w：%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
