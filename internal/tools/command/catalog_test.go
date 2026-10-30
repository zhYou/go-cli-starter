package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/zhYou/go-cli-starter/internal/base/fault"
	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/messages"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCommandErrorsHaveOwnedBilingualDefinitions(t *testing.T) {
	bundle, err := messages.NewBundle(Catalog())
	if err != nil {
		t.Fatal(err)
	}
	emptySpec, err := (Spec[string]{}).Build("")
	_ = emptySpec
	_, argErr := (Spec[string]{Program: "tool", Args: []Arg[string]{nil}}).Build("")
	_, batchErr := RunBatch(t.Context(), nil, Execution{}, BatchOptions{Concurrency: -1})
	cases := []struct {
		code, zh, en string
		err          error
	}{
		{"command.windows_suspend_invalid", "Windows 主线程挂起计数异常", "Unexpected suspend count on the Windows primary thread", windowsSuspendInvalid.New()},
		{"command.windows_job_not_empty", "Windows Job 终止后仍有活动进程", "Windows Job still has active processes after termination", windowsJobNotEmpty.New()},
		{"command.program_required", "必须指定执行程序", "An executable program is required", err},
		{"command.argument_required", "参数构造函数不能为空", "An argument builder is required", argErr},
		{"command.program_required", "必须指定执行程序", "An executable program is required", Run(t.Context(), Invocation{}, Execution{})},
		{"command.wait_delay_invalid", "进程收尾宽限不能为负数", "Process cleanup grace must not be negative", Run(t.Context(), Invocation{Program: "unused"}, Execution{WaitDelay: -time.Second})},
		{"command.capture_limit_invalid", "捕获上限不能为负数", "Capture limit must not be negative", Capture(t.Context(), Invocation{}, Execution{}, CaptureOptions{Limit: -1}).Err},
		{"command.batch_options_invalid", "批量并发数、超时和捕获上限不能为负数", "Batch concurrency, timeout and capture limit must not be negative", batchErr},
		{"command.batch_item_incomplete", "第 2 项执行未完成", "Batch item 2 did not complete", batchFailure(1, context.DeadlineExceeded)},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			var typed *fault.Error
			if !errors.As(tc.err, &typed) || string(typed.Code) != tc.code || typed.Kind != fault.Runtime {
				t.Fatalf("缺少所属层稳定错误定义: %v", tc.err)
			}
			if got := bundle.Localizer("zh-CN").Render(tc.err); got != tc.zh {
				t.Fatalf("中文 %q", got)
			}
			if got := bundle.Localizer("en").Render(tc.err); got != tc.en {
				t.Fatalf("英文 %q", got)
			}
		})
	}
	call, options := helper(t, "exit")
	result, err := RunBatch(t.Context(), []Invocation{call}, options, BatchOptions{})
	if err == nil || len(result) != 1 {
		t.Fatalf("真实失败未返回 %v", err)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("丢失原生错误 %v", err)
	}
	var typed *fault.Error
	if !errors.As(err, &typed) || typed.Code != "command.batch_item_failed" || typed.Params["Index"] != 1 {
		t.Fatalf("批量结构化字段 %v", err)
	}
	for language, prefix := range map[string]string{"zh-CN": "第 1 项执行失败", "en": "Batch item 1 failed"} {
		if got := bundle.Localizer(language).Render(err); !strings.HasPrefix(got, prefix) || !strings.Contains(got, "exit status 7") {
			t.Fatalf("%s: %s", language, got)
		}
	}
	if errors.Is(cases[len(cases)-1].err, context.DeadlineExceeded) {
		t.Fatal("结构化整体错误重新引入派生 deadline")
	}
}

func TestCommandLogsUseExplicitLanguageAndStableFields(t *testing.T) {
	bundle, err := messages.NewBundle(Catalog())
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"zh-CN", "en", "default"} {
		t.Run(language, func(t *testing.T) {
			var sink bytes.Buffer
			logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "debug", Output: "stderr"}, Writer: &sink}, func(err error) { t.Error(err) })
			call, options := helper(t, "bytes")
			options.Logger = logger
			if language != "default" {
				options.Localizer = bundle.Localizer(language)
			}
			result := Capture(t.Context(), call, options, CaptureOptions{})
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			lines := bytes.Split(bytes.TrimSpace(sink.Bytes()), []byte("\n"))
			if len(lines) != 2 {
				t.Fatalf("日志数量 %s", sink.Bytes())
			}
			var begin, end map[string]any
			if err := json.Unmarshal(lines[0], &begin); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(lines[1], &end); err != nil {
				t.Fatal(err)
			}
			wantBegin, wantEnd := "开始执行外部工具", "外部工具执行结束"
			if language == "en" {
				wantBegin, wantEnd = "External tool started", "External tool finished"
			}
			if begin["msg"] != wantBegin || end["msg"] != wantEnd {
				t.Fatalf("工具日志语言未生效 %s", sink.Bytes())
			}
			if end["event"] != "tool_finished" || end["operation"] != "test.helper" || end["outcome"] != "success" || end["exit_code"] != float64(0) {
				t.Fatalf("机器字段变化 %v", end)
			}
		})
	}
}
