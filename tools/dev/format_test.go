package main

import (
	"os"
	"path/filepath"
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
