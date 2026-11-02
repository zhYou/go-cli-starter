package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
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
		time.Sleep(400 * time.Millisecond)
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
func TestGovulncheckBuildInfoExact(t *testing.T) {
	valid := buildinfo.BuildInfo{Path: "golang.org/x/vuln/cmd/govulncheck", Main: debug.Module{Path: "golang.org/x/vuln", Version: "v1.9.0"}}
	require.NoError(t, matchGovulncheckBuildInfo(&valid, "1.9.0"))
	for _, mutate := range []func(*buildinfo.BuildInfo){
		func(b *buildinfo.BuildInfo) { b.Path = "example.org/other" },
		func(b *buildinfo.BuildInfo) { b.Main.Path = "example.org/other" },
		func(b *buildinfo.BuildInfo) { b.Main.Version = "v1.9.00" },
		func(b *buildinfo.BuildInfo) { b.Main.Version = "v1.9.0-rc.1" },
		func(b *buildinfo.BuildInfo) { b.Main.Replace = &debug.Module{Path: "local"} },
	} {
		b := valid
		mutate(&b)
		require.Error(t, matchGovulncheckBuildInfo(&b, "1.9.0"))
	}
	spec, err := versionTool("go:golang.org/x/vuln/cmd/govulncheck")
	require.NoError(t, err)
	require.Equal(t, []string{"-h"}, spec.Args)
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
	t.Setenv("GO_CLI_STARTER_HOME", t.TempDir())
	exe, err := os.Executable()
	require.NoError(t, err)
	calls := make([]cmdexec.Invocation, 8)
	for i := range calls {
		calls[i] = cmdexec.Invocation{Program: exe, Args: []string{"-test.run=^TestDoctorHelper$", "--", "interval"}}
	}
	results := runDoctorProbes(t.Context(), calls)
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

// doctorToolFixture 仅模拟命令边界；真实工具和 MCP 在交付验收中独立执行。
func doctorToolFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(source, []byte(`package main
import ("fmt"; "os"; "path/filepath"; "strings")
func main() {
    name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
    arg := "--version"
    if name == "gopls" { arg = "version" }
    if len(os.Args) != 2 || os.Args[1] != arg { os.Exit(2) }
    switch name {
    case "mise": fmt.Println(os.Getenv("DOCTOR_MISE_VERSION"))
    case "gopls": fmt.Println("golang.org/x/tools/gopls v0.24.0")
    default: fmt.Println(name + " fixture")
    }
}
`), 0600))
	binary := filepath.Join(dir, "probe")
	if runtime.GOOS == windowsPlatform {
		binary += executableSuffix
	}
	//nolint:gosec // G204：只构建固定的版本探测 fixture 到临时目录，不证明实际工具安装。
	output, err := exec.Command("go", "build", "-o", binary, source).CombinedOutput()
	require.NoError(t, err, "%s", output)
	return binary
}

func doctorLayerFixture(t *testing.T, binary, missing string) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.toml"), []byte("min_version = \"2026.10.21\"\n[tools]\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.agent.toml"), []byte("[tools]\n\"go:golang.org/x/tools/gopls\" = \"0.24.0\"\n"), 0600))
	bin := t.TempDir()
	for _, name := range []string{"mise", "git", "gh", "codex", "gopls"} {
		if name == missing || missing == "agent" && name != "mise" && name != "git" {
			continue
		}
		if runtime.GOOS == windowsPlatform {
			name += executableSuffix
		}
		require.NoError(t, copyFile(binary, filepath.Join(bin, name)))
	}
	t.Setenv("PATH", bin)
	t.Setenv("DOCTOR_MISE_VERSION", "2026.10.25 linux-x64")
	t.Setenv("GO_CLI_STARTER_HOME", t.TempDir())
	t.Chdir(root)
}

func TestDoctorLayerRequirements(t *testing.T) {
	binary := doctorToolFixture(t)
	t.Run("base-without-agent", func(t *testing.T) {
		doctorLayerFixture(t, binary, "agent")
		require.NoError(t, doctor(false))
	})
	t.Run("agent-without-optional-rtk", func(t *testing.T) {
		doctorLayerFixture(t, binary, "")
		require.NoError(t, doctor(true))
	})
	for _, name := range []string{"gh", "codex", "gopls"} {
		t.Run("agent-without-"+name, func(t *testing.T) {
			doctorLayerFixture(t, binary, name)
			require.ErrorContains(t, doctor(true), name)
		})
	}
	t.Run("mise-below-minimum", func(t *testing.T) {
		doctorLayerFixture(t, binary, "agent")
		t.Setenv("DOCTOR_MISE_VERSION", "2026.10.20 linux-x64")
		require.ErrorContains(t, doctor(false), "mise")
	})
}

func TestGoplsVersionMismatchSuggestsAgentSetup(t *testing.T) {
	require.ErrorContains(t, matchToolVersion("go:golang.org/x/tools/gopls", "golang.org/x/tools/gopls v0.24.1", "0.24.0"), "mise run setup:agent")
}
