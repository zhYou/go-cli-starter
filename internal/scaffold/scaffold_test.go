package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":    "module github.com/zhYou/go-cli-starter\n\ngo 1.27.0\n",
		"main.go":   "package main\nconst appName = \"go-cli-starter\"\nfunc main() {}\n",
		"README.md": "go-cli-starter github.com/zhYou/go-cli-starter GO_CLI_STARTER_LOG_LEVEL\n",
		"user.txt":  "go-cli-starter\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitializeRewritesOnlyTemplateFiles(t *testing.T) {
	root := fixture(t)
	before, err := os.Stat(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Initialize(root, "github.com/acme/tool", "my-cli")
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 3 {
		t.Fatalf("changed=%v", changed)
	}
	if !strings.HasPrefix(read(t, root, "go.mod"), "module github.com/acme/tool\n") {
		t.Fatal("模块未重命名")
	}
	if !strings.Contains(read(t, root, "main.go"), `appName = "my-cli"`) {
		t.Fatal("程序标识未重命名")
	}
	if got := read(t, root, "README.md"); got != "my-cli github.com/acme/tool MY_CLI_LOG_LEVEL\n" {
		t.Fatalf("README=%q", got)
	}
	if read(t, root, "user.txt") != "go-cli-starter\n" {
		t.Fatal("修改了无关文件")
	}
	info, err := os.Stat(filepath.Join(root, "main.go"))
	if err != nil || info.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("文件权限发生变化：%v %v", info, err)
	}
}

func TestInitializeIsIdempotentAndRejectsConflictingIdentity(t *testing.T) {
	root := fixture(t)
	if _, err := Initialize(root, "example.com/team/tool", "tool"); err != nil {
		t.Fatal(err)
	}
	changed, err := Initialize(root, "example.com/team/tool", "tool")
	if err != nil || len(changed) != 0 {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	before := read(t, root, "main.go")
	if _, err := Initialize(root, "example.com/team/other", "other"); err == nil {
		t.Fatal("错误地接受了冲突的重复初始化")
	}
	if read(t, root, "main.go") != before {
		t.Fatal("冲突的初始化修改了项目")
	}
}

func TestInitializeRejectsInvalidInputsBeforeWriting(t *testing.T) {
	for _, tc := range []struct{ module, name string }{
		{"", "tool"}, {"example.com/tool", ""}, {"example.com/tool", "../tool"},
		{"example.com/tool", "tool;touch bad"}, {"../tool", "tool"},
		{"example.com/team//tool", "tool"}, {"example.com/tool\nother", "tool"},
		{"example.com/tool/v1", "tool"}, {"example.com/tool.", "tool"},
	} {
		root := fixture(t)
		if _, err := Initialize(root, tc.module, tc.name); err == nil {
			t.Errorf("错误地接受了无效输入 %+v", tc)
		}
		if !strings.Contains(read(t, root, "go.mod"), "github.com/zhYou/go-cli-starter") {
			t.Fatal("无效输入修改了模块")
		}
	}
}

func TestInitializeDoesNotRewriteNewModuleSubstring(t *testing.T) {
	root := fixture(t)
	if _, err := Initialize(root, "example.com/go-cli-starter-tools", "tool"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, root, "go.mod"), "module example.com/go-cli-starter-tools\n") {
		t.Fatal("模块替换发生了级联改写")
	}
}

func TestInitializeKeepsRenamedImportsFormatted(t *testing.T) {
	root := fixture(t)
	source := "package main\n\nimport (\n\t\"github.com/spf13/cobra\"\n\t\"github.com/zhYou/go-cli-starter/internal/config\"\n)\n\nconst appName = \"go-cli-starter\"\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(root, "example.com/team/tool", "tool"); err != nil {
		t.Fatal(err)
	}
	wantImports := "import (\n\t\"example.com/team/tool/internal/config\"\n\t\"github.com/spf13/cobra\"\n)"
	if got := read(t, root, "main.go"); !strings.Contains(got, wantImports) {
		t.Fatalf("重命名后的导入未格式化：\n%s", got)
	}
}

func TestInitializeRejectsSymlinkBeforeAnyWrites(t *testing.T) {
	root := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("go-cli-starter"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "README.md")); err != nil {
		t.Skipf("符号链接不可用：%v", err)
	}
	if _, err := Initialize(root, "example.com/tool", "tool"); err == nil {
		t.Fatal("错误地接受了符号链接")
	}
	if !strings.Contains(read(t, root, "go.mod"), "github.com/zhYou/go-cli-starter") {
		t.Fatal("预检失败时仍修改了部分项目文件")
	}
}
