package logging

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/stretchr/testify/require"
)

func TestConfiguredLevelRetainsFailedCommandWithoutErrorCause(t *testing.T) {
	var output bytes.Buffer
	logger := New(RunInfo{Component: "go-cli-starter", Command: "greet"}, Options{Config: Config{Level: "error"}, Writer: &output, RunID: "fixture"}, nil)
	logger.Infow("filtered")
	logger.Record(CommandResult{ExitCode: 1, Duration: time.Second})
	require.NoError(t, logger.Sync())
	b := output.Bytes()
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	require.Len(t, lines, 1)
	var e map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &e))
	require.Equal(t, "error", e["level"])
	require.Equal(t, float64(1), e["exit_code"])
}
func TestDerivedTraceContextReplacesIdentifiersWithoutDuplicateJSONKeys(t *testing.T) {
	var output bytes.Buffer
	logger := New(RunInfo{Component: "go-cli-starter", Command: "greet"}, Options{Config: Defaults(), Writer: &output, RunID: "fixture"}, nil)
	traceID, err := trace.TraceIDFromHex("11111111111111111111111111111111")
	require.NoError(t, err)
	parentID, err := trace.SpanIDFromHex("1111111111111111")
	require.NoError(t, err)
	childID, err := trace.SpanIDFromHex("2222222222222222")
	require.NoError(t, err)
	parentCtx := trace.ContextWithSpanContext(t.Context(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: parentID}))
	childCtx := trace.ContextWithSpanContext(parentCtx, trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: childID}))
	parent := logger.WithContext(parentCtx)
	parent.With("stage", "child").WithContext(childCtx).Infow("child")
	parent.Infow("parent")
	require.NoError(t, logger.Sync())
	data := output.Bytes()
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	require.Len(t, lines, 2)
	require.Equal(t, 1, bytes.Count(lines[0], []byte("\"span_id\"")))
	require.Equal(t, 1, bytes.Count(lines[0], []byte("\"trace_id\"")))
	var child, root map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &child))
	require.NoError(t, json.Unmarshal(lines[1], &root))
	require.Equal(t, "2222222222222222", child["span_id"])
	require.Equal(t, "1111111111111111", root["span_id"])
	require.NotContains(t, root, "stage")
}
