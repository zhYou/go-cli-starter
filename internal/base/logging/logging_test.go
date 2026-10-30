package logging

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

type sink struct {
	bytes.Buffer
	closed bool
}

func (s *sink) Close() error { s.closed = true; return nil }
func TestLogLevelsAndSinkOwnership(t *testing.T) {
	for _, level := range []string{"warn", "debug"} {
		s := &sink{}
		logger, err := New(s, level)
		require.NoError(t, err)
		logger.Debug("调试消息")
		logger.Warn("警告消息")
		require.NoError(t, logger.Sync())
		require.False(t, s.closed)
		require.Contains(t, s.String(), "警告消息")
		require.Equal(t, level == "debug", bytes.Contains(s.Bytes(), []byte("调试消息")))
	}
	_, err := New(&bytes.Buffer{}, "noisy")
	require.Error(t, err)
}
