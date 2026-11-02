package command

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func waitMarker(t *testing.T, path string) {
	t.Helper()
	limit := time.Now().Add(3 * time.Second)
	for time.Now().Before(limit) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("未出现进程同步标记 %s", path)
}

// TestHelperProcess 只运行测试程序自身，不加载应用配置或用户状态。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("COMMAND_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	switch args[0] {
	case "echo":
		cwd, _ := os.Getwd()
		data, _ := json.Marshal([]any{args[1:], os.Getenv("VALUE"), cwd})
		_, _ = os.Stdout.Write(data)
	case "bytes":
		_, _ = os.Stdout.Write([]byte{0, 255, '\n'})
		_, _ = os.Stderr.Write([]byte{254, 0, 'x'})
	case "large":
		_, _ = os.Stdout.Write(bytes.Repeat([]byte{'a'}, 2<<20))
		_, _ = os.Stderr.Write(bytes.Repeat([]byte{'b'}, 2<<20))
	case "exit":
		os.Exit(7)
	case "gate-fail":
		signal.Ignore(syscall.SIGTERM)
		//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
		_ = os.WriteFile(args[1], []byte("ready"), 0600)
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			if _, err := os.Stat(args[2]); err == nil {
				os.Exit(7)
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(8)
	case "term-marker":
		received := make(chan os.Signal, 1)
		signal.Notify(received, syscall.SIGTERM)
		defer signal.Stop(received)
		//nolint:gosec // G703：测试路径和权限只来自当前临时 fixture，不读取用户文件。
		_ = os.WriteFile(args[1], []byte("ready"), 0600)
		select {
		case <-received:
			//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			_ = os.WriteFile(args[2], []byte("term"), 0600)
		case <-time.After(5 * time.Second):
			os.Exit(8)
		}
		time.Sleep(5 * time.Second)
	case "interval":
		start := time.Now().UnixNano()
		time.Sleep(100 * time.Millisecond)
		data, _ := json.Marshal([]int64{start, time.Now().UnixNano()})
		_, _ = os.Stdout.Write(data)
	case "sleep":
		delay, _ := time.ParseDuration(args[1])
		time.Sleep(delay)
		_, _ = os.Stdout.WriteString(args[2])
	case "heartbeat":
		signal.Ignore(syscall.SIGTERM)
		timer := time.AfterFunc(10*time.Second, func() { os.Exit(10) })
		defer timer.Stop()
		for {
			//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			_ = os.WriteFile(args[1], []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0600)
			time.Sleep(10 * time.Millisecond)
		}
	case "branch":
		executable, _ := os.Executable()
		//nolint:gosec // G204：测试只运行当前测试二进制或固定 Git/Go 命令，参数来自临时 fixture。
		child := exec.Command(executable, "-test.run=^TestHelperProcess$", "--", "spawn", args[1], args[2], "success")
		child.Env = os.Environ()
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Run(); err != nil {
			os.Exit(9)
		}
	case "spawn":
		executable, _ := os.Executable()
		//nolint:gosec // G204：测试只运行当前测试二进制或固定 Git/Go 命令，参数来自临时 fixture。
		child := exec.Command(executable, "-test.run=^TestHelperProcess$", "--", "heartbeat", args[1])
		child.Env = os.Environ()
		if args[2] == "pipes" {
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
		}
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		for i := 0; i < 200; i++ {
			//nolint:gosec // G703：路径只来自当前测试的临时目录或测试子进程的显式协议参数。
			if _, err := os.Stat(args[1]); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if args[3] == "wait" {
			time.Sleep(time.Minute)
		}
		if args[3] == "fail" {
			os.Exit(7)
		}
	}
	os.Exit(0)
}

func helper(t *testing.T, args ...string) (Invocation, Execution) {
	t.Helper()
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Invocation{Program: executable, Operation: "test.helper", Args: append([]string{"-test.run=^TestHelperProcess$", "--"}, args...)}, Execution{Dir: root, Env: []string{"COMMAND_HELPER=1", "GORACE=atexit_sleep_ms=0", "GO_CLI_STARTER_HOME=" + root, "HOME=" + root, "USERPROFILE=" + root, "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}}
}
