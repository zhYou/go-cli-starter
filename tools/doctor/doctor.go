package main

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	key         string
	want        string
	spec        toolchain.ToolSpec
	preparation error
	optional    bool
}

func runDoctorProbes(ctx context.Context, calls []cmdexec.Invocation) []cmdexec.Result {
	//nolint:errcheck // 每项失败已保存在 results，reportDoctorProbes 分别处理必需与可选工具，不能以批量错误改变策略。
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
			if err := toolchain.Match(
				probe.key,
				string(results[i].Stdout)+"\n"+string(results[i].Stderr),
				probe.want,
			); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		version := probe.want
		if probe.key == miseProgram {
			version = ">= " + version
		}
		if _, err := fmt.Fprintf(out, "通过：%s\n", strings.TrimSpace(probe.spec.Name+" "+version)); err != nil {
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
		if key == govulncheckTool {
			path, err := exec.LookPath(spec.Name)
			if err == nil {
				var info *buildinfo.BuildInfo
				info, err = buildinfo.ReadFile(path)
				if err == nil {
					err = toolchain.MatchGovulncheckBuildInfo(info, probe.want)
				}
			}
			if err != nil {
				probe.preparation = fmt.Errorf("govulncheck 本地构建信息无效：%w", err)
			} else {
				probe.spec.Name = path
			}
		}
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
	calls := make([]cmdexec.Invocation, len(probes))
	for i, probe := range probes {
		calls[i] = cmdexec.Invocation{Program: probe.spec.Name, Args: probe.spec.Args}
	}
	results := runDoctorProbes(context.Background(), calls)
	// 展示稳定工具名；执行仍使用已核验的绝对路径，避免核验与运行的对象不同。
	for i := range probes {
		if probes[i].key == govulncheckTool {
			probes[i].spec.Name = govulncheckProgram
		}
	}
	err = reportDoctorProbes(os.Stdout, probes, results)
	fmt.Println("用户工具：superpowers，请确认用户级技能在 Codex 中可用，项目不限定安装路径。")
	fmt.Println("运行 mise run mcp:check 检查 gopls 的实际连接。")
	return err
}
