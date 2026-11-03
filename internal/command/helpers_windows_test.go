package command

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestWindowsHelper 使用测试二进制及临时目录，不读取应用配置。
func TestWindowsHelper(t *testing.T) {
	if os.Getenv("WINDOWS_COMMAND_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	switch args[0] {
	case "stdio":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(9)
		}
		_, _ = os.Stdout.Write(data)
		_, _ = os.Stderr.WriteString("诊断")
	case "env":
		value, present := os.LookupEnv("SYSTEMROOT")
		data, _ := json.Marshal([]any{value, present, os.Getenv("VALUE"), args[1:]})
		_, _ = os.Stdout.Write(data)
	case "spawn-loop":
		executable, _ := os.Executable()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			//nolint:gosec // G204：辅助进程只执行自身测试二进制；路径来自当前测试生成的临时目录。
			child := exec.Command(executable, "-test.run=^TestHelperProcess$", "--", "heartbeat", args[1])
			child.Env = os.Environ()
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if err := child.Start(); err != nil {
				os.Exit(9)
			}
			_ = child.Process.Release()
			time.Sleep(10 * time.Millisecond)
		}
	case "host-job":
		if err := json.NewEncoder(os.Stdout).Encode(windowsHostJobScenario(args[1], args[2] == "quota")); err != nil {
			os.Exit(9)
		}
	case "marker-exit":
		//nolint:gosec // G703：辅助进程只执行自身测试二进制；路径来自当前测试生成的临时目录。
		if err := os.WriteFile(args[1], []byte("executed"), 0600); err != nil {
			os.Exit(9)
		}
	case "marker":
		//nolint:gosec // G703：辅助进程只执行自身测试二进制；路径来自当前测试生成的临时目录。
		_ = os.WriteFile(args[1], []byte("executed"), 0600)
		time.Sleep(5 * time.Second)
	case "handle":
		raw, _ := strconv.ParseUint(args[1], 10, 64)
		// 数字句柄在子进程内可能复用；只通过父进程事件的实际信号判定继承。
		_ = windows.SetEvent(windows.Handle(raw))
	}
	os.Exit(0)
}

func windowsHelper(t *testing.T, args ...string) (Invocation, Execution) {
	t.Helper()
	call, options := helper(t)
	call.Args = append([]string{"-test.run=^TestWindowsHelper$", "--"}, args...)
	options.Env = append(options.Env, "WINDOWS_COMMAND_HELPER=1")
	return call, options
}

func processHandleCount(t *testing.T) uint32 {
	t.Helper()
	var count uint32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	//nolint:gosec // G103：测试调用同步 Win32 API，传入固定原生布局与局部对象的有效地址。
	ok, _, err := proc.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		t.Fatal(err)
	}
	return count
}
