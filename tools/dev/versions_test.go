package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func versionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"mise.toml", "mise.agent.toml", "go.mod", "README.md", "AGENTS.md", ".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		dest := filepath.Join(root, name)
		//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0755))
		require.NoError(t, copyFile(filepath.Join("../..", name), dest))
	}
	return root
}
func TestVersionsWithoutLock(t *testing.T) {
	require.Error(t, matchToolVersion("go", "go version go1.27.30 linux/amd64", "1.27.3"))
	require.NoError(t, matchToolVersion("go", "go version go1.27.3 linux/amd64", "1.27.3"))
	root := versionFixture(t)
	require.NoError(t, checkVersions(root))
}

func TestVersionsRejectNonExactAgentVersion(t *testing.T) {
	for _, version := range []string{"latest", "0.24", "0.24.0-rc.1"} {
		t.Run(version, func(t *testing.T) {
			root := versionFixture(t)
			require.NoError(t, os.WriteFile(filepath.Join(root, "mise.agent.toml"), []byte("[tools]\n\"go:golang.org/x/tools/gopls\" = \""+version+"\"\n"), 0600))
			require.ErrorContains(t, checkVersions(root), "必须固定稳定的精确版本")
		})
	}
}

func TestVersionsRejectAgentDocumentationMismatch(t *testing.T) {
	root := versionFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.agent.toml"), []byte("[tools]\n\"go:golang.org/x/tools/gopls\" = \"0.24.1\"\n"), 0600))
	require.ErrorContains(t, checkVersions(root), "README 工具 gopls")
}

func TestVersionsRejectCIMismatch(t *testing.T) {
	root := versionFixture(t)
	path := filepath.Join(root, ".github/workflows/ci.yml")
	//nolint:gosec // G304：路径仅来自当前测试创建的临时项目，不读取用户文件。
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	//nolint:gosec // G703：只修改临时 fixture 的固定 CI 配置，用于验证版本不一致。
	require.NoError(t, os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "version: 2026.10.21", "version: 2026.10.20")), 0600))
	require.ErrorContains(t, checkVersions(root), "CI mise 版本不一致")
}
func TestHistoricalVersionTextIsNotCurrentDeclaration(t *testing.T) {
	root := versionFixture(t)
	path := filepath.Join(root, "docs/superpowers/specs/history.md")
	//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("Go 1.20.0 mise 2020.1.1"), 0600))
	require.NoError(t, checkVersions(root))
	file := filepath.Join(root, "README.md")
	//nolint:gosec // G304：测试路径和权限只来自当前临时 fixture，不读取用户文件。
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
	require.NoError(t, os.WriteFile(file, []byte(strings.Replace(string(data), "| Go | 1.27.3 |", "| Go | 1.27.30 |", 1)), 0600))
	require.Error(t, checkVersions(root))
}

func TestVersionRejectsPrereleaseOrBuildSuffix(t *testing.T) {
	for _, version := range []string{"1.27.30", "1.27.3-rc.1", "1.27.3+patched"} {
		require.Error(t, matchToolVersion("go", "go version go"+version+" linux/amd64", "1.27.3"))
	}
}

func TestVersionsGoToolchain(t *testing.T) {
	tests := []struct {
		name      string
		minimum   string
		toolchain string
		wantError bool
	}{
		{name: "implicit-matching", minimum: "1.27.3"},
		{name: "explicit-matching", minimum: "1.27.3", toolchain: "go1.27.3"},
		{name: "explicit-newer-than-minimum", minimum: "1.27.0", toolchain: "go1.27.3"},
		{name: "implicit-mismatching", minimum: "1.27.0", wantError: true},
		{name: "explicit-mismatching", minimum: "1.27.3", toolchain: "go1.27.4", wantError: true},
		{name: "minimum-too-new", minimum: "1.27.4", toolchain: "go1.27.3", wantError: true},
		{name: "minimum-different-series", minimum: "1.26.0", toolchain: "go1.27.3", wantError: true},
		{name: "missing-minimum", toolchain: "go1.27.3", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := versionFixture(t)
			module := "module example.com/versions\n"
			if tt.minimum != "" {
				module += "\ngo " + tt.minimum + "\n"
			}
			if tt.toolchain != "" {
				module += "\ntoolchain " + tt.toolchain + "\n"
			}
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0600))
			err := checkVersions(root)
			if tt.wantError {
				require.ErrorContains(t, err, "模块最低 Go 版本、工具链与 mise 不一致")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
