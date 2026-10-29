// Command dev 提供跨平台的开发任务。
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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
		if len(args) == 3 && args[1] == "--against" {
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
		return checkReleaseSnapshot("dist")
	case "vuln":
		return checkVulnerabilities(context.Background(), nil)
	case "versions-check":
		return checkVersions(".")
	case "contract-update":
		return updateContract(context.Background())
	case "build":
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

func command(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行 %s %s 失败：%w", name, strings.Join(args, " "), err)
	}
	return nil
}

func build() error {
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return err
	}
	name := appName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	version, commit := "dev", "none"
	if data, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		version = strings.TrimSpace(string(data))
	}
	if data, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(data))
	}
	flags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.date=%s", version, commit, time.Now().UTC().Format(time.RFC3339))
	return command("", "go", "build", "-trimpath", "-ldflags", flags, "-o", filepath.Join("bin", name), ".")
}

func format(write bool) error {
	var files []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".idea", ".superpowers", "bin", "dist", "vendor":
				return filepath.SkipDir
			}
		} else if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	flag := "-l"
	if write {
		flag = "-w"
	}
	cmd := exec.Command("gofmt", append([]string{flag}, files...)...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt 执行失败：%w\n%s", err, data)
	}
	if !write && len(data) != 0 {
		return fmt.Errorf("请运行 mise run fmt 格式化以下文件：\n%s", data)
	}
	return nil
}

func doctor(agent bool) error {
	versions, err := loadVersions(".")
	if err != nil {
		return err
	}
	var problems []error
	if _, err := exec.LookPath("git"); err != nil {
		problems = append(problems, errors.New("未找到 git"))
	} else {
		fmt.Println("通过：git")
	}
	for _, key := range sortedTools(versions) {
		spec, err := versionTool(key)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		data, err := exec.Command(spec.Name, spec.Args...).CombinedOutput()
		if err != nil {
			problems = append(problems, fmt.Errorf("%s 不可用：%w", spec.Name, err))
			continue
		}
		if err := matchToolVersion(key, string(data), versions.Tools[key]); err != nil {
			problems = append(problems, err)
			continue
		}
		fmt.Printf("通过：%s %s\n", spec.Name, versions.Tools[key])
	}

	if agent {
		for _, name := range []string{"codex", "rtk"} {
			if _, err := exec.LookPath(name); err != nil {
				fmt.Printf("可选工具：PATH 中未找到 %s\n", name)
			} else {
				data, err := exec.Command(name, "--version").CombinedOutput()
				if err != nil {
					fmt.Printf("可选工具：无法读取 %s 的版本：%v\n", name, err)
				} else {
					fmt.Printf("用户工具：%s", data)
					if len(data) == 0 || data[len(data)-1] != '\n' {
						fmt.Println()
					}
				}
			}
		}
		fmt.Println("用户工具：superpowers，请确认用户级技能在 Codex 中可用，项目不限定安装路径。")
		fmt.Println("运行 mise run mcp:check 检查 gopls 的实际连接。")
	}
	return errors.Join(problems...)
}
