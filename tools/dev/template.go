package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"
	"github.com/zhYou/go-cli-starter/internal/base/paths"
	"github.com/zhYou/go-cli-starter/internal/dev/scaffold"
)

func verifyTemplate() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "cli-template-check-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// 利用 Git 文件列表排除本地凭据、缓存和构建产物。
	cmd := exec.Command(gitProgram, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	files, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("列出模板文件失败（请在 Git 仓库中运行）：%w", err)
	}
	for _, name := range strings.Split(string(files), "\x00") {
		if name == "" || name == ".template-init.lock" || strings.HasPrefix(name, ".template-init/") {
			continue
		}
		if err := copyFile(filepath.Join(root, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	module, name, err := scaffold.Identity(dir)
	if err != nil {
		return err
	}
	if module == scaffold.TemplateModule && name == scaffold.TemplateName {
		module, name = "example.com/template-check/mycli", "mycli"
	}
	if _, err := scaffold.Initialize(dir, module, name); err != nil {
		return err
	}
	if changed, err := scaffold.Initialize(dir, module, name); err != nil || len(changed) != 0 {
		if err != nil {
			return fmt.Errorf("初始化不满足幂等性：%w", err)
		}
		return fmt.Errorf("初始化不满足幂等性：重复调用仍改写文件")
	}
	project, err := scaffold.Inspect(dir)
	if err != nil {
		return err
	}
	if err := validateTemplateMetadata(dir); err != nil {
		return err
	}
	if err := prepareFixture(dir); err != nil {
		return err
	}
	if err := command(dir, "go", runCommand, "./tools/dev", "fmt-check"); err != nil {
		return err
	}
	// 子进程使用生成应用的前缀，原模板的 HOME 不会被改名后的程序读取。
	if err := commandEnv(dir, []string{paths.EnvPrefix(project.Name) + "_HOME=" + filepath.Join(dir, ".template-check-home")}, "go", "test", allPackages); err != nil {
		return err
	}
	if err := command(dir, "go", buildCommand, "-o", filepath.Join(dir, "app-check"), "./"+project.CommandPath); err != nil {
		return err
	}
	//nolint:gosec // G204：开发任务显式执行固定工具及拆分参数，不经过 shell；参数来自仓库或调用者。
	check := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./"+project.CommandPath)
	check.Dir = dir
	imports, err := check.Output()
	if err != nil {
		return err
	}
	if module != scaffold.TemplateModule && strings.Contains(string(imports), scaffold.TemplateModule) {
		return fmt.Errorf("生成的项目仍在导入原模板模块")
	}
	if err := command(dir, "go", runCommand, "./"+project.CommandPath, schemaCommand, greetCommand); err != nil {
		return err
	}
	if err := verifyCommandInstall(dir, project); err != nil {
		return err
	}
	if err := command(dir, releaseProgram, "check"); err != nil {
		return err
	}
	fmt.Println("模板副本检查通过：初始化、幂等性、测试、构建、命令描述和发布配置。")
	return nil
}

func prepareFixture(dir string) error {
	if err := command(dir, gitProgram, "-c", "init.templateDir=", "init", "--quiet"); err != nil {
		return err
	}
	// 此仓库只用于临时检查，不修改调用方的远端。
	// GoReleaser 的 SCM 校验需要 GitHub 格式的 origin，但不会连接该地址。
	return command(dir, gitProgram, "remote", "add", "origin", "https://github.com/example/template-check.git")
}

func copyFile(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("无法复制非常规模板文件 %s", source)
	}
	//nolint:gosec // G301：这是公开构建产物或机器契约文件的既定权限，不包含凭据。
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	//nolint:gosec // G304：开发任务只读取调用者指定的项目目录或本地构建 fixture，不作为服务接收外部路径。
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func validateTemplateMetadata(dir string) error {
	origin, err := scaffold.ReadOrigin(afero.NewOsFs(), dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	module, name, err := scaffold.Identity(dir)
	if err != nil {
		return err
	}
	if origin.Module != module || origin.Name != name {
		return fmt.Errorf("来源记录与项目标识不一致")
	}
	for _, name := range []string{"docs/template-upgrades.md", "docs/maintenance.md", "SECURITY.md", ".github/PULL_REQUEST_TEMPLATE.md", ".github/CODEOWNERS.example"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func verifyCommandInstall(root string, project scaffold.Project) error {
	installDir, err := os.MkdirTemp("", "cli-install-check-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(installDir) }()
	//nolint:gosec // G204：开发任务显式执行固定工具及拆分参数，不经过 shell；参数来自仓库或调用者。
	cmd := exec.Command("go", "install", "./"+project.CommandPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOBIN="+installDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("安装命令失败：%w：%s", err, out)
	}
	name := project.Name
	if runtime.GOOS == windowsPlatform {
		name += executableSuffix
	}
	return command(root, filepath.Join(installDir, name), schemaCommand, greetCommand)
}
