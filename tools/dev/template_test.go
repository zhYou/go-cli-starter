package main

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/scaffold"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCopiedRepositorySupportsGoVCSStamping(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":              "module example.com/fixture\n\ngo 1.27.0\n",
		"cmd/fixture/main.go": "package main\nconst appName=\"fixture\"\nfunc main(){}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755); err != nil {
			t.Fatal(err)
		}
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
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "fixture"), "./cmd/fixture")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("独立构建失败：%v\n%s", err, output)
	}
}

func TestTemplateMetadataAndGuidesSurviveRename(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module " + scaffold.TemplateModule + "\n\ngo 1.27.0\n", "cmd/" + scaffold.TemplateName + "/main.go": "package main\nconst appName = \"" + scaffold.TemplateName + "\"\nfunc main(){}\n", ".template-version": "0.1.0\n"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0600))
	}
	for _, name := range []string{"SECURITY.md", ".github/PULL_REQUEST_TEMPLATE.md", ".github/CODEOWNERS.example", "docs/maintenance.md"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755))
		require.NoError(t, copyFile(filepath.Join("../..", name), filepath.Join(dir, name)))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0755))
	require.NoError(t, copyFile("../../docs/template-upgrades.md", filepath.Join(dir, "docs/template-upgrades.md")))
	_, err := scaffold.Initialize(dir, "example.com/copied/tool", "copied")
	require.NoError(t, err)
	origin, err := scaffold.ReadOrigin(afero.NewOsFs(), dir)
	require.NoError(t, err)
	require.Equal(t, scaffold.TemplateModule, origin.Template)
	require.Equal(t, "example.com/copied/tool", origin.Module)
	require.FileExists(t, filepath.Join(dir, "docs/template-upgrades.md"))
	require.NoError(t, validateTemplateMetadata(dir))
}
