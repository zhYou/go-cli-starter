package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
