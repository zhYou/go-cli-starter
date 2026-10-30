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
	"syscall"
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
			//nolint:gosec // G304：辅助进程只执行自身测试二进制；路径来自当前测试生成的临时目录。
			before, _ := os.ReadFile(marker)
			time.Sleep(80 * time.Millisecond)
			//nolint:gosec // G304：辅助进程只执行自身测试二进制；路径来自当前测试生成的临时目录。
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
	//nolint:gosec // G103：测试调用同步 Win32 API，传入固定原生布局与局部对象的有效地址。
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
	//nolint:gosec // G115：Win32 返回 DWORD；uintptr 来自系统 ABI，低 32 位保留文档规定的结果或错误码。
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

// 夹具结果明确区分宿主建立、挂起时归属与实际执行结果。
type windowsHostJobResult struct {
	Stage           string `json:"stage"`
	SetupError      string `json:"setup_error"`
	LimitFlags      uint32 `json:"limit_flags"`
	ActiveLimit     uint32 `json:"active_limit"`
	UIRestrictions  uint32 `json:"ui_restrictions"`
	HelperInHost    bool   `json:"helper_in_host"`
	InitialActive   uint32 `json:"initial_active"`
	Created         bool   `json:"created"`
	ChildInHost     bool   `json:"child_in_host"`
	ChildInOwned    bool   `json:"child_in_owned"`
	OwnedActive     uint32 `json:"owned_active"`
	BeforeBusiness  bool   `json:"before_business"`
	ChildQueryError string `json:"child_query_error"`
	RunError        string `json:"run_error"`
	NativeError     uint32 `json:"native_error"`
	HasExitState    bool   `json:"has_exit_state"`
	Canceled        bool   `json:"canceled"`
	Active          uint32 `json:"active"`
	QueryError      string `json:"query_error"`
}

// 只由独立 helper 调用，主 go test 进程永远不加入此 Job。
func windowsHostJobScenario(marker string, quota bool) (result windowsHostJobResult) {
	result.Stage = "create_host_job"
	host, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		result.SetupError = err.Error()
		return result
	}
	assigned := false
	defer func() {
		if !assigned {
			_ = windows.CloseHandle(host)
		}
	}()
	// 显式允许 BREAKAWAY，但不设 SILENT_BREAKAWAY；错误后端若主动脱离宿主，会被归属或 marker 检出。
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if quota {
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
		limits.BasicLimitInformation.ActiveProcessLimit = 1
	}
	result.Stage = "set_host_limits"
	//nolint:gosec // G103：同步 Win32 API 读取固定原生结构的有效地址。
	if _, err = windows.SetInformationJobObject(host, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		result.SetupError = err.Error()
		return result
	}
	restrictions := windows.JOBOBJECT_BASIC_UI_RESTRICTIONS{}
	if !quota {
		restrictions.UIRestrictionsClass = windows.JOB_OBJECT_UILIMIT_HANDLES
	}
	result.Stage = "set_ui_restrictions"
	//nolint:gosec // G103：同步 Win32 API 读取固定原生结构的有效地址。
	if _, err = windows.SetInformationJobObject(host, windows.JobObjectBasicUIRestrictions, uintptr(unsafe.Pointer(&restrictions)), uint32(unsafe.Sizeof(restrictions))); err != nil {
		result.SetupError = err.Error()
		return result
	}
	result.Stage = "assign_helper_to_host"
	if err = windows.AssignProcessToJobObject(host, windows.CurrentProcess()); err != nil {
		result.SetupError = err.Error()
		return result
	}
	// 由 helper 进程寿命持有：先序列化结果再 os.Exit；提前 Close 会终止自身。
	// helper 正常退出或被外层超时终止时，内核关闭句柄并清理遗留后代。
	assigned = true
	result.Stage = "query_host_configuration"
	//nolint:gosec // G103：同步 Win32 API 读取固定原生结构的有效地址。
	if err = windows.QueryInformationJobObject(host, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil); err != nil {
		result.SetupError = err.Error()
		return result
	}
	result.LimitFlags = limits.BasicLimitInformation.LimitFlags
	result.ActiveLimit = limits.BasicLimitInformation.ActiveProcessLimit
	//nolint:gosec // G103：同步 Win32 API 读取固定原生结构的有效地址。
	if err = windows.QueryInformationJobObject(host, windows.JobObjectBasicUIRestrictions, uintptr(unsafe.Pointer(&restrictions)), uint32(unsafe.Sizeof(restrictions)), nil); err != nil {
		result.SetupError = err.Error()
		return result
	}
	result.UIRestrictions = restrictions.UIRestrictionsClass
	if err = probeWindowsProcessInJob(windows.CurrentProcess(), host, &result.HelperInHost); err != nil {
		result.SetupError = err.Error()
		return result
	}
	result.InitialActive, err = jobActiveProcesses(host)
	if err != nil {
		result.SetupError = err.Error()
		return result
	}
	executable, err := os.Executable()
	if err != nil {
		result.SetupError = err.Error()
		return result
	}
	result.Stage = "run_backend"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	call := Invocation{Program: executable, Args: []string{"-test.run=^TestWindowsHelper$", "--", "marker-exit", marker}}
	options := Execution{Dir: filepath.Dir(marker), Env: os.Environ()}
	if quota {
		// helper 已占满唯一名额。新子进程必须在创建阶段失败，不能降级或逃离宿主。
		err = Run(ctx, call, options)
	} else {
		options.IO = options.IO.WithDefaults()
		var child *windowsChild
		child, err = createWindows(ctx, call.Program, call.Args, options)
		if err == nil {
			result.Created = true
			queryErr := probeWindowsProcessInJob(child.info.Process, host, &result.ChildInHost)
			queryErr = errors.Join(queryErr, probeWindowsProcessInJob(child.info.Process, child.job, &result.ChildInOwned))
			var activeErr error
			result.OwnedActive, activeErr = jobActiveProcesses(child.job)
			queryErr = errors.Join(queryErr, activeErr)
			if queryErr != nil {
				result.ChildQueryError = queryErr.Error()
			}
			//nolint:gosec // G703：独立测试 helper 仅检查父测试生成并传入的临时 marker 路径。
			_, markerErr := os.Stat(marker)
			result.BeforeBusiness = errors.Is(markerErr, os.ErrNotExist)
			if queryErr != nil || !result.ChildInHost || !result.ChildInOwned || result.OwnedActive != 1 || !result.BeforeBusiness {
				cancel()
			}
			err = child.wait(ctx, DefaultWaitDelay)
		}
	}
	if err != nil {
		result.RunError = err.Error()
	}
	if native, ok := errors.AsType[syscall.Errno](err); ok {
		//nolint:gosec // G115：Win32 错误码是 DWORD，转换保留系统返回值。
		result.NativeError = uint32(native)
	}
	var exit *exec.ExitError
	result.HasExitState = errors.As(err, &exit)
	result.Canceled = errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	result.Active, err = jobActiveProcesses(host)
	if err != nil {
		result.QueryError = err.Error()
	}
	result.Stage = "complete"
	return result
}

func runWindowsHostJobFixture(t *testing.T, quota bool) (windowsHostJobResult, string) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "business-marker")
	mode := "ui"
	if quota {
		mode = "quota"
	}
	call, options := windowsHelper(t, "host-job", marker, mode)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// 原生独立 helper 构造宿主，不经过被测后端，不改主测试进程的 Job 归属。
	//nolint:gosec // G204：辅助进程仅执行自身测试二进制，路径来自当前测试。
	fixture := exec.CommandContext(ctx, call.Program, call.Args...)
	fixture.Dir, fixture.Env = options.Dir, options.Env
	fixture.WaitDelay = DefaultWaitDelay
	output, err := fixture.CombinedOutput()
	if err != nil {
		t.Fatalf("宿主夹具未完成，未覆盖该场景: %v；%s", err, output)
	}
	var result windowsHostJobResult
	if err = json.Unmarshal(output, &result); err != nil {
		t.Fatalf("夹具结果无效，未覆盖该场景: %v；%s", err, output)
	}
	t.Logf("宿主 Job 原生证据: %s", output)
	if result.SetupError != "" || result.Stage != "complete" {
		// 准备失败不能证明后端拒绝；明确失败，不 Skip。
		t.Fatalf("未覆盖宿主场景，夹具在 %s 阶段失败: %s", result.Stage, result.SetupError)
	}
	wantFlags := uint32(windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE)
	wantUI, wantLimit := uint32(windows.JOB_OBJECT_UILIMIT_HANDLES), uint32(0)
	if quota {
		wantFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
		wantUI, wantLimit = 0, 1
	}
	if result.LimitFlags != wantFlags || result.UIRestrictions != wantUI || result.ActiveLimit != wantLimit || !result.HelperInHost || result.InitialActive != 1 {
		t.Fatalf("宿主原生限制或初始归属不符: %+v", result)
	}
	if result.QueryError != "" || result.Active != 1 {
		t.Fatalf("收尾后宿主除 helper 外仍有活动进程: %+v", result)
	}
	return result, marker
}

func TestWindowsIncompatibleHostJobRejectsBeforeBusiness(t *testing.T) {
	result, marker := runWindowsHostJobFixture(t, true)
	// 官方只保证超出活动进程配额时拒绝分配，不保证特定 GetLastError 值。
	// https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject
	if result.RunError == "" || result.NativeError == 0 || result.HasExitState || result.Canceled {
		t.Fatalf("未在创建时拒绝已满宿主 Job: %+v", result)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("已满宿主中执行了业务，疑似 breakaway 降级: %v", err)
	}
}

func TestWindowsUIRestrictedHostPreservesJobMembership(t *testing.T) {
	result, marker := runWindowsHostJobFixture(t, false)
	if !result.Created {
		// 兼容旧主机严格拒绝 UI 嵌套的行为；未知创建错误、业务执行或退出状态均不能通过。
		if result.NativeError != uint32(windows.ERROR_ACCESS_DENIED) || result.RunError == "" || result.HasExitState || result.Canceled {
			t.Fatalf("UI 宿主创建返回未知失败: %+v", result)
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("UI 宿主拒绝创建后仍执行业务: %v", err)
		}
		return
	}
	// 当前原生 Windows 允许此组合；必须在 Resume 前同时属于宿主与自有 Job。
	if result.ChildQueryError != "" || !result.ChildInHost || !result.ChildInOwned || result.OwnedActive != 1 || !result.BeforeBusiness {
		t.Fatalf("创建时未满足 Job 双归属或挂起保障: %+v", result)
	}
	if result.RunError != "" || result.NativeError != 0 || result.HasExitState || result.Canceled {
		t.Fatalf("UI 宿主内正常执行失败: %+v", result)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("Resume 后未执行业务: %v", err)
	}
}

func TestWindowsTerminationFailureClosesJobBeforeWaiting(t *testing.T) {
	for _, phase := range []string{"cancel_suspended", "abort_before_wait_handle"} {
		t.Run(phase, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			call, options := windowsHelper(t, "marker", marker)
			options.IO = options.IO.WithDefaults()
			child, err := createWindows(t.Context(), call.Program, call.Args, options)
			if err != nil {
				t.Fatal(err)
			}
			defer child.streams.close()
			current := windows.CurrentProcess()
			var restricted, rescue windows.Handle
			// JOB_OBJECT_QUERY (0x0004) 不含 JOB_OBJECT_TERMINATE；保留真实 Job 的 kill-on-close。
			if err = windows.DuplicateHandle(current, child.job, current, &restricted, 0x0004, false, 0); err != nil {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				_ = child.wait(ctx, DefaultWaitDelay)
				t.Fatal(err)
			}
			// 独立救援句柄让回归在旧实现死锁时仍可终止 helper，不触碰已关闭后可能复用的原始句柄。
			if err = windows.DuplicateHandle(current, child.info.Process, current, &rescue, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
				_ = windows.CloseHandle(restricted)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				_ = child.wait(ctx, DefaultWaitDelay)
				t.Fatal(err)
			}
			defer func() { _ = windows.CloseHandle(rescue) }()
			_ = windows.CloseHandle(child.job)
			child.job = restricted
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var finish func() error
			if phase == "cancel_suspended" {
				finish = func() error { return child.wait(ctx, DefaultWaitDelay) }
			} else {
				// 模拟尚未获得独立 os.Process 等待句柄的创建中止；执行正式 abort 收尾函数。
				_ = child.process.Release()
				finish = func() error { return abortWindowsProcess(child.info, child.job) }
			}
			done := make(chan error, 1)
			go func() { done <- finish() }()
			select {
			case err = <-done:
			case <-time.After(3 * time.Second):
				rescueErr := windows.TerminateProcess(rescue, 1)
				select {
				case cleanupErr := <-done:
					t.Fatalf("终止Job失败后等待根进程死锁，救援=%v，收尾=%v", rescueErr, cleanupErr)
				case <-time.After(3 * time.Second):
					t.Fatalf("终止Job失败后未收尾，救援=%v", rescueErr)
				}
			}
			if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				t.Fatalf("丢失Job终止原生错误: %v", err)
			}
			if errors.Is(err, windows.ERROR_INVALID_HANDLE) {
				t.Fatalf("重复关闭原生句柄: %v", err)
			}
			if status, waitErr := windows.WaitForSingleObject(rescue, 0); waitErr != nil || status != windows.WAIT_OBJECT_0 {
				t.Fatalf("收尾返回后根进程仍存活: status=%d err=%v", status, waitErr)
			}
			var exitCode uint32
			if queryErr := windows.GetExitCodeProcess(rescue, &exitCode); queryErr != nil {
				t.Fatal(queryErr)
			}
			if phase == "cancel_suspended" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("丢失取消原因: %v", err)
				}
				// kill-on-close 未承诺非零退出码；独立原生句柄证明 ExitError 忠实反映实际退出。
				exit, hasExit := errors.AsType[*exec.ExitError](err)
				if hasExit != (exitCode != 0) || (hasExit && int64(exit.ExitCode()) != int64(exitCode)) {
					t.Fatalf("退出错误与原生状态不同: code=%d err=%v", exitCode, err)
				}
				// Go Windows 的 Wait 完成后释放等待句柄，再次等待返回 EINVAL。
				if _, secondWaitErr := child.process.Wait(); !errors.Is(secondWaitErr, syscall.EINVAL) {
					t.Fatalf("尚未完成原生 Wait: %v", secondWaitErr)
				}
			}
			t.Logf("kill-on-close 原生退出码: %d", exitCode)
			if _, err = os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("挂起进程执行业务: %v", err)
			}
		})
	}
}

func probeWindowsProcessInJob(process, job windows.Handle, member *bool) error {
	var inJob int32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	//nolint:gosec // G103：同步 Win32 API 的 BOOL 输出指向有效 int32 地址。
	ok, _, err := proc.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&inJob)))
	if ok == 0 {
		return err
	}
	*member = inJob != 0
	return nil
}
