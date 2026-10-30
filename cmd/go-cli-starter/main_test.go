package main

import (
	"bufio"
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"github.com/zhYou/go-cli-starter/internal/app/cli"
	"github.com/zhYou/go-cli-starter/internal/app/config"
	"github.com/zhYou/go-cli-starter/internal/app/greeting"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestSignalHelper(t *testing.T) {
	if os.Getenv("CLI_SIGNAL_HELPER") != "1" {
		return
	}
	notify := signalContext
	if runtime.GOOS == "windows" {
		notify = func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			go func() { _, _ = os.Stdin.Read(make([]byte, 1)); cancel() }()
			return ctx, cancel
		}
	}
	deps := cli.Dependencies{LoadConfig: func(config.Options) (config.Config, error) { return config.Config{LogLevel: "warn"}, nil }, Greet: func(ctx context.Context, _ greeting.Options) (greeting.Result, error) {
		_, _ = io.WriteString(os.Stderr, "就绪\n")
		<-ctx.Done()
		return greeting.Result{}, ctx.Err()
	}}
	os.Exit(run([]string{"greet", "--name", "test"}, &deps, notify))
}
func TestMainSignalCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSignalHelper$")
	cmd.Env = append(os.Environ(), "CLI_SIGNAL_HELPER=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	reader := bufio.NewReader(stderr)
	ready := make(chan string, 1)
	go func() { line, _ := reader.ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		require.Equal(t, "就绪\n", line)
	case <-ctx.Done():
		t.Fatal("等待子进程就绪超时")
	}
	if runtime.GOOS == "windows" {
		t.Log("Windows 无法发送等价 Unix 信号，使用注入 notify 的受控取消")
		_, err = stdin.Write([]byte{1})
	} else {
		err = cmd.Process.Signal(os.Interrupt)
	}
	require.NoError(t, err)
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	err = cmd.Wait()
	require.Error(t, err)
	require.Equal(t, 130, cmd.ProcessState.ExitCode())
	require.Empty(t, out.String())
	require.Contains(t, string(rest), "操作已取消")
}
