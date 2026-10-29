// Package scaffold renames this template without walking or modifying user files.
// It intentionally depends only on the standard library.
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

// These are the only files whose application identity is managed by the template.
// Keep this implementation and its fixtures unchanged when initializing a project.
var managedFiles = []string{
	"go.mod", "main.go", "internal/cli/cli.go", "tools/init/main.go",
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

// Initialize changes a pristine template's identity, or does nothing if it matches.
func Initialize(root, module, name string) ([]string, error) {
	if !validName.MatchString(name) {
		return nil, errors.New("name must start with a lowercase letter and contain only lowercase letters, digits, or hyphens")
	}
	if !validModule.MatchString(module) || strings.HasSuffix(module, "/") {
		return nil, errors.New("invalid Go module path")
	}
	for _, part := range strings.Split(module, "/") {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("invalid Go module path segment")
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
		return nil, errors.New("project is already initialized; refusing to replace its identity")
	}
	replacer := strings.NewReplacer(TemplateModule, module, TemplateName, name, "GO_CLI_STARTER", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	var changes []change
	// Preflight every destination before writing any file.
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
			return nil, fmt.Errorf("refusing non-regular or symlinked template file %q", relative)
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
					return nil, fmt.Errorf("format renamed file %q: %w", relative, err)
				}
			}
			changes = append(changes, change{path, updatedData, info.Mode().Perm()})
		}
	}
	changed := make([]string, 0, len(changes))
	for _, item := range changes {
		if err := atomicWrite(item); err != nil {
			return changed, fmt.Errorf("initialize %s: %w", item.path, err)
		}
		relative, err := filepath.Rel(root, item.path)
		if err != nil {
			return changed, err
		}
		changed = append(changed, filepath.ToSlash(relative))
	}
	return changed, nil
}

// Identity reads the module path and executable name without running project code.
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
		return "", "", errors.New("cannot find module path or appName in this template")
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

// Let the installed Go tool validate version suffixes and platform-specific path
// rules. The scratch module is removed before any project file is changed.
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
		return fmt.Errorf("invalid Go module path: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
