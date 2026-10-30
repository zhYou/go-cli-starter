package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/dev/scaffold"
	"go.yaml.in/yaml/v3"
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
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0600))
	}
	for _, name := range []string{"SECURITY.md", ".github/PULL_REQUEST_TEMPLATE.md", ".github/CODEOWNERS.example", ".github/ISSUE_TEMPLATE/backlog.yml", "docs/maintenance.md"} {
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

func TestBacklogIssueTemplate(t *testing.T) {
	data, err := os.ReadFile("../../.github/ISSUE_TEMPLATE/backlog.yml")
	require.NoError(t, err)
	var form struct {
		Name   string   `yaml:"name"`
		Labels []string `yaml:"labels"`
		Body   []struct {
			Attributes struct {
				Label string `yaml:"label"`
			} `yaml:"attributes"`
			Validations struct {
				Required bool `yaml:"required"`
			} `yaml:"validations"`
		} `yaml:"body"`
	}
	require.NoError(t, yaml.Unmarshal(data, &form))
	require.Equal(t, "项目待办", form.Name)
	require.Contains(t, form.Labels, "backlog")
	fields := map[string]bool{
		"优先级":         true,
		"工作量":         true,
		"背景":          true,
		"范围":          true,
		"验收标准":        true,
		"关联 Issue/PR": false,
	}
	for _, field := range form.Body {
		label := field.Attributes.Label
		required, ok := fields[label]
		if !ok {
			continue
		}
		require.Equal(t, required, field.Validations.Required, label)
		delete(fields, label)
	}
	require.Empty(t, fields)
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

func TestTemplateExecutablePath(t *testing.T) {
	for _, tc := range []struct {
		platform string
		name     string
	}{
		{platform: "linux", name: "app-check"},
		{platform: "darwin", name: "app-check"},
		{platform: "windows", name: "app-check.exe"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			dir := t.TempDir()
			require.Equal(t, filepath.Join(dir, tc.name), templateExecutablePath(dir, tc.platform))
		})
	}
}

// TestTemplateExecutableAcceptance 直接调用模板任务的构建与执行边界，避免测试自行补齐扩展名。
func TestTemplateExecutableAcceptance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_CLI_STARTER_HOME", filepath.Join(dir, "starter-home"))
	t.Setenv("MYCLI_HOME", filepath.Join(dir, "mycli-home"))
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	files, err := cmd.Output()
	require.NoError(t, err)
	for name := range strings.SplitSeq(string(files), "\x00") {
		if name != "" {
			require.NoError(t, copyFile(filepath.Join(root, name), filepath.Join(dir, name)))
		}
	}
	_, err = scaffold.Initialize(dir, "example.com/template-check/mycli", "mycli")
	require.NoError(t, err)
	require.NoError(t, prepareFixture(dir))
	project, err := scaffold.Inspect(dir)
	require.NoError(t, err)
	require.NoError(t, verifyTemplateExecutable(dir, project))
}
