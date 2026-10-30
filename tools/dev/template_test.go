package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/dev/scaffold"
)

func TestCopiedRepositorySupportsGoVCSStamping(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":              "module example.com/fixture\n\ngo 1.27.0\n",
		"cmd/fixture/main.go": "package main\nconst appName=\"fixture\"\nfunc main(){}\n",
	} {
		//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
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
	//nolint:gosec // G204：测试只运行当前测试二进制或固定 Git/Go 命令，参数来自临时 fixture。
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "fixture"), "./cmd/fixture")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("独立构建失败：%v\n%s", err, output)
	}
}

func TestTemplateMetadataAndGuidesSurviveRename(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module " + scaffold.TemplateModule + "\n\ngo 1.27.0\n", "cmd/" + scaffold.TemplateName + "/main.go": "package main\nconst appName = \"" + scaffold.TemplateName + "\"\nfunc main(){}\n", ".template-version": "0.1.0\n"} {
		//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
		//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0600))
	}
	for _, name := range []string{"SECURITY.md", ".github/PULL_REQUEST_TEMPLATE.md", ".github/CODEOWNERS.example", "docs/maintenance.md"} {
		//nolint:gosec // G301：测试路径和权限只来自当前临时 fixture，不读取用户文件。
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755))
		require.NoError(t, copyFile(filepath.Join("../..", name), filepath.Join(dir, name)))
	}
	//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
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

func TestInstallVersionLikeCommandNames(t *testing.T) {
	for _, name := range []string{"v0", "v1", "v02"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/installfixture\n\ngo 1.27.0\n"), 0600))
			dir := filepath.Join(root, "cmd", name)
			//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
			require.NoError(t, os.MkdirAll(dir, 0755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nconst appName=\""+name+"\"\nfunc main(){}\n"), 0600))
			require.NoError(t, prepareFixture(root))
			project, err := scaffold.Inspect(root)
			require.NoError(t, err)
			require.NoError(t, verifyCommandInstall(root, project))
		})
	}
}
