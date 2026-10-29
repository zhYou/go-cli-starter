package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCopiedRepositorySupportsGoVCSStamping(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/fixture\n\ngo 1.27.0\n",
		"main.go": "package main\nfunc main(){}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := prepareFixture(dir); err != nil {
		t.Fatal(err)
	}
	remote := exec.Command("git", "remote", "get-url", "origin")
	remote.Dir = dir
	if output, err := remote.Output(); err != nil || len(output) == 0 {
		t.Fatalf("测试仓库缺少发布校验所需的 SCM 标识：%v", err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "fixture"), ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("独立构建失败：%v\n%s", err, output)
	}
}
