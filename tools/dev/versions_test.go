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
	for _, name := range []string{"mise.toml", "mise.lock", "go.mod", "README.md", "AGENTS.md", ".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		dest := filepath.Join(root, name)
		//nolint:gosec // G301：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0755))
		require.NoError(t, copyFile(filepath.Join("../..", name), dest))
	}
	return root
}
func TestVersionsRejectSubstringAndMissingPlatform(t *testing.T) {
	require.Error(t, matchToolVersion("go", "go version go1.27.30 linux/amd64", "1.27.3"))
	require.NoError(t, matchToolVersion("go", "go version go1.27.3 linux/amd64", "1.27.3"))
	root := versionFixture(t)
	require.NoError(t, checkVersions(root))
	file := filepath.Join(root, "mise.lock")
	//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
	//nolint:gosec // G304：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	text := string(data)
	a := strings.Index(text, "[tools.go.\"platforms.windows-arm64\"]")
	require.GreaterOrEqual(t, a, 0)
	b := strings.Index(text[a+1:], "\n[")
	require.GreaterOrEqual(t, b, 0)
	text = text[:a] + text[a+1+b:]
	//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
	require.NoError(t, os.WriteFile(file, []byte(text), 0600))
	require.Error(t, checkVersions(root))
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
