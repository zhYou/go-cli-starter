package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEntryRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"extra"}} {
		require.ErrorContains(t, run(args), "mise run mcp:check")
	}
}

func TestRunMCPChecksRunsConcurrentlyAndJoinsErrors(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	checks := []mcpCheck{
		{
			name: "first",
			run: func(context.Context, string) error {
				started <- struct{}{}
				<-release
				return errors.New("first failed")
			},
		},
		{
			name: "second",
			run: func(context.Context, string) error {
				started <- struct{}{}
				<-release
				return errors.New("second failed")
			},
		},
	}
	done := make(chan error, 1)
	ctx := t.Context()
	root := t.TempDir()
	go func() {
		done <- runMCPChecks(ctx, root, checks)
	}()
	for range checks {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("MCP 验收未并行启动")
		}
	}
	close(release)
	err := <-done
	require.ErrorContains(t, err, "验收 first：first failed")
	require.ErrorContains(t, err, "验收 second：second failed")
}
