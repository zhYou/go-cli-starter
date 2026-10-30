// Command dev 提供跨平台的开发任务。
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zhYou/go-cli-starter/internal/dev/scaffold"
)

const appName = "go-cli-starter"

func main() {

	if err := runArgs(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

func run(task string) error { return runArgs([]string{task}) }
func runArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("必须指定开发任务")
	}
	task := args[0]
	if task == "contract-check" {
		against := ""
		//nolint:gosec // G602：上方已检查 len(args)==3，只有该分支读取这两个固定下标。
		if len(args) == 3 && args[1] == "--against" {
			//nolint:gosec // G602：上方已检查 len(args)==3，只有该分支读取这两个固定下标。
			against = args[2]
		} else if len(args) != 1 {
			return fmt.Errorf("用法：dev contract-check [--against Git引用]")
		}
		return checkContract(context.Background(), against)
	}
	if len(args) != 1 {
		return fmt.Errorf("任务 %s 不接受额外参数", task)
	}
	switch task {
	case "release-verify":
		return verifyRelease("dist")
	case "vuln":
		return checkVulnerabilities(context.Background(), nil)
	case "rules-check":
		return checkRules(context.Background())
	case "versions-check":
		return checkVersions(".")
	case "contract-update":
		return updateContract(context.Background())
	case taskBuild:
		return build()
	case "fmt":
		return format(true)
	case "fmt-check":
		return format(false)
	case "doctor":
		return doctor(false)
	case "doctor-agent":
		return doctor(true)
	case "template-check":
		return verifyTemplate()
	case "mcp-check":
		return checkMCP()
	default:
		return fmt.Errorf("未知任务 %q", task)
	}
}

func command(dir, name string, args ...string) error { return commandEnv(dir, nil, name, args...) }

func commandEnv(dir string, env []string, name string, args ...string) error {
	//nolint:gosec // G204：开发任务显式执行固定工具及拆分参数，不经过 shell；参数来自仓库或调用者。
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行 %s %s 失败：%w", name, strings.Join(args, " "), err)
	}
	return nil
}

func build() error {
	project, err := scaffold.Inspect(".")
	if err != nil {
		return err
	}
	//nolint:gosec // G301：这是公开构建产物或机器契约文件的既定权限，不包含凭据。
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return err
	}
	name := appName
	if runtime.GOOS == windowsPlatform {
		name += executableSuffix
	}
	version, commit := developmentVersion, developmentCommit
	if data, err := exec.Command(gitProgram, "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		version = strings.TrimSpace(string(data))
	}
	if data, err := exec.Command(gitProgram, gitRevParse, "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(data))
	}
	flags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.date=%s", version, commit, time.Now().UTC().Format(time.RFC3339))
	return command("", "go", buildCommand, "-trimpath", "-ldflags", flags, "-o", filepath.Join("bin", name), "./"+project.CommandPath)
}

func format(write bool) error {
	args := []string{"fmt", "--color=never"}
	if !write {
		args = append(args, "--diff")
	}
	data, err := exec.Command(lintProgram, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("golangci-lint 格式检查失败：%w\n%s", err, data)
	}
	if !write && len(data) != 0 {
		return fmt.Errorf("请运行 mise run fmt：\n%s", data)
	}
	return nil
}
