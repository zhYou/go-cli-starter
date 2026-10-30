package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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
	case "marker":
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

func TestWindowsCanceledBeforeCreation(t *testing.T) {
	call, options := windowsHelper(t, "marker", filepath.Join(t.TempDir(), "marker"))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Run(ctx, call, options); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(call.Args[3]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("取消后执行了业务: %v", err)
	}
}

// 实际创建的进程必须在 Job 内保持挂起；取消后不得执行第一条业务指令。
func TestWindowsCanceledBetweenCreationAndResume(t *testing.T) {
	baseline := processHandleCount(t)
	defer func() {
		if count := processHandleCount(t); count > baseline+4 {
			t.Errorf("取消后句柄泄漏: before=%d after=%d", baseline, count)
		}
	}()
	for range 8 {
		marker := filepath.Join(t.TempDir(), "marker")
		call, options := windowsHelper(t, "marker", marker)
		options.IO = options.IO.WithDefaults()
		child, err := createWindows(t.Context(), call.Program, call.Args, options)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		active, queryErr := jobActiveProcesses(child.job)
		_, markerErr := os.Stat(marker)
		cancel()
		err = child.wait(ctx, DefaultWaitDelay)
		if queryErr != nil || active != 1 {
			t.Fatalf("创建时未绑定 Job: active=%d err=%v", active, queryErr)
		}
		if !errors.Is(markerErr, os.ErrNotExist) {
			t.Fatalf("创建后已执行业务: %v", markerErr)
		}
		var exit *exec.ExitError
		if !errors.Is(err, context.Canceled) || !errors.As(err, &exit) {
			t.Fatalf("丢失取消/退出状态: %v", err)
		}
		if _, err = os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Resume前取消仍执行了业务: %v", err)
		}
	}
}

func TestWindowsCancelAfterResumeAndDuringSpawn(t *testing.T) {
	for _, mode := range []string{"marker", "spawn", "spawn-loop"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			call, options := windowsHelper(t, "marker", marker)
			switch mode {
			case "spawn":
				call, options = helper(t, "spawn", marker, "pipes", "wait")
			case "spawn-loop":
				call, options = windowsHelper(t, "spawn-loop", marker)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, call, options) }()
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(marker); err == nil {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := os.Stat(marker); err != nil {
				cancel()
				waitErr := <-done
				t.Fatalf("未观察到 helper 就绪: %v；执行结果: %v", err, waitErr)
			}
			cancel()
			err := <-done
			var exit *exec.ExitError
			if !errors.Is(err, context.Canceled) || !errors.As(err, &exit) {
				t.Fatalf("取消退出错误: %v", err)
			}
			before, _ := os.ReadFile(marker)
			time.Sleep(80 * time.Millisecond)
			after, _ := os.ReadFile(marker)
			if !bytes.Equal(before, after) {
				t.Fatal("取消后后代继续写入")
			}
		})
	}
}

func TestWindowsNativeIOAndBorrowedFiles(t *testing.T) {
	call, options := windowsHelper(t, "stdio")
	options.IO.In = strings.NewReader("输入\x00字节")
	result := Capture(t.Context(), call, options, CaptureOptions{})
	if result.Err != nil || string(result.Stdout) != "输入\x00字节" || string(result.Stderr) != "诊断" {
		t.Fatalf("复制标准流: %+v", result)
	}
	options.IO.In = nil
	if err := Run(t.Context(), call, options); err != nil {
		t.Fatalf("默认 EOF/discard: %v", err)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "borrowed"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	options.IO.Out = file
	options.IO.ErrOut = file
	if err = Run(t.Context(), call, options); err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("still open"); err != nil {
		t.Fatalf("关闭了借用句柄: %v", err)
	}
	var shared bytes.Buffer
	options.IO.Out = &shared
	options.IO.ErrOut = &shared
	options.IO.In = strings.NewReader("输入")
	if err = Run(t.Context(), call, options); err != nil {
		t.Fatal(err)
	}
	if shared.String() != "输入诊断" {
		t.Fatalf("共享输出流: %q", shared.String())
	}
}

func TestWindowsEnvironmentAndArgumentEscaping(t *testing.T) {
	args := []string{"", `C:\path with space\`, `a\"b`, "\t", "中文", "a=b"}
	for _, explicitEmpty := range []bool{false, true} {
		call, options := windowsHelper(t, append([]string{"env"}, args...)...)
		env := options.Env[:0]
		for _, item := range options.Env {
			if !strings.HasPrefix(item, "SYSTEMROOT=") {
				env = append(env, item)
			}
		}
		options.Env = append(env, "", "VALUE=old", "vAlUe=new", "")
		if explicitEmpty {
			options.Env = append(options.Env, "SystemRoot=")
		}
		r := Capture(t.Context(), call, options, CaptureOptions{})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		var raw []json.RawMessage
		if err := json.Unmarshal(r.Stdout, &raw); err != nil {
			t.Fatal(err)
		}
		expected := os.Getenv("SYSTEMROOT")
		if explicitEmpty {
			expected = ""
		}
		var root, value string
		var gotArgs []string
		_ = json.Unmarshal(raw[0], &root)
		_ = json.Unmarshal(raw[2], &value)
		_ = json.Unmarshal(raw[3], &gotArgs)
		if root != expected || string(raw[1]) != "true" || value != "new" || !reflect.DeepEqual(gotArgs, args) {
			t.Fatalf("环境/argv: %s", r.Stdout)
		}
	}
}

func processHandleCount(t *testing.T) uint32 {
	t.Helper()
	var count uint32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	ok, _, err := proc.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		t.Fatal(err)
	}
	return count
}

func TestWindowsHandleWhitelistAndCleanup(t *testing.T) {
	event, err := windows.CreateEvent(&windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(event) }()
	call, options := windowsHelper(t, "handle", strconv.FormatUint(uint64(event), 10))
	if err = Run(t.Context(), call, options); err != nil {
		t.Fatalf("泄漏了无关可继承句柄: %v", err)
	}
	if status, waitErr := windows.WaitForSingleObject(event, 0); waitErr != nil || status != uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("继承了白名单外事件: status=%d err=%v", status, waitErr)
	}
	baseline := processHandleCount(t)
	for i := range 32 {
		currentOptions := options
		if i%2 == 0 {
			currentOptions.Dir = filepath.Join(t.TempDir(), "missing-directory")
		}
		err := Run(t.Context(), call, currentOptions)
		if (i%2 == 0) != (err != nil) {
			t.Fatalf("准备失败/成功结果与预期不同: %v", err)
		}
	}
	if count := processHandleCount(t); count > baseline+4 {
		t.Fatalf("句柄泄漏: before=%d after=%d", baseline, count)
	}
}

func TestWindowsConcurrentJobIsolation(t *testing.T) {
	sibling, opts := helper(t, "sleep", "250ms", "survived")
	canceled, cancelOpts := helper(t, "sleep", "5s", "never")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan Result, 1)
	go func() { done <- Capture(t.Context(), sibling, opts, CaptureOptions{}) }()
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	r := Capture(ctx, canceled, cancelOpts, CaptureOptions{})
	if !errors.Is(r.Err, context.Canceled) {
		t.Fatal(r.Err)
	}
	survivor := <-done
	if survivor.Err != nil || string(survivor.Stdout) != "survived" {
		t.Fatalf("取消误伤并发 Job: %+v", survivor)
	}
}

func TestWindowsUnexpectedSuspendCountStopsBeforeBusiness(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	call, options := windowsHelper(t, "marker", marker)
	options.IO = options.IO.WithDefaults()
	child, err := createWindows(t.Context(), call.Program, call.Args, options)
	if err != nil {
		t.Fatal(err)
	}
	// 真正增加挂起计数，验证 Resume 返回2时不会误报成功或遗留挂起进程。
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("SuspendThread")
	previous, _, suspendErr := proc.Call(uintptr(child.info.Thread))
	if uint32(previous) == 0xffffffff {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_ = child.wait(ctx, DefaultWaitDelay)
		t.Fatal(suspendErr)
	}
	err = child.wait(t.Context(), DefaultWaitDelay)
	if err == nil {
		t.Fatal("未检测异常挂起计数")
	}
	if _, err = os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("异常 Resume 执行了业务: %v", err)
	}
}

type windowsFailWriter struct{ err error }

func (writer windowsFailWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestWindowsCopyFailureIsReturned(t *testing.T) {
	call, options := windowsHelper(t, "stdio")
	sentinel := errors.New("写入被拒绝")
	options.IO.ErrOut = windowsFailWriter{sentinel}
	if err := Run(t.Context(), call, options); !errors.Is(err, sentinel) {
		t.Fatalf("丢失借用 writer 错误: %v", err)
	}
}

func TestWindowsBorrowedInputFileRemainsOpen(t *testing.T) {
	input, err := os.Create(filepath.Join(t.TempDir(), "input"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close() }()
	if _, err = input.WriteString("borrowed input"); err != nil {
		t.Fatal(err)
	}
	if _, err = input.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	call, options := windowsHelper(t, "stdio")
	options.IO.In = input
	r := Capture(t.Context(), call, options, CaptureOptions{})
	if r.Err != nil || string(r.Stdout) != "borrowed input" {
		t.Fatalf("文件输入: %+v", r)
	}
	if _, err = input.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("关闭了借用输入文件: %v", err)
	}
}

func TestWindowsDrainDeadlineClosesOnlyOwnedPipes(t *testing.T) {
	// 借用管道有独立的有界释放；复制关闭自有写端后仍不得关闭借用 reader。
	borrowed, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = borrowed.Close() }()
	defer func() { _ = writer.Close() }()
	call, options := helper(t, "exit")
	// 包装为 Reader，真实文件读取在 goroutine 中进行，驱动 WaitDelay 分支。
	options.IO.In = struct{ io.Reader }{borrowed}
	options.WaitDelay = 10 * time.Millisecond
	released := make(chan struct{})
	timer := time.AfterFunc(500*time.Millisecond, func() { _ = writer.Close(); close(released) })
	defer timer.Stop()
	r := Capture(t.Context(), call, options, CaptureOptions{})
	<-released
	if !errors.Is(r.Err, exec.ErrWaitDelay) || r.ExitCode != 7 {
		t.Fatalf("排空宽限/根退出状态: %+v", r)
	}
	if _, err = borrowed.Stat(); err != nil {
		t.Fatalf("关闭了借用 reader: %v", err)
	}
}

func TestWindowsResumeFailureCleansSuspendedProcess(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	call, options := windowsHelper(t, "marker", marker)
	options.IO = options.IO.WithDefaults()
	child, err := createWindows(t.Context(), call.Program, call.Args, options)
	if err != nil {
		t.Fatal(err)
	}
	var restricted windows.Handle
	current := windows.CurrentProcess()
	err = windows.DuplicateHandle(current, child.info.Thread, current, &restricted, windows.THREAD_QUERY_INFORMATION, false, 0)
	if err != nil {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_ = child.wait(ctx, DefaultWaitDelay)
		t.Fatal(err)
	}
	// 保留真实线程，但移除 Resume 所需权限，以真实系统错误驱动失败路径。
	_ = windows.CloseHandle(child.info.Thread)
	child.info.Thread = restricted
	err = child.wait(t.Context(), DefaultWaitDelay)
	var exit *exec.ExitError
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) || !errors.As(err, &exit) {
		t.Fatalf("恢复失败/退出状态: %v", err)
	}
	if _, err = os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Resume失败后执行业务: %v", err)
	}
}
