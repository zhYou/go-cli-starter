package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

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
func TestTypedArgumentsAreIndependent(t *testing.T) {
	source := []string{"literal", ""}
	spec := Spec[string]{Program: "tool", Args: []Arg[string]{Literal[string](source...), Value(func(s string) string { return s })}}
	source[0] = "changed"
	a, err := spec.Build("a b")
	if err != nil {
		t.Fatal(err)
	}
	a.Args[0] = "mutated"
	b, err := spec.Build("$HOME")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Args, []string{"literal", "", "$HOME"}) {
		t.Fatal(b)
	}
	if _, err := (Spec[string]{Program: "tool", Validate: func(string) error { return errors.New("invalid") }}).Build(""); err == nil {
		t.Fatal("缺少校验错误")
	}
}
func TestCapturePreservesArgsEnvironmentDirectoryAndBytes(t *testing.T) {
	call, options := helper(t, "echo", "", "a b", "$HOME", "\"quoted\"")
	options.Env = append(options.Env, "VALUE=old", "VALUE=new")
	result := Capture(t.Context(), call, options, CaptureOptions{})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	var data []json.RawMessage
	if err := json.Unmarshal(result.Stdout, &data); err != nil {
		t.Fatal(err)
	}
	var args []string
	_ = json.Unmarshal(data[0], &args)
	if !reflect.DeepEqual(args, call.Args[3:]) {
		t.Fatalf("argv %q", args)
	}
	if string(data[1]) != `"new"` || string(data[2]) != strconv.Quote(options.Dir) {
		t.Fatalf("env/cwd %s", result.Stdout)
	}
	call.Args = []string{"-test.run=^TestHelperProcess$", "--", "bytes"}
	result = Capture(t.Context(), call, options, CaptureOptions{})
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if !bytes.Equal(result.Stdout, []byte{0, 255, '\n'}) || !bytes.Equal(result.Stderr, []byte{254, 0, 'x'}) {
		t.Fatalf("原始输出 %q %q", result.Stdout, result.Stderr)
	}
	original := append([]string(nil), options.Env...)
	updated := options.WithEnv("VALUE=other")
	updated.Env[0] = "mutated"
	if !reflect.DeepEqual(original, options.Env) {
		t.Fatal("共享环境切片")
	}
}
func TestCaptureDrainsAfterLimitAndPreservesExitError(t *testing.T) {
	call, options := helper(t, "large")
	r := Capture(t.Context(), call, options, CaptureOptions{Limit: 31})
	if r.Err != nil || len(r.Stdout) != 31 || len(r.Stderr) != 31 || !r.StdoutTruncated || !r.StderrTruncated {
		t.Fatalf("截断结果 %+v", r)
	}
	call, options = helper(t, "exit")
	r = Capture(t.Context(), call, options, CaptureOptions{})
	var exit *exec.ExitError
	if !errors.As(r.Err, &exit) || r.ExitCode != 7 || r.Status != Failed {
		t.Fatalf("失败结果 %+v", r)
	}
}
func TestPathUsesInvocationEnvironment(t *testing.T) {
	call, options := helper(t, "bytes")
	name := "helper" + filepath.Ext(call.Program)
	data, err := os.ReadFile(call.Program)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G306：临时测试目录或可执行 fixture 明确模拟项目文件权限，不保存用户数据。
	if err = os.WriteFile(filepath.Join(options.Dir, name), data, 0700); err != nil {
		t.Fatal(err)
	}
	call.Program = name
	options.Env = append(options.Env, "PATH="+options.Dir)
	if r := Capture(t.Context(), call, options, CaptureOptions{}); r.Err != nil {
		t.Fatal(r.Err)
	}
	options.Env = append(options.Env, "PATH=")
	if err := Run(t.Context(), call, options); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("空 PATH: %v", err)
	}
}
func TestCancellationRetainsExitAndCause(t *testing.T) {
	call, options := helper(t, "sleep", "1m", "never")
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("调用者取消")
	timer := time.AfterFunc(100*time.Millisecond, func() { cancel(cause) })
	defer timer.Stop()
	r := Capture(ctx, call, options, CaptureOptions{})
	var exit *exec.ExitError
	if !errors.Is(r.Err, context.Canceled) || !errors.Is(r.Err, cause) || !errors.As(r.Err, &exit) || r.Status != Canceled {
		t.Fatalf("取消错误链 %+v", r)
	}
}
func TestBatchOrderCollectAllAndTimeoutClassification(t *testing.T) {
	first, options := helper(t, "sleep", "140ms", "first")
	second := first
	second.Args = append([]string(nil), first.Args...)
	second.Args[len(second.Args)-1] = "second"
	failed := first
	failed.Args = []string{"-test.run=^TestHelperProcess$", "--", "exit"}
	items := []Invocation{first, failed, second}
	results, err := RunBatch(t.Context(), items, options, BatchOptions{Concurrency: 1, ItemTimeout: 200 * time.Millisecond})
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("整体分类 %v", err)
	}
	if string(results[0].Stdout) != "first" || results[1].ExitCode != 7 || string(results[2].Stdout) != "second" {
		t.Fatalf("顺序 %+v", results)
	}
	results, err = RunBatch(t.Context(), []Invocation{first}, options, BatchOptions{ItemTimeout: 10 * time.Millisecond})
	if err == nil || errors.Is(err, context.DeadlineExceeded) || !errors.Is(results[0].Err, context.DeadlineExceeded) {
		t.Fatalf("单项超时: %v %+v", err, results)
	}
}
func TestBatchFailFastAndParentCancellation(t *testing.T) {
	fail, options := helper(t, "exit")
	sleep := fail
	sleep.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "1m", "never"}
	results, err := RunBatch(t.Context(), []Invocation{fail, sleep, sleep}, options, BatchOptions{Concurrency: 1, FailFast: true})
	if err == nil || errors.Is(err, context.Canceled) || results[1].Status != Skipped || results[2].Status != Skipped {
		t.Fatalf("fail-fast %v %+v", err, results)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	results, err = RunBatch(ctx, []Invocation{sleep, sleep}, options, BatchOptions{})
	if !errors.Is(err, context.Canceled) || results[0].Status != Skipped || results[1].Status != Skipped {
		t.Fatalf("父取消 %v %+v", err, results)
	}
}
func TestErrDot(t *testing.T) {
	call, options := helper(t, "bytes")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, call.Program)
	if err != nil {
		t.Fatal(err)
	}
	options.Env = append(options.Env, "PATH="+filepath.Dir(relative))
	call.Program = filepath.Base(call.Program)
	if err := Run(t.Context(), call, options); !errors.Is(err, exec.ErrDot) {
		t.Fatalf("需要 ErrDot: %v", err)
	}
}
func TestInvalidExecutionDoesNotStart(t *testing.T) {
	if err := Run(t.Context(), Invocation{}, Execution{}); err == nil {
		t.Fatal("空程序应失败")
	}
	if _, err := RunBatch(t.Context(), nil, Execution{}, BatchOptions{Concurrency: -1}); err == nil || strings.Contains(err.Error(), "panic") {
		t.Fatalf("无效并发数: %v", err)
	}
}

func TestBatchConcurrencyBound(t *testing.T) {
	call, options := helper(t, "interval")
	items := make([]Invocation, 6)
	for i := range items {
		items[i] = call
	}
	results, err := RunBatch(t.Context(), items, options, BatchOptions{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	intervals := make([][2]int64, len(results))
	for i, r := range results {
		if err := json.Unmarshal(r.Stdout, &intervals[i]); err != nil {
			t.Fatal(err)
		}
	}
	maxActive := 0
	for _, period := range intervals {
		active := 0
		for _, other := range intervals {
			if other[0] <= period[0] && period[0] < other[1] {
				active++
			}
		}
		if active > maxActive {
			maxActive = active
		}
	}
	if maxActive != 2 {
		t.Fatalf("实际最大并发 %d", maxActive)
	}
}

func TestNoImplicitExecutionDeadline(t *testing.T) {
	call, options := helper(t, "sleep", "400ms", "complete")
	r := Capture(t.Context(), call, options, CaptureOptions{})
	if r.Err != nil || string(r.Stdout) != "complete" {
		t.Fatalf("默认宽限错误地限制执行时间 %+v", r)
	}
}
func TestConcurrentFailFastKeepsItemCauseOnly(t *testing.T) {
	call, options := helper(t, "exit")
	sleep := call
	sleep.Args = []string{"-test.run=^TestHelperProcess$", "--", "sleep", "1m", "never"}
	results, err := RunBatch(t.Context(), []Invocation{sleep, call, sleep}, options, BatchOptions{Concurrency: 2, FailFast: true})
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("整体错误 %v", err)
	}
	if results[0].Status != Canceled || !errors.Is(results[0].Err, context.Canceled) || results[2].Status != Skipped {
		t.Fatalf("派生取消 %+v", results)
	}
}
func TestActiveParentCancelStopsQueueAndRetainsCause(t *testing.T) {
	call, options := helper(t, "sleep", "1m", "never")
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("父调用终止")
	timer := time.AfterFunc(100*time.Millisecond, func() { cancel(cause) })
	defer timer.Stop()
	results, err := RunBatch(ctx, []Invocation{call, call, call}, options, BatchOptions{Concurrency: 1})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) || results[0].Status != Canceled || results[1].Status != Skipped || results[2].Status != Skipped {
		t.Fatalf("父取消 %v %+v", err, results)
	}
}

func TestWindowsExtensionsUseOnlyInvocationEnvironment(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "helper")
	if err := os.WriteFile(base+".tool", []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveWindowsExecutable(base, []string{"PATHEXT=exe;;tool;"})
	if err != nil || got != base+".tool" {
		t.Fatalf("本次 PATHEXT 未生效：%q %v", got, err)
	}
	if _, err = resolveWindowsExecutable(base, []string{"PATHEXT=.exe"}); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("不应使用其他扩展 %v", err)
	}
	got, err = resolveWindowsExecutable(base+".tool", []string{"PATHEXT=.exe"})
	if err != nil || got != base+".tool" {
		t.Fatalf("显式扩展被改写 %q %v", got, err)
	}
}
