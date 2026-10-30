package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONMarshalFailureLeavesOutputEmpty(t *testing.T) {
	var out bytes.Buffer
	require.Error(t, writeJSON(&out, make(chan int)))
	require.Zero(t, out.Len())
}
func TestTextRenderFailureLeavesOutputEmpty(t *testing.T) {
	var out bytes.Buffer
	require.Error(t, writeText(&out, func(w io.Writer) error { _, _ = io.WriteString(w, "局部结果"); return errors.New("渲染失败") }))
	require.Zero(t, out.Len())
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }
func TestShortWriteIsFailure(t *testing.T) {
	require.ErrorIs(t, writeJSON(shortWriter{}, true), io.ErrShortWrite)
	require.Equal(t, 1, Run(context.Background(), []string{"version", "--json"}, Options{Out: shortWriter{}}))
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("写入失败") }
func TestHelpWriteFailureIsRuntimeError(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help", "greet"}, {"version"}, {"schema"}, {"completion", "bash"}} {
		require.Equal(t, 1, Run(context.Background(), args, Options{Out: failedWriter{}}), "%v", args)
	}
}
