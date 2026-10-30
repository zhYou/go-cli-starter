// Package scaffold 重命名模板，不遍历或修改用户文件。
package scaffold

import (
	"context"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/afero"
)

// TemplateModule 是原始模板的模块路径。
const TemplateModule = "github.com/zhYou/go-cli-starter"

// TemplateName 是原始模板的应用标识。
const TemplateName = "go-cli-starter"

// 模板只管理这些文件中的应用标识。
// 初始化项目时保留本实现及其测试夹具，避免改动初始化基准。
var managedFiles = []string{
	".github/workflows/ci.yml", ".github/workflows/release.yml",
	"tools/dev/acceptance.go", "tools/dev/release_install.go",
	"tools/dev/doctor.go", "tools/dev/doctor_test.go", "tools/dev/rules.go", "internal/app/cli/fuzz_test.go",
	"internal/tools/command/batch.go",
	"internal/tools/command/catalog.go",
	"internal/tools/command/catalog_test.go",
	"internal/app/cli/command_embedding_test.go",
	"internal/tools/command/process.go",
	"internal/tools/command/io_windows.go",
	"internal/tools/command/observability.go",
	"internal/tools/command/observability_test.go",
	"internal/tools/command/command_test.go",
	"internal/app/cli/stderr_test.go",
	"internal/app/session/stderr_test.go",
	"internal/base/logging/external_event_test.go",
	"internal/base/telemetry/environment_linux_test.go",
	"internal/base/diagnostics/process_test.go",
	"internal/app/cli/cli_test.go",
	"internal/app/cli/session_test.go",
	"internal/app/session/session.go",
	"internal/app/session/session_test.go",
	"internal/base/diagnostics/catalog.go",
	"internal/base/logging/catalog.go",
	"internal/base/logging/logging.go",
	"internal/base/logging/new_test.go",
	"internal/base/logging/policy_test.go",
	"internal/base/telemetry/catalog.go",
	"internal/base/telemetry/session.go",
	"internal/base/telemetry/telemetry_test.go",

	"internal/app/cli/language.go", "internal/app/cli/language_test.go", "internal/app/cli/help.go", "internal/app/cli/discovery.go",
	"internal/base/messages/messages.go", "internal/base/messages/messages_test.go",
	"internal/base/paths/paths.go", "internal/base/paths/paths_test.go",

	moduleFile, "cmd/" + TemplateName + "/main.go", "cmd/" + TemplateName + "/main_test.go", "internal/app/cli/cli.go",
	"internal/app/cli/errors.go",
	"test/integration/process_test.go",
	"test/integration/testdata/process/language.txtar",
	"tools/dev/contracts_test.go",
	"tools/dev/contracts.go",
	"tools/dev/template_test.go",
	"tools/dev/mcp.go",
	"internal/app/cli/result_schema_test.go",
	"internal/app/cli/results.go",
	"internal/app/cli/dependencies.go",
	"internal/app/cli/greet.go",
	"internal/app/cli/dependencies_test.go",
	"internal/app/config/load.go",
	"internal/app/config/config_test.go",
	"internal/app/greeting/greeting.go",
	"internal/app/greeting/greeting_test.go",
	"internal/base/validation/validation.go",
	"internal/app/cli/output_test.go",
	"internal/app/cli/errors_test.go",
	"internal/base/fault/error_test.go", "tools/init/main.go",
	"tools/dev/main.go", "tools/dev/template.go", "mise.toml", ".goreleaser.yaml", "README.md", "AGENTS.md",
	"configs/config.example.toml", "docs/development.md", "docs/template-upgrades.md", "docs/design-guidelines.md",
	"docs/backlog.md",
}

var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var validModule = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

type change struct {
	path string
	data []byte
	mode os.FileMode
}

// Initialize 修改原始模板的标识；标识已匹配时不做修改。
func Initialize(root, module, name string) ([]string, error) {
	return (&Initializer{}).Initialize(context.Background(), root, module, name)
}

// Initialize 以事务方式重命名模板，并返回变更文件列表。
func (original *Initializer) Initialize(ctx context.Context, root, module, name string) ([]string, error) {
	copy := original.defaults()
	i := &copy
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateCommandName(name); err != nil {
		return nil, err
	}
	if !validModule.MatchString(module) || strings.HasSuffix(module, "/") {
		return nil, errors.New("无效的 Go 模块路径")
	}
	for _, part := range strings.Split(module, "/") {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("无效的 Go 模块路径片段")
		}
	}
	if err := i.validateModule(ctx, module); err != nil {
		return nil, err
	}
	var err error
	if _, ok := i.FS.(*afero.OsFs); ok {
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return nil, err
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	lockPath, err := i.safePath(root, ".template-init.lock")
	if err != nil {
		return nil, err
	}
	unlock, err := i.Lock(lockPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unlock() }()
	if err := i.recover(root, module, name); err != nil {
		return nil, err
	}
	currentModule, currentName, err := i.identity(root)
	if err != nil {
		return nil, err
	}
	if currentModule == module && currentName == name {
		return []string{}, nil
	}
	if currentModule != TemplateModule || currentName != TemplateName {
		return nil, errors.New("项目已初始化，拒绝替换现有标识")
	}
	replacer := strings.NewReplacer(TemplateModule, module, TemplateName, name, "GO_CLI_STARTER", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	var changes []change
	// 写入文件前先检查所有目标路径。
	for _, relative := range managedFiles {
		path, err := i.safePath(root, relative)
		if err != nil {
			return nil, err
		}
		info, err := i.FS.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("拒绝修改非常规文件 %q", relative)
		}
		data, err := afero.ReadFile(i.FS, path)
		if err != nil {
			return nil, err
		}
		updated := replaceManagedIdentity(relative, string(data), replacer)
		if updated != string(data) {
			updatedData := []byte(updated)
			if filepath.Ext(path) == ".go" {
				updatedData, err = format.Source(updatedData)
				if err != nil {
					return nil, fmt.Errorf("格式化重命名后的文件 %q 失败：%w", relative, err)
				}
			}
			changes = append(changes, change{path, updatedData, info.Mode().Perm()})
		}
	}
	origin, err := i.originChange(ctx, root, module, name)
	if err != nil {
		return nil, err
	}
	if origin != nil {
		changes = append(changes, *origin)
	}
	if len(changes) == 0 {
		return []string{}, nil
	}
	var move *commandMove
	if name != TemplateName {
		move = &commandMove{From: "cmd/" + TemplateName, To: "cmd/" + name}
	}
	return i.apply(ctx, root, module, name, changes, move)
}

// Identity 读取模块路径和可执行文件名，不执行项目代码。
func Identity(root string) (string, string, error) {
	return (&Initializer{FS: afero.NewOsFs()}).identity(root)
}
func (i *Initializer) identity(root string) (string, string, error) {
	project, err := i.inspect(root)
	return project.Module, project.Name, err
}
