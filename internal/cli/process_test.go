package cli

import (
	"errors"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/scaffold"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestProcessExitCodes(t *testing.T) {
	name := "cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	project, err := scaffold.Inspect(root)
	require.NoError(t, err)
	cmd := exec.Command("go", "build", "-o", binary, "./"+project.CommandPath)
	cmd.Dir = root
	data, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", data)
	testscript.Run(t, testscript.Params{Dir: "testdata/process", Setup: func(env *testscript.Env) error {
		home := filepath.Join(env.WorkDir, "home")
		if err := os.MkdirAll(home, 0700); err != nil {
			return err
		}
		for _, key := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "APPDATA"} {
			env.Setenv(key, home)
		}
		env.Setenv("GO_CLI_STARTER_LOG_LEVEL", "")
		return nil
	}, Cmds: map[string]func(*testscript.TestScript, bool, []string){"cli": func(ts *testscript.TestScript, neg bool, args []string) {
		if neg || len(args) == 0 {
			ts.Fatalf("用法：cli <退出码> [参数...]")
		}
		want, err := strconv.Atoi(args[0])
		ts.Check(err)
		err = ts.Exec(binary, args[1:]...)
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				ts.Fatalf("执行程序失败：%v", err)
			}
			code = exit.ExitCode()
		}
		if code != want {
			ts.Fatalf("退出码=%d，期望=%d", code, want)
		}
	}}})
}
