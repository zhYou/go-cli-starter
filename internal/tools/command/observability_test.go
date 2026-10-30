package command

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/zhYou/go-cli-starter/internal/base/logging"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOnlyStartedItemsEmitSpansAndNoRawData(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	var logs bytes.Buffer
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "info", Output: "stderr"}, Writer: &logs}, func(err error) { t.Error(err) })
	call, options := helper(t, "exit")
	call.Args = append(call.Args, "private-argv")
	options.Env = append(options.Env, "VALUE=private-env")
	options.Logger = logger
	options.Tracer = provider.Tracer("command-test")
	results, err := RunBatch(t.Context(), []Invocation{call, call, call}, options, BatchOptions{Concurrency: 1, FailFast: true})
	if err == nil || results[1].Status != Skipped {
		t.Fatalf("意外结果 %v %+v", err, results)
	}
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("排队项创建了 span：%d", len(spans))
	}
	if bytes.Contains(logs.Bytes(), []byte("private-")) {
		t.Fatalf("日志暴露原始参数 %s", logs.Bytes())
	}
	var event map[string]any
	if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["trace_id"] != spans[0].SpanContext().TraceID().String() || event["span_id"] != spans[0].SpanContext().SpanID().String() || event["event"] != "tool_finished" {
		t.Fatalf("日志与 span 不关联 %v", event)
	}
	for _, a := range spans[0].Attributes() {
		if bytes.Contains([]byte(a.Value.AsString()), []byte("private-")) {
			t.Fatal("span 暴露原始参数")
		}
	}
}
