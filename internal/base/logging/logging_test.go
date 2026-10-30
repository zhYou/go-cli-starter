package logging

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type failedSink struct{}

func (failedSink) Write([]byte) (int, error) { return 0, errors.New("fixture write failure") }
func (failedSink) Sync() error               { return errors.New("fixture sync failure") }

func TestWriterFailuresReachReporterAndSyncCaller(t *testing.T) {
	warnings := 0
	logger := New(RunInfo{}, Options{Config: Defaults(), Writer: failedSink{}, RunID: "fixture"}, func(error) { warnings++ })
	logger.Infow("cannot write")
	require.Error(t, logger.Sync())
	require.Positive(t, warnings)
}

type countedSink struct {
	bytes.Buffer
	syncs int
}

func (s *countedSink) Sync() error { s.syncs++; return nil }
