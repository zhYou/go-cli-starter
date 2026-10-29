package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zhYou/go-cli-starter/internal/scaffold"
)

func verifyTemplate() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "cli-template-check-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// Git's file selection excludes local credentials, caches and build artifacts.
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	files, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list template files (run inside a Git repository): %w", err)
	}
	for _, name := range strings.Split(string(files), "\x00") {
		if name == "" {
			continue
		}
		if err := copyFile(filepath.Join(root, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	module, name, err := scaffold.Identity(dir)
	if err != nil {
		return err
	}
	if module == scaffold.TemplateModule && name == scaffold.TemplateName {
		module, name = "example.com/template-check/mycli", "mycli"
	}
	if _, err := scaffold.Initialize(dir, module, name); err != nil {
		return err
	}
	if changed, err := scaffold.Initialize(dir, module, name); err != nil || len(changed) != 0 {
		return fmt.Errorf("initializer is not idempotent: %v", err)
	}
	if err := prepareFixture(dir); err != nil {
		return err
	}
	if err := command(dir, "go", "run", "./tools/dev", "fmt-check"); err != nil {
		return err
	}
	if err := command(dir, "go", "test", "./..."); err != nil {
		return err
	}
	if err := command(dir, "go", "build", "-o", filepath.Join(dir, "app-check"), "."); err != nil {
		return err
	}
	check := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".")
	check.Dir = dir
	imports, err := check.Output()
	if err != nil {
		return err
	}
	if module != scaffold.TemplateModule && strings.Contains(string(imports), scaffold.TemplateModule) {
		return fmt.Errorf("generated project still imports the original template")
	}
	if err := command(dir, "go", "run", ".", "schema", "greet"); err != nil {
		return err
	}
	if err := command(dir, "goreleaser", "check"); err != nil {
		return err
	}
	fmt.Println("Template copy: initialization, idempotence, tests, build, schema and release configuration passed.")
	return nil
}

func prepareFixture(dir string) error {
	if err := command(dir, "git", "-c", "init.templateDir=", "init", "--quiet"); err != nil {
		return err
	}
	// This is a disposable repository, never the caller's remote. GoReleaser's
	// SCM validation needs a GitHub-shaped origin, but does not contact it.
	return command(dir, "git", "remote", "add", "origin", "https://github.com/example/template-check.git")
}

func copyFile(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cannot copy non-regular template file %s", source)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
