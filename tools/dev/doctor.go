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

	cmdexec "github.com/zhYou/go-cli-starter/internal/tools/command"
)

// 开发工具探测统一限制资源，不影响产品命令的默认执行策略。
const (
	doctorTimeout      = 3 * time.Second
	doctorCaptureLimit = 2 * 1024
	doctorConcurrency  = 4
)

type doctorProbe struct {
	key, want   string
	spec        toolSpec
	preparation error
}

func runDoctorProbes(ctx context.Context, calls []cmdexec.Invocation) []cmdexec.Result {
	results, _ := cmdexec.RunBatch(ctx, calls, cmdexec.Execution{}, cmdexec.BatchOptions{
		Concurrency: doctorConcurrency, ItemTimeout: doctorTimeout, Capture: cmdexec.CaptureOptions{Limit: doctorCaptureLimit},
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
		if probe.preparation != nil {
			problems = append(problems, probe.preparation)
			continue
		}
		if err := doctorProbeError(probe.spec.Name, results[i]); err != nil {
			problems = append(problems, err)
			continue
		}
		if probe.key != "" && probe.key != govulncheckTool {
			if err := matchToolVersion(probe.key, string(results[i].Stdout)+"\n"+string(results[i].Stderr), probe.want); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		if _, err := fmt.Fprintf(out, "通过：%s\n", strings.TrimSpace(probe.spec.Name+" "+probe.want)); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}
func doctor(agent bool) error {
	versions, err := loadVersions(".")
	if err != nil {
		return err
	}
	probes := []doctorProbe{{spec: toolSpec{Name: gitProgram, Args: []string{versionFlag}}}}
	for _, key := range sortedTools(versions) {
		spec, err := versionTool(key)
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
					err = matchGovulncheckBuildInfo(info, probe.want)
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
	if agent {
		optional := []cmdexec.Invocation{{Program: "codex", Args: []string{versionFlag}}, {Program: "rtk", Args: []string{versionFlag}}}
		for i, result := range runDoctorProbes(context.Background(), optional) {
			if problem := doctorProbeError(optional[i].Program, result); problem != nil {
				fmt.Printf("可选工具：%v\n", problem)
			} else {
				fmt.Printf("用户工具：%s：%s\n", optional[i].Program, result.Stdout)
			}
		}
		fmt.Println("用户工具：superpowers，请确认用户级技能在 Codex 中可用，项目不限定安装路径。")
		fmt.Println("运行 mise run mcp:check 检查 gopls 的实际连接。")
	}
	return err
}
