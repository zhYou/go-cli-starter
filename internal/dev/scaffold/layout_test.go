package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializeMovesCommandDirectory(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	source := `package main
import _ "` + TemplateModule + `/internal/app/cli"
const appName = "` + TemplateName + `"
func main() {}
`
	//nolint:gosec // G306：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", TemplateName, "main.go"), []byte(source), 0640))
	changed, err := Initialize(root, "example.com/team/repository", "my-cli")
	require.NoError(t, err)
	require.NotEmpty(t, changed)
	project, err := Inspect(root)
	require.NoError(t, err)
	require.Equal(t, Project{"example.com/team/repository", "my-cli", "cmd/my-cli"}, project)
	require.NoDirExists(t, filepath.Join(root, "cmd", TemplateName))
	//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
	data, err := os.ReadFile(filepath.Join(root, "cmd/my-cli/main.go"))
	require.NoError(t, err)
	require.Contains(t, string(data), "example.com/team/repository/internal/app/cli")
	changed, err = Initialize(root, "example.com/team/repository", "my-cli")
	require.NoError(t, err)
	require.Empty(t, changed)
}

func TestInitializeSameNameDoesNotMove(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	_, err := Initialize(root, "example.com/team/repository", TemplateName)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "cmd", TemplateName, "main.go"))
	changed, err := Initialize(root, "example.com/team/repository", TemplateName)
	require.NoError(t, err)
	require.Empty(t, changed)
}

func TestOriginUsesCommandEntry(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".template-version"), []byte("0.1.0\n"), 0600))
	git := func(args ...string) string {
		t.Helper()
		//nolint:gosec // G204：测试只运行当前测试二进制或固定 Git/Go 命令，参数来自临时 fixture。
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		return string(out)
	}
	git("-c", "init.templateDir=", "init", "--quiet")
	git("remote", "add", "origin", "https://"+TemplateModule+".git")
	git("add", ".")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "commit", "--quiet", "-m", "fixture")
	origin, err := makeOrigin(root, "example.com/tool", "tool")
	require.NoError(t, err)
	require.NotEqual(t, "unknown", origin.SourceCommit)
	git("remote", "set-url", "origin", "https://example.com/copied.git")
	origin, err = makeOrigin(root, "example.com/tool", "tool")
	require.NoError(t, err)
	require.Equal(t, "unknown", origin.SourceCommit)
}

func TestInitializeRejectsGoVersionNamesBeforeWriting(t *testing.T) {
	for _, name := range []string{"v2", "v10", "v999999999999999999999999999999999999999"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, TemplateModule, TemplateName)
			//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			before, err := os.ReadFile(filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			_, err = Initialize(root, "example.com/tool", name)
			require.Error(t, err)
			//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			after, err := os.ReadFile(filepath.Join(root, "go.mod"))
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.NoDirExists(t, filepath.Join(root, "cmd", name))
		})
	}
}

func TestInspectRejectsGoVersionNames(t *testing.T) {
	for _, name := range []string{"v2", "v10"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, "example.com/tool", name)
			_, err := Inspect(root)
			require.Error(t, err)
		})
	}
	for _, name := range []string{"v0", "v1", "v02"} {
		t.Run(name, func(t *testing.T) {
			root := projectFixture(t, "example.com/tool", name)
			project, err := Inspect(root)
			require.NoError(t, err)
			require.Equal(t, name, project.Name)
		})
	}
}

func TestInitializeAcceptanceIdentityWhitelist(t *testing.T) {
	root := projectFixture(t, TemplateModule, TemplateName)
	for _, name := range []string{"tools/dev/acceptance.go", "tools/dev/release_install.go"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700))
		content := "package main\nimport _ \"" + TemplateModule + "/internal/base/paths\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0600))
	}
	unknown := filepath.Join(root, "user.toml")
	require.NoError(t, os.WriteFile(unknown, []byte("name = \""+TemplateName+"\""), 0600))
	_, err := Initialize(root, "example.com/team/mycli", "mycli")
	require.NoError(t, err)
	for _, name := range []string{"tools/dev/acceptance.go", "tools/dev/release_install.go"} {
		//nolint:gosec // G304：仅读取本测试的临时 fixture。
		data, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		require.Contains(t, string(data), "example.com/team/mycli/internal/base/paths")
	}
	//nolint:gosec // G304：仅读取本测试临时目录中的未知文件。
	data, err := os.ReadFile(unknown)
	require.NoError(t, err)
	require.Contains(t, string(data), TemplateName)
}
