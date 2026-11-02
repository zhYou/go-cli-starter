package main

import (
	"debug/buildinfo"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// VersionSet 保存所选工具层的固定版本和 mise 最低版本。
type VersionSet struct {
	MiseMinimum string
	Tools       map[string]string
}

func loadVersions(root string, agent bool) (VersionSet, error) {
	files := []string{miseConfig}
	if agent {
		files = append(files, "mise.agent.toml")
	}
	v := VersionSet{Tools: make(map[string]string)}
	for _, name := range files {
		//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return v, err
		}
		var cfg struct {
			Minimum string            `toml:"min_version"`
			Tools   map[string]string `toml:"tools"`
		}
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return v, err
		}
		if name == miseConfig {
			v.MiseMinimum = cfg.Minimum
		}
		for key, version := range cfg.Tools {
			if _, exists := v.Tools[key]; exists {
				return v, fmt.Errorf("工具 %s 在多个层级重复声明", key)
			}
			v.Tools[key] = version
		}
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(v.MiseMinimum) {
		return v, fmt.Errorf("mise 最低版本必须精确固定")
	}
	for key, version := range v.Tools {
		if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
			return v, fmt.Errorf("工具 %s 必须固定稳定的精确版本", key)
		}
	}
	return v, nil
}

type toolSpec struct {
	Name, Display, Pattern string
	Args                   []string
}

func versionTool(key string) (toolSpec, error) {
	switch key {
	case miseProgram:
		return toolSpec{miseProgram, miseProgram, `(?m)^([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionFlag}}, nil
	case "go":
		return toolSpec{"go", "Go", `\bgo([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionCommand}}, nil
	case goplsTool:
		return toolSpec{goplsProgram, goplsProgram, `(?m)^golang.org/x/tools/gopls v([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionCommand}}, nil
	case lintProgram:
		return toolSpec{lintProgram, lintProgram, `\bgolangci-lint has version ([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionCommand}}, nil
	case releaseProgram:
		return toolSpec{releaseProgram, "GoReleaser", `(?m)^GitVersion:\s+v?([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionFlag}}, nil
	case govulncheckTool:
		return toolSpec{govulncheckProgram, govulncheckProgram, `(?m)^Scanner:\s+govulncheck@v([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{"-h"}}, nil
	case "syft":
		return toolSpec{"syft", "Syft", `(?m)^Version:\s+v?([0-9]+\.[0-9]+\.[0-9]+)(?:[ \t\r\n]|$)`, []string{versionCommand}}, nil
	default:
		return toolSpec{}, fmt.Errorf("缺少工具 %s 的版本提取器", key)
	}
}
func matchToolVersion(key, output, want string) error {
	spec, err := versionTool(key)
	if err != nil {
		return err
	}
	match := regexp.MustCompile(spec.Pattern).FindStringSubmatch(output)
	if key == miseProgram {
		if len(match) != 2 || semver.Compare("v"+match[1], "v"+want) < 0 {
			return fmt.Errorf("mise 版本无效或低于最低版本 %s", want)
		}
		return nil
	}
	if len(match) != 2 || match[1] != want {
		setup := "setup"
		if key == goplsTool {
			setup = "setup:agent"
		}
		return fmt.Errorf("%s 与固定版本 %s 不符，请运行 mise run %s", spec.Name, want, setup)
	}
	return nil
}
func checkVersions(root string) error {
	versions, err := loadVersions(root, true)
	if err != nil {
		return err
	}
	//nolint:gosec // G304：开发工具读取显式项目文件或执行固定 Git 子命令，不经过 shell，不是外部服务入口。
	data, err := os.ReadFile(filepath.Join(root, moduleFile))
	if err != nil {
		return err
	}
	module, err := modfile.Parse(moduleFile, data, nil)
	if err != nil {
		return err
	}
	goVersion := versions.Tools["go"]
	toolchain := ""
	if module.Toolchain != nil {
		toolchain = module.Toolchain.Name
	} else if module.Go != nil {
		// 未显式声明时，Go 使用与 go 指令同版本的隐式工具链。
		toolchain = "go" + module.Go.Version
	}
	if module.Go == nil || toolchain != "go"+goVersion || semver.Compare("v"+module.Go.Version, "v"+goVersion) > 0 || semver.MajorMinor("v"+module.Go.Version) != semver.MajorMinor("v"+goVersion) {
		return fmt.Errorf("模块最低 Go 版本、工具链与 mise 不一致")
	}
	//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
	data, err = os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return err
	}
	readme := string(data)
	start := strings.Index(readme, "<!-- tool-versions:start -->")
	end := strings.Index(readme, "<!-- tool-versions:end -->")
	if start < 0 || end <= start {
		return fmt.Errorf("README 缺少当前工具版本受管表")
	}
	table := map[string]string{}
	for line := range strings.SplitSeq(readme[start:end], "\n") {
		cells := strings.Split(line, "|")
		if len(cells) == 4 {
			table[strings.TrimSpace(cells[1])] = strings.TrimSpace(cells[2])
		}
	}
	expected := map[string]string{"mise（本地最低版本、CI 固定版本）": versions.MiseMinimum}
	for key, want := range versions.Tools {
		spec, err := versionTool(key)
		if err != nil {
			return err
		}
		expected[spec.Display] = want
	}
	for key, want := range expected {
		if table[key] != want {
			return fmt.Errorf("README 工具 %s 的当前声明不一致", key)
		}
	}
	//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
	data, err = os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return err
	}
	match := regexp.MustCompile(`mise ([0-9]+\.[0-9]+\.[0-9]+) 或更新版本`).FindSubmatch(data)
	if len(match) != 2 || string(match[1]) != versions.MiseMinimum {
		return fmt.Errorf("AGENTS 的 mise 最低版本不一致")
	}
	paths, err := filepath.Glob(filepath.Join(root, ".github/workflows/*.y*ml"))
	if err != nil {
		return err
	}
	count := 0
	for _, path := range paths {
		//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string            `yaml:"uses"`
					With map[string]string `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			return err
		}
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "jdx/mise-action@") {
					count++
					if step.With[miseActionVersionField] != versions.MiseMinimum {
						return fmt.Errorf("%s 的 CI mise 版本不一致", path)
					}
				}
			}
		}
	}
	if count == 0 {
		return fmt.Errorf("未找到 CI mise 引导声明")
	}
	return nil
}
func sortedTools(v VersionSet) []string {
	keys := slices.Collect(func(yield func(string) bool) {
		for key := range v.Tools {
			if !yield(key) {
				return
			}
		}
	})
	sort.Strings(keys)
	return keys
}

// matchGovulncheckBuildInfo 离线核验真实 Go 可执行文件，避免 -version 访问漏洞库。
func matchGovulncheckBuildInfo(info *buildinfo.BuildInfo, want string) error {
	if info.Path != "golang.org/x/vuln/cmd/govulncheck" || info.Main.Path != "golang.org/x/vuln" || info.Main.Version != "v"+want || info.Main.Replace != nil {
		return fmt.Errorf("govulncheck 与固定来源和版本 %s 不符，请运行 mise run setup", want)
	}
	return nil
}
