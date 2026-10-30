package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zhYou/go-cli-starter/internal/base/logging"
	"github.com/zhYou/go-cli-starter/internal/base/telemetry"
)

func TestOnlyStartedItemsEmitSpansAndNoRawData(t *testing.T) {
	var traceOutput bytes.Buffer
	cfg := telemetry.Defaults()
	cfg.Exporter = telemetry.File
	provider := telemetry.Start(t.Context(), cfg, telemetry.Info{Service: "command-test"}, &traceOutput, func(err error) { t.Error(err) })
	t.Cleanup(func() { _ = provider.Close(t.Context()) })
	var logs bytes.Buffer
	logger := logging.New(logging.RunInfo{}, logging.Options{Config: logging.Config{Level: "info", Output: "stderr"}, Writer: &logs}, func(err error) { t.Error(err) })
	call, options := helper(t, "exit")
	call.Args = append(call.Args, "private-argv")
	options.Env = append(options.Env, "VALUE=private-env")
	options.Logger = logger
	options.Tracer = provider.Tracer
	results, err := RunBatch(t.Context(), []Invocation{call, call, call}, options, BatchOptions{Concurrency: 1, FailFast: true})
	if err == nil || results[1].Status != Skipped {
		t.Fatalf("意外结果 %v %+v", err, results)
	}
	if err := provider.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	var spans []struct {
		SpanContext struct{ TraceID, SpanID string }
		Attributes  []struct{ Value struct{ Value any } }
	}
	for _, line := range bytes.Split(bytes.TrimSpace(traceOutput.Bytes()), []byte("\n")) {
		var span struct {
			SpanContext struct{ TraceID, SpanID string }
			Attributes  []struct{ Value struct{ Value any } }
		}
		if err := json.Unmarshal(line, &span); err != nil {
			t.Fatal(err)
		}
		spans = append(spans, span)
	}
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
	if event["trace_id"] != spans[0].SpanContext.TraceID || event["span_id"] != spans[0].SpanContext.SpanID || event["event"] != "tool_finished" {
		t.Fatalf("日志与 span 不关联 %v", event)
	}
	for _, a := range spans[0].Attributes {
		if bytes.Contains([]byte(fmt.Sprint(a.Value.Value)), []byte("private-")) {
			t.Fatal("span 暴露原始参数")
		}
	}
}
