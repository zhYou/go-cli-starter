package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
)

// 用 Go overlay 重建真实内嵌目录，避免修改仓库或共享可变全局变量。
func TestCorruptCatalogEndToEnd(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
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

func TestCorruptCatalogProcess(t *testing.T) {
	if os.Getenv("TEST_CORRUPT_I18N_CATALOG") != "1" {
		t.Skip("仅由损坏目录子进程执行")
	}
	var sink bytes.Buffer
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "debug", Output: "stderr"}, Writer: &sink}, func(err error) { t.Error(err) })
	call, options := helper(t, "exit")
	options.Logger = logger
	err := Run(t.Context(), call, options)
	var exit *exec.ExitError
	require.True(t, errors.As(err, &exit))
	require.Equal(t, 7, exit.ExitCode())
	lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	var start, finish map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &start))
	require.NoError(t, json.Unmarshal(lines[1], &finish))
	require.Equal(t, "Operation failed [command.started]", start["msg"])
	require.Equal(t, "Operation failed [command.finished]", finish["msg"])
	require.Equal(t, "failed", finish["outcome"])
	require.Equal(t, float64(7), finish["exit_code"])

	bundle, err := messages.NewBundle(fstest.MapFS{"active.en.toml": {Data: []byte("[\"command.started\"]\nother = \"Injected start\"\n[\"command.finished\"]\nother = \"Injected finish\"\n")}})
	require.NoError(t, err)
	sink.Reset()
	options.Localizer = bundle.Localizer("en")
	err = Run(t.Context(), call, options)
	require.True(t, errors.As(err, &exit))
	require.Equal(t, 7, exit.ExitCode())
	require.Contains(t, sink.String(), "Injected start")
	require.Contains(t, sink.String(), "Injected finish")
}
