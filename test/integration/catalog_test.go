package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 用 Go overlay 重建真实内嵌目录，避免修改仓库或共享可变全局变量。
func TestCorruptCatalogEndToEnd(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.toml")
	require.NoError(t, os.WriteFile(broken, []byte("[invalid\n"), 0600))
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, "internal", "locales", "active.en.toml"): broken}})
	require.NoError(t, err)
	overlayFile := filepath.Join(dir, "overlay.json")
	require.NoError(t, os.WriteFile(overlayFile, overlay, 0600))
	//nolint:gosec // G204：执行固定 Go 测试命令，overlay 路径来自当前测试的临时目录。
	cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", "-overlay", overlayFile, "-run", "^TestCorruptCatalogProcess$", "./internal/app/greeting", "./internal/app/session", "./internal/tools/command")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "TEST_CORRUPT_I18N_CATALOG=1", "GO_CLI_STARTER_HOME="+filepath.Join(dir, "app-home"), "GOPROXY=off", "GOTOOLCHAIN=local")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}
