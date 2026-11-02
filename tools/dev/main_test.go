package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatCheckIsReadOnlyAndUsesBothFormatters(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, copyFile("../../.golangci.yml", filepath.Join(root, ".golangci.yml")))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/format\n\ngo 1.27.0\n"), 0600))
	file := filepath.Join(root, "main.go")
	// gofmt 修复函数间距，goimports 删除未使用的 os 导入。
	original := []byte(`package main
import "os"
func main( ){}
`)
	require.NoError(t, os.WriteFile(file, original, 0600))
	t.Chdir(root)
	require.Error(t, format(false))
	//nolint:gosec // G304：只读取本测试在临时目录创建的格式 fixture。
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, original, data)
	require.NoError(t, format(true))
	require.NoError(t, format(false))
	//nolint:gosec // G304：只读取本测试在临时目录创建的格式 fixture。
	data, err = os.ReadFile(file)
	require.NoError(t, err)
	require.NotContains(t, string(data), "os")
	require.Contains(t, string(data), "func main() {}")
}

// 开发入口必须执行真实检查，并向调用者返回配置错误。
func TestI18nAndConfigCheckTasks(t *testing.T) {
	require.NoError(t, run("i18n-check"))
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll("configs", 0700))
	example := filepath.Join("configs", "config.example.toml")
	require.NoError(t, os.WriteFile(example, []byte("[logging]\nlevel = \"invalid\"\n"), 0600))
	err := run("config-check")
	require.Error(t, err)
	require.Contains(t, err.Error(), "config.example.toml")
	require.NoError(t, os.WriteFile(example, []byte("[logging]\nlevel = \"info\"\n"), 0600))
	require.NoError(t, run("config-check"))
}

func TestUnknownTaskUsesChinese(t *testing.T) {
	err := run("unknown")
	if err == nil || !strings.Contains(err.Error(), "未知任务") || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("未知任务错误=%v", err)
	}
}
