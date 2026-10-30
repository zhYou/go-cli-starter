package logging

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestChildLoggersKeepParentAndSiblingFieldsIndependent(t *testing.T) {
	var output bytes.Buffer
	backend := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.Lock(zapcore.AddSync(&output)), zap.InfoLevel)).Sugar()
	parent := &Logger{sugar: backend}
	var workers sync.WaitGroup
	for _, name := range []string{"go", "git"} {
		child := parent.With("tool", name)
		workers.Go(func() { child.Record(CommandResult{ExitCode: 0}); child.Infow("diagnostic") })
	}
	workers.Wait()
	parent.Infow("parent")
	counts := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var row map[string]any
		require.NoError(t, json.Unmarshal(line, &row))
		if row["msg"] == "parent" {
			require.NotContains(t, row, "tool")
			continue
		}
		name, ok := row["tool"].(string)
		require.True(t, ok)
		counts[name]++
	}
	require.Equal(t, map[string]int{"go": 2, "git": 2}, counts)
	NewNop().Record(CommandResult{})
}
