package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cmdexec "github.com/zhYou/go-cli-starter/internal/tools/command"
)

func TestDoctorHelper(t *testing.T) {
	if os.Getenv("DOCTOR_HELPER") != "1" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "large":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 4096))
		_, _ = os.Stderr.WriteString(strings.Repeat("y", 4096))
	case "interval":

		start := time.Now().UnixNano()
		ready, gate := os.Args[len(os.Args)-3], os.Args[len(os.Args)-2]
		//nolint:gosec // G703：同步路径只由当前测试的临时目录生成。
		if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		//nolint:gosec // G703：释放路径只由父测试的临时 fixture 传入。
		require.Eventually(t, func() bool { _, err := os.Stat(gate); return err == nil }, 2*time.Second, time.Millisecond, "未收到探测释放信号")
		data, _ := json.Marshal([2]int64{start, time.Now().UnixNano()})
		_, _ = os.Stdout.Write(data)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "version":
		_, _ = os.Stdout.WriteString("go version go1.27.3 linux/amd64\n")
	}
	os.Exit(0)
}

func TestDoctorProbesBoundedAndOrdered(t *testing.T) {
	t.Setenv("DOCTOR_HELPER", "1")
	exe, err := os.Executable()
	require.NoError(t, err)
	calls := []cmdexec.Invocation{}
	for _, mode := range []string{"sleep", "large", "version"} {
		calls = append(calls, cmdexec.Invocation{Program: exe, Args: []string{"-test.run=^TestDoctorHelper$", "--", mode}})
	}
	started := time.Now()
	results := runDoctorProbes(t.Context(), calls)
	require.Less(t, time.Since(started), 8*time.Second)
	require.Len(t, results, 3)
	require.Equal(t, cmdexec.Canceled, results[0].Status)
	require.ErrorIs(t, results[0].Err, context.DeadlineExceeded)
	require.Len(t, results[1].Stdout, 2048)
	require.Len(t, results[1].Stderr, 2048)
	require.True(t, results[1].StdoutTruncated)
	require.True(t, results[1].StderrTruncated)
	require.Equal(t, "go version go1.27.3 linux/amd64\n", string(results[2].Stdout))
	require.Contains(t, doctorProbeError("helper", results[0]).Error(), "超时")
	require.Contains(t, doctorProbeError("helper", results[1]).Error(), "截断")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	results = runDoctorProbes(ctx, calls)
	for _, result := range results {
		require.Equal(t, cmdexec.Skipped, result.Status)
		require.Contains(t, doctorProbeError("helper", result).Error(), "取消")
	}
}

func TestDoctorSummaryStableAndExplicit(t *testing.T) {
	var out bytes.Buffer
	calls := []doctorProbe{{key: "go", want: "1.27.3", spec: toolSpec{Name: "go"}}, {key: "syft", want: "1.55.0", spec: toolSpec{Name: "syft"}}}
	results := []cmdexec.Result{{Status: cmdexec.Succeeded, Stdout: []byte("go version go1.27.3 linux/amd64\n")}, {Status: cmdexec.Succeeded, StdoutTruncated: true}}
	require.Error(t, reportDoctorProbes(&out, calls, results))
	require.Equal(t, "通过：go 1.27.3\n", out.String())
}

func TestDoctorReportsTimeoutAndTruncationTogether(t *testing.T) {
	err := doctorProbeError("helper", cmdexec.Result{Status: cmdexec.Canceled, Err: context.DeadlineExceeded, StdoutTruncated: true})
	require.ErrorContains(t, err, "超时")
	require.ErrorContains(t, err, "截断")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDoctorConcurrencyLimit(t *testing.T) {
	t.Setenv("DOCTOR_HELPER", "1")
	root := t.TempDir()
	t.Setenv("GO_CLI_STARTER_HOME", root)
	exe, err := os.Executable()
	require.NoError(t, err)
	calls := make([]cmdexec.Invocation, 8)
	for i := range calls {
		calls[i] = cmdexec.Invocation{Program: exe, Args: []string{"-test.run=^TestDoctorHelper$", "--", filepath.Join(root, fmt.Sprintf("ready-%d", i)), filepath.Join(root, fmt.Sprintf("gate-%d", i/4)), "interval"}}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	done := make(chan []cmdexec.Result, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); done <- runDoctorProbes(ctx, calls) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(4 * time.Second):
			t.Error("探测 helper 未完成收尾")
		}
	})
	for wave := 0; wave < 2; wave++ {
		for slot := 0; slot < 4; slot++ {
			ready := filepath.Join(root, fmt.Sprintf("ready-%d", wave*4+slot))
			require.Eventually(t, func() bool { _, err := os.Stat(ready); return err == nil }, 2*time.Second, time.Millisecond, "未出现探测就绪标记 %s", ready)
		}
		require.NoError(t, os.WriteFile(filepath.Join(root, fmt.Sprintf("gate-%d", wave)), nil, 0600))
	}
	var results []cmdexec.Result
	select {
	case results = <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("探测结果超时")
	}
	intervals := make([][2]int64, len(results))
	for i, result := range results {
		require.NoError(t, result.Err)
		require.NoError(t, json.Unmarshal(result.Stdout, &intervals[i]))
	}
	maxActive := 0
	for _, interval := range intervals {
		active := 0
		for _, other := range intervals {
			if other[0] <= interval[0] && interval[0] < other[1] {
				active++
			}
		}
		if active > maxActive {
			maxActive = active
		}
	}
	require.Equal(t, 4, maxActive, "真实进程区间应恰好达到四并发")
}
