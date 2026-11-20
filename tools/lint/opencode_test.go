package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenCodeConfigAcceptsRepository(t *testing.T) {
	require.NoError(t, checkOpenCodeConfig(filepath.Join("..", "..")))
}

func TestOpenCodeConfigAcceptsJSONC(t *testing.T) {
	root, contents := openCodeFixture(t)
	commented := "// 项目级 OpenCode 配置\n" + strings.Replace(
		string(contents),
		`"directory": ".worktrees"`,
		`"directory": ".worktrees", // 原生 worktree 目录`,
		1,
	)
	writeOpenCodeFixture(t, root, commented)
	require.NoError(t, checkOpenCodeConfig(root))
}

func TestNormalizeJSONC(t *testing.T) {
	normalized, err := normalizeJSONC([]byte(`{
  // 行注释
  "url": "https://example.com/*",
  /* 块注释 */
  "values": [1, 2,],
}`))
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(normalized, &decoded))
	require.Equal(t, "https://example.com/*", decoded["url"])
	require.Equal(t, []any{float64(1), float64(2)}, decoded["values"])

	normalized, err = normalizeJSONC([]byte(`{"path": "/* 字符串内不按注释处理 */", "quote": "\"// 仍属于字符串\""}`))
	require.NoError(t, err)
	var stringValues map[string]any
	require.NoError(t, json.Unmarshal(normalized, &stringValues))
	require.Equal(t, "/* 字符串内不按注释处理 */", stringValues["path"])
	require.Equal(t, `"// 仍属于字符串"`, stringValues["quote"])

	_, err = normalizeJSONC([]byte(`{"value": 1 /* 未闭合`))
	require.ErrorContains(t, err, "块注释未闭合")
}

func TestOpenCodeConfigRejectsDrift(t *testing.T) {
	root, contents := openCodeFixture(t)
	for _, tt := range []struct {
		name string
		old  string
		new  string
	}{
		{"超时", `"startup": 300000`, `"startup": 30000`},
		{"工作区", `"directory": ".worktrees"`, `"directory": "../worktrees"`},
		{"Codex context", `"mcp:serve:opencode"`, `"mcp:serve"`},
		{"MCP 目录", `"cwd": "."`, `"cwd": ".."`},
		{"服务超时覆盖", `"cwd": "."`, `"cwd": ".",
        "timeout": {"startup": 30000}`},
		{"默认审批", `"resource": "*",
      "effect": "ask"`, `"resource": "*",
      "effect": "allow"`},
		{"Graphify 工具", `"graphify_god_nodes"`, `"graphify_debug"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, string(contents), tt.old)
			path := writeOpenCodeFixture(t, root, strings.Replace(string(contents), tt.old, tt.new, 1))
			err := checkOpenCodeConfig(root)
			require.Error(t, err)
			require.Contains(t, err.Error(), filepath.FromSlash(openCodeConfigPath))
			require.FileExists(t, path)
		})
	}
}

func TestOpenCodeConfigRejectsInvalidOrMissingFile(t *testing.T) {
	root, _ := openCodeFixture(t)
	path := writeOpenCodeFixture(t, root, "{")
	require.ErrorContains(t, checkOpenCodeConfig(root), "JSONC 无效")
	writeOpenCodeFixture(t, root, `{"value": 1 /* 未闭合`)
	require.ErrorContains(t, checkOpenCodeConfig(root), "块注释未闭合")
	require.NoError(t, os.Remove(path))
	require.Error(t, checkOpenCodeConfig(root))
}

func TestOpenCodeConfigRejectsSymlink(t *testing.T) {
	root, contents := openCodeFixture(t)
	target := filepath.Join(t.TempDir(), "opencode.jsonc")
	require.NoError(t, os.WriteFile(target, contents, 0o600))
	path := filepath.Join(root, filepath.FromSlash(openCodeConfigPath))
	require.NoError(t, os.Remove(path))
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("当前系统无法创建符号链接: %v", err)
	}
	require.ErrorContains(t, checkOpenCodeConfig(root), "不是普通文件")
}

func openCodeFixture(t *testing.T) (string, []byte) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(openCodeConfigPath)))
	require.NoError(t, err)
	root := t.TempDir()
	writeOpenCodeFixture(t, root, string(contents))
	return root, contents
}

func writeOpenCodeFixture(t *testing.T, root, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(openCodeConfigPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	//nolint:gosec // G703：只改写当前测试创建的临时 fixture。
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}
