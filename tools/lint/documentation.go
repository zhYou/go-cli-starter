package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const documentationFile = "doc.go"

type sourceFile struct {
	path string
	file *ast.File
}

// checkDocumentation 检查 root 中正式 Go 包的说明是否集中在纯文档 doc.go 中。
// 不按当前平台筛选源码；测试夹具、生成代码和外部测试包不受约束。
func checkDocumentation(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("解析仓库路径: %w", err)
	}
	packages := map[string][]sourceFile{}
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("读取 %s: %w", path, walkErr)
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == testdataDirectory || name == "vendor" || (filepath.Dir(path) == root && (name == "bin" || name == "dist"))) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("检查 %s: %w", relative, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s 不是普通文件", relative)
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("解析 %s: %w", relative, err)
		}
		if ast.IsGenerated(file) || (strings.HasSuffix(name, "_test.go") && strings.HasSuffix(file.Name.Name, "_test")) {
			return nil
		}
		directory := filepath.Dir(relative)
		packages[directory] = append(packages[directory], sourceFile{relative, file})
		return nil
	})
	if err != nil {
		return err
	}
	var directories []string
	for directory := range packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	var problems []error
	for _, directory := range directories {
		problems = append(problems, checkPackage(directory, packages[directory])...)
	}
	return errors.Join(problems...)
}

func checkPackage(directory string, files []sourceFile) []error {
	var documentation *ast.File
	packageName := ""
	var problems []error
	for _, source := range files {
		if filepath.Base(source.path) == documentationFile {
			documentation = source.file
			continue
		}
		if packageName == "" {
			packageName = source.file.Name.Name
		} else if source.file.Name.Name != packageName {
			problems = append(problems, fmt.Errorf("%s: 包名不一致，期望 %s", source.path, packageName))
		}
		if !strings.HasSuffix(source.path, "_test.go") && source.file.Doc != nil {
			problems = append(problems, fmt.Errorf("%s: 包说明必须集中在 doc.go", source.path))
		}
	}
	path := filepath.ToSlash(filepath.Join(directory, documentationFile))
	if documentation == nil {
		return append(problems, fmt.Errorf("%s: 缺少 doc.go", path))
	}
	if packageName != "" && documentation.Name.Name != packageName {
		problems = append(problems, fmt.Errorf("%s: 包名不一致，期望 %s", path, packageName))
	}
	description := ""
	if documentation.Doc != nil {
		description = strings.TrimSpace(documentation.Doc.Text())
	}
	description = strings.TrimPrefix(description, "Package "+documentation.Name.Name)
	if !strings.ContainsFunc(description, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r)
	}) {
		problems = append(problems, fmt.Errorf("%s: 包说明不能为空", path))
	}
	if len(documentation.Decls) != 0 {
		problems = append(problems, fmt.Errorf("%s: 只能包含包说明和 package 声明", path))
	}
	return problems
}
