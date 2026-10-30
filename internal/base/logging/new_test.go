package logging

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWritesBoundFieldsAndPreservesWriterSync(t *testing.T) {
	sink := &countedSink{}
	logger := New(RunInfo{Component: "go-cli-starter", Command: "doctor", Version: "fixture"}, Options{
		Config: Defaults(), Writer: sink, RunID: "fixture-run",
	}, func(err error) { t.Error(err) })
	logger.Infow("fixture", "tool", "git")
	require.NoError(t, logger.Sync())
	require.Equal(t, 1, sink.syncs)
	var entry map[string]any
	require.NoError(t, json.Unmarshal(sink.Bytes(), &entry))
	require.Equal(t, "fixture-run", entry["run_id"])
	require.Equal(t, "go-cli-starter", entry["component"])
	require.Equal(t, "doctor", entry["command"])
	require.Equal(t, "fixture", entry["version"])
	require.Equal(t, "git", entry["tool"])
}

func TestNewWithoutWriterReturnsUsableFallback(t *testing.T) {
	warnings := 0
	logger := New(RunInfo{}, Options{Config: Defaults(), RunID: "fixture"}, func(error) { warnings++ })
	logger.Infow("discarded")
	require.NoError(t, logger.Sync())
	require.Equal(t, 1, warnings)
}
