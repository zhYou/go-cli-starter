package main

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhYou/go-cli-starter/tools/internal/toolchain"

	cmdexec "github.com/zhYou/go-cli-starter/internal/command"
)

// 开发工具探测统一限制资源，不影响产品命令的默认执行策略。
const (
	doctorTimeout      = 3 * time.Second
	doctorCaptureLimit = 2 * 1024
	doctorConcurrency  = 4
)

type doctorProbe struct {
	key, want   string
	program     string
	spec        toolchain.ToolSpec
	preparation error
	optional    bool
}

func runDoctorProbes(ctx context.Context, calls []cmdexec.Invocation) []cmdexec.Result {
	results, _ := cmdexec.RunBatch(ctx, calls, cmdexec.Execution{}, cmdexec.BatchOptions{
		Concurrency: doctorConcurrency,
		ItemTimeout: doctorTimeout,
		Capture:     cmdexec.CaptureOptions{Limit: doctorCaptureLimit},
	})
	return results
}

func doctorProbeError(name string, result cmdexec.Result) error {
	var problems []error
	if result.StdoutTruncated || result.StderrTruncated {
		problems = append(problems, fmt.Errorf("%s 探测输出已截断（每个流最多 %d 字节）", name, doctorCaptureLimit))
	}
	switch {
	case errors.Is(result.Err, context.DeadlineExceeded):
		problems = append(problems, fmt.Errorf("%s 探测超时（%s）：%w", name, doctorTimeout, result.Err))
	case result.Status == cmdexec.Canceled || result.Status == cmdexec.Skipped:
		problems = append(problems, fmt.Errorf("%s 探测已取消：%w", name, result.Err))
	case result.Err != nil:
		problems = append(problems, fmt.Errorf("%s 不可用：%w", name, result.Err))
	}
	return errors.Join(problems...)
}

func reportDoctorProbes(out io.Writer, probes []doctorProbe, results []cmdexec.Result) error {
	var problems []error
	for i, probe := range probes {
		if probe.optional {
			detail := strings.TrimSpace(string(results[i].Stdout) + "\n" + string(results[i].Stderr))
			if err := doctorProbeError(probe.spec.Name, results[i]); err != nil {
				detail = err.Error()
			}
			if _, err := fmt.Fprintf(out, "用户工具：%s：%s\n", probe.spec.Name, detail); err != nil {
				problems = append(problems, err)
			}
			continue
		}
		if probe.preparation != nil {
			problems = append(problems, probe.preparation)
			continue
		}
		if err := doctorProbeError(probe.spec.Name, results[i]); err != nil {
			problems = append(problems, err)
			continue
		}
		if probe.key != "" && probe.key != govulncheckTool {
			if err := toolchain.Match(probe.key, string(results[i].Stdout)+"\n"+string(results[i].Stderr), probe.want); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		name := probe.spec.Name
		if name == miseProgram && probe.key != miseProgram {
			name = probe.spec.Display
		}
		version := probe.want
		if probe.key == miseProgram {
			version = ">= " + version
		}
		if _, err := fmt.Fprintf(out, "通过：%s\n", strings.TrimSpace(name+" "+version)); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func doctor() error {
	versions, err := toolchain.Load(".")
	if err != nil {
		return err
	}
	miseSpec, err := toolchain.Spec(miseProgram)
	if err != nil {
		return err
	}
	probes := []doctorProbe{{key: miseProgram, want: versions.MiseMinimum, spec: miseSpec}, {spec: toolchain.ToolSpec{
		Name: gitProgram,
		Args: []string{versionFlag},
	}}}
	for _, key := range toolchain.SortedTools(versions) {
		spec, err := toolchain.Spec(key)
		if err != nil {
			return err
		}
		probe := doctorProbe{key: key, want: versions.Tools[key], spec: spec}

		probes = append(probes, probe)
	}
	for _, name := range []string{"gh", "codex", "rtk"} {
		probes = append(probes, doctorProbe{
			spec: toolchain.ToolSpec{
				Name: name,
				Args: []string{versionFlag},
			},
			optional: true,
		})
	}
	prepareDoctorProbes(probes)
	calls := make([]cmdexec.Invocation, len(probes))
	for i, probe := range probes {
		if probe.preparation != nil {
			continue
		}
		program := probe.program
		if program == "" {
			program = probe.spec.Name
		}
		calls[i] = cmdexec.Invocation{Program: program, Args: probe.spec.Args}
	}
	results := runDoctorProbes(context.Background(), calls)
	err = reportDoctorProbes(os.Stdout, probes, results)
	fmt.Println("用户工具：superpowers，请确认用户级技能在 Codex 中可用，项目不限定安装路径。")
	fmt.Println("运行 mise run mcp:check 检查 gopls 的实际连接。")
	return err
}

// prepareDoctorProbes 通过 mise 取得纳管工具的绝对路径，避免父进程 PATH 中的旧工具遮蔽固定版本。
func prepareDoctorProbes(probes []doctorProbe) {
	var indices []int
	var calls []cmdexec.Invocation
	prettierIndex := -1
	for i, probe := range probes {
		if probe.key == "" || probe.key == miseProgram || probe.spec.Name == miseProgram {
			continue
		}
		indices = append(indices, i)
		program := probe.spec.Name
		if probe.key == "npm:prettier" {
			program = "node"
			prettierIndex = i
		}
		calls = append(calls, cmdexec.Invocation{Program: miseProgram, Args: []string{"which", program}})
	}
	prefixIndex := len(calls)
	if prettierIndex >= 0 {
		calls = append(calls, cmdexec.Invocation{Program: miseProgram, Args: []string{"where", "npm:prettier"}})
	}
	results := runDoctorProbes(context.Background(), calls)
	for n, i := range indices {
		probe := &probes[i]
		if err := doctorProbeError(probe.spec.Name, results[n]); err != nil {
			probe.preparation = err
			continue
		}
		path := strings.TrimSpace(string(results[n].Stdout))
		if !filepath.IsAbs(path) {
			probe.preparation = fmt.Errorf("%s 的 mise 安装路径无效", probe.spec.Name)
			continue
		}
		probe.program = path
		if i == prettierIndex {
			if err := doctorProbeError("Prettier", results[prefixIndex]); err != nil {
				probe.preparation = err
				continue
			}
			entry, err := prettierEntry(strings.TrimSpace(string(results[prefixIndex].Stdout)))
			if err != nil {
				probe.preparation = err
				continue
			}
			probe.spec.Args = append([]string{entry}, probe.spec.Args...)
		}
		if probe.key != govulncheckTool {
			continue
		}
		info, err := buildinfo.ReadFile(path)
		if err == nil {
			err = toolchain.MatchGovulncheckBuildInfo(info, probe.want)
		}
		if err != nil {
			probe.preparation = fmt.Errorf("govulncheck 本地构建信息无效：%w", err)
		}
	}
}

// prettierEntry 定位已安装的 JS 入口，通过 Node 运行，避免 Windows 的 cmd 包装脚本。
func prettierEntry(prefix string) (string, error) {
	if !filepath.IsAbs(prefix) {
		return "", fmt.Errorf("已安装的 Prettier 路径无效")
	}
	for _, base := range []string{prefix, filepath.Join(prefix, "lib")} {
		path := filepath.Join(base, "node_modules", "prettier", "bin", "prettier.cjs")
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("已安装的 Prettier 入口不是普通文件: %s", path)
		}
		return path, nil
	}
	return "", fmt.Errorf("未找到 Prettier 的 JS 入口，请运行 mise run setup")
}
