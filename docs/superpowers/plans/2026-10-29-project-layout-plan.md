# Go CLI 模板目录迁移实施计划

> 面向智能体实施者：必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。

**目标：** 完成 BL-017，让入口、配置和集成测试采用已确认的参考布局，并保留初始化、机器契约和发布能力。

**架构：** 先增加静态入口识别和可恢复的目录移动，再联动迁移主入口及使用路径。
最后移动测试与配置、保留历史契约比较，并验证原模板及改名后项目的真实构建和发布产物。

**技术栈：** 现有 Go、Cobra、afero、Testify、testscript、mise、gopls MCP、GoReleaser 和 Syft。

**设计：** [已确认的目录迁移设计](../specs/2026-10-29-project-layout-design.md)。

**状态：** 设计已确认；本计划待审阅，尚未开始代码实施。

## 全局约束

- 程序入口迁入 `cmd/<name>/`；初始化后的目录名与 `--name`、可执行文件名一致。
- 配置示例迁入 `configs/`；真实进程集成测试与共享契约数据迁入 `test/`。
- 命令、选项、JSON、schema、配置优先级、输出流及退出码保持现有行为；退出码仍为 0、1、2、130。
- 源码运行和安装入口明确变更；根目录不保留第二份入口，单元测试留在各自包中。
- 本次不新增业务命令、公开库、服务端、部署配置或依赖，不升级工具版本。
- 模板版本从 0.1.0 升到 0.2.0；事务日志格式升级到 2；`.template-origin.json` 的格式仍为 1。
- 格式 1 日志由原版本工具恢复；新工具提示并保留现场，不自动升级旧布局项目。
- 中文交流、文档、注释与项目提示；命令、选项、环境变量、机器字段及协议标识保持原样。
- 在仓库根目录通过 mise 执行 Go 命令；实施前按 `using-git-worktrees` 确认隔离工作区，并运行 setup、doctor。
- 修改符号前查询 gopls 引用；每批 Go 修改后先检查诊断，再执行针对性测试。
- 每项仅提交本任务文件；后续任务依赖前项接口，按 1 → 2 → 3 → 4 → 5 顺序执行。

## 审查重点

- 模块末段与应用名称不同：`example.com/team/repository` 配 `my-cli`，入口、安装产物和配置身份须以应用名称为准，任务 1、3。
- 只改模块而保持原命令名：不创建目录移动记录，同参数重试仍幂等，任务 3。
- 目录已移动但提交日志尚未落盘，或提交后清理中断：可分别回滚和继续清理，任务 2。
- 历史新路径含坏快照或符号链接而旧路径有效：明确失败，不能回退掩盖错误，任务 4。
- 发布归档校验和正确却缺少根目录配置、配置位于旧的父目录或缺少二进制：验证失败，任务 5。

## 文件结构与职责

| 文件或迁移 | 职责与所属任务 |
| --- | --- |
| 新建 `internal/scaffold/project.go`、`project_test.go` | 发现 `cmd/*/main.go` 与应用身份，任务 1 |
| 新建 `internal/scaffold/command_move.go`、`command_move_test.go` | 约束命令目录移动、恢复时定位文件，任务 2 |
| 修改 `internal/scaffold/{scaffold,transaction,filesystem}.go` 与现有恢复测试 | 接入格式 2、初始化移动和幂等性，任务 2、3 |
| `main.go`、`main_test.go` → `cmd/go-cli-starter/` | 唯一主入口与信号测试，任务 3 |
| 修改 `internal/scaffold/origin.go`、`origin_test.go` | 按新入口核验来源，任务 3 |
| 修改 `tools/dev/{main,template,mcp}.go` 及相关测试 | 统一使用入口发现，任务 3 |
| `internal/cli/process_test.go` → `test/integration/process_test.go` | 真实进程测试，任务 4 |
| `internal/cli/testdata/process/` → `test/integration/testdata/process/` | 跟随集成测试的脚本，任务 4 |
| `testdata/contracts/` → `test/testdata/contracts/` | 原样移动共享基线，任务 4 |
| 新建 `tools/dev/contracts_base.go`、`contracts_base_test.go` | 可靠读取迁移前后的 Git 基准，任务 4 |
| `config.example.yaml` → `configs/config.example.yaml` | 源码配置示例，任务 5 |
| 新建 `tools/dev/release_archive.go`、`release_archive_test.go` | 读取真实归档并检查必要文件，任务 5 |
| 修改 `.goreleaser.yaml`、`.template-version`、README、AGENTS 和操作指南 | 入口、归档映射及迁移说明，任务 3—5 |

本计划含当前应用路径，任务 3 将本文件加入初始化受管清单；新增 Go 文件若导入原模板模块，
在所属任务同步加入。历史设计与计划不批量改写，当前操作指南使用迁移后的路径。

### Task 1：静态识别命令入口

**文件：** 新建 `internal/scaffold/project.go`、`internal/scaffold/project_test.go`。

**接口：** 输入为仓库根目录与 `Initializer.FS`；新增
`Project{Module, Name, CommandPath string}`、`Inspect(root string) (Project, error)` 和
`(*Initializer).inspect(root string) (Project, error)`。
`CommandPath` 是使用 `/` 的相对路径，例如 `cmd/my-cli`，不含 `./`；Go 命令调用处添加 `./`。
当前 `Identity` 暂不切换，任务 3 统一接线，保持本任务独立可测试。

- [x] 写测试辅助 `projectFixture(t *testing.T, module, name string) string`，创建临时
  `go.mod` 与 `cmd/<name>/main.go`，入口含 `package main`、`appName` 常量和无参数主函数。
  写 `TestInspectCommandProject`，核心断言如下：

  ```go
  root := projectFixture(t, "example.com/team/repository", "my-cli")
  project, err := Inspect(root)
  require.NoError(t, err)
  require.Equal(t, Project{"example.com/team/repository", "my-cli", "cmd/my-cli"}, project)
  ```

- [x] 写 `TestInspectRejectsAmbiguousOrInvalidEntry` 表测试：无入口、两个合法入口、目录与
  `appName` 不符、无效名称、语法错误、非 main 包、无主函数均 `require.Error(t, err)`。
  写 `TestInspectRejectsSymlink`：真实符号链接指向外部入口时失败；系统不支持创建链接才跳过。
- [x] 运行 `mise exec -- go test ./internal/scaffold -run 'TestInspect'`，确认因入口发现接口缺失失败。
- [x] 在 `project.go` 实现上述接口，复用 `safePath` 与 `validName`，静态解析 `go.mod` 和 Go AST；
  只扫描 `cmd` 的直接子目录，不执行项目代码，不按模块末段推测名称。
  不增加根目录入口回退，也不依赖 `.template-origin.json` 存在。
- [x] gopls 诊断无错误后运行上述测试及 `mise exec -- go test ./internal/scaffold`，期望全部 PASS。
- [x] 仅暂存本任务文件，提交 `feat: 统一识别命令入口与项目身份`。

### Task 2：让命令目录移动进入恢复事务

**文件：** 新建 `internal/scaffold/command_move.go`、`command_move_test.go`；修改
`internal/scaffold/transaction.go`、`scaffold.go` 及 `transaction_test.go`、
`recovery_validation_test.go`、`cleanup_test.go`、`filesystem_test.go`。

**接口：** 新增 `commandMove{From, To string}`（均为相对目录，JSON 字段 `from`、`to`），
`transaction.Move *commandMove`（JSON 字段 `command_move,omitempty`）。
`prepare(root, module, name string, changes []change, move *commandMove) (transaction, error)` 和
`apply(ctx context.Context, root, module, name string, changes []change, move *commandMove) ([]string, error)`
均为 `Initializer` 方法。新增方法
`preflightCommandMove(root string, move *commandMove) error`、
`transactionPath(root string, tx transaction, relative string) (string, error)`。
任务 3 之前，既有初始化调用传入 nil，现有根入口测试仍应通过。

- [x] 新建测试辅助
  `commandTransactionFixture(t *testing.T) (*Initializer, string, []change, *commandMove)`：
  使用内存文件系统，创建原模板模块、`cmd/<原模板名>/main.go` 和 `main_test.go`，目标名称为
  `my-cli`，变更包含入口及模块内容。写 `TestCommandMoveTransaction`，调用 `apply` 后断言
  `afero.Exists(fs, root+"/cmd/my-cli/main.go")` 为 true、原目录不存在，权限保持，返回最终路径。
- [x] 写 `TestCommandMovePreflightProtectsUserFiles`：目标目录预存在、源目录含额外文件、
  符号链接、非普通文件时失败，断言 `require.Equal(t, before, after)` 覆盖所有原始受管字节。
  写 `TestCommandMoveRecovery`，用现有 `faultFS` 在移动前、移动已完成但尚未写提交日志时、
  持久提交后及清理中断时分别注入错误或 panic；恢复后检查目录位置、内容、权限与日志清理。
- [x] 写 `TestCommandMoveRecoveryRejectsExternalChanges`：源和目标同时存在、两者均缺失、
  人工修改（包括内容原本不需要替换的 `main_test.go`）、未知文件、目录链接、参数冲突、
  篡改移动路径均失败，并保留 journal。
  写 `TestCommandMoveCancellation`：移动前取消回滚；移动完成后检测到取消则在提交前回滚。
  写 `TestRecoveryRejectsV1Journal`：格式 1 返回包含“原版本”的中文提示，日志与目标字节不变。
- [x] 运行 `mise exec -- go test ./internal/scaffold -run 'TestCommandMove|TestRecoveryRejectsV1Journal'`，
  确认失败来自缺少目录事务能力。
- [x] 实现格式 2：预检通过后保存备份与日志；`prepare` 为命令目录全部受管文件记录摘要和
  备份，包括不需要替换内容的文件。任务 2 先将命令入口与测试路径加入受管清单，
  根入口路径由任务 3 移除。应用全部源路径内容，再移动目录，检查取消后保存提交终态。
  `transactionPath` 根据目录实际位置映射日志中的原路径；恢复不调用
  `Inspect`，以免事务中间态被身份一致性检查阻断。移动路径只允许原模板命令目录到目标名称。
  对有移动和无移动的事务分别验证目录状态；`prepared` 与回滚终态必须在源位置，
  `committed` 必须在目标位置，`applying` 允许源或目标单独存在。
- [x] 实现回滚先移回目录、再恢复内容，提交后仅清理；每次操作前校验文件摘要和目录成员。
  保留现有字段存在性、备份摘要、来源文件新增、路径限制、部分清理重试和系统锁的检查。
  返回移动后文件的绝对路径，目录移动也以目标目录绝对路径加入结果；无移动时保持现有规则。
- [x] gopls 诊断无错误后运行 `mise exec -- go test ./internal/scaffold`，期望全部 PASS；
  仅暂存本任务文件，提交 `feat: 支持初始化命令目录的可恢复改名`。

### Task 3：迁移唯一入口并接通初始化和开发任务

**文件：** 移动 `main.go`、`main_test.go` 到 `cmd/go-cli-starter/`；修改
`internal/scaffold/scaffold.go`、`origin.go`、`origin_test.go` 和所有根入口 fixture 与故障注入；
修改 `tools/dev/main.go`、`template.go`、`template_test.go`、`mcp.go`、
`internal/cli/process_test.go`、`.goreleaser.yaml`、README、AGENTS 与本计划的受管清单记录。

**接口：** 使用任务 1 的 `Project`、`Inspect`、`Initializer.inspect` 与任务 2 的
`commandMove`、`apply`。保留 `Identity(root string) (string, string, error)` 和
`Initializer.identity` 的签名，改为委托入口发现。
新增 `verifyCommandInstall(root string, project scaffold.Project) error` 于 `tools/dev/template.go`。

- [ ] 写 `TestInitializeMovesCommandDirectory`：模块 `example.com/team/repository`、名称
  `my-cli`，初始化后 `Inspect` 返回目标身份，`cmd/my-cli` 存在且原目录消失，导入与内容已替换。
  写 `TestInitializeSameNameDoesNotMove`：仅改模块，目录保持，重复调用变更为空。
  更新原有幂等性、来源字节稳定、不同身份拒绝、权限和锁测试的 fixture 到命令目录布局。
- [ ] 运行 `mise exec -- go test ./internal/scaffold -run 'TestInitializeMoves|TestInitializeSameName'`，
  确认当前初始化尚未接通新入口和目录移动。
- [ ] 接通 `Identity` 与 `Initialize`：恢复仍早于身份读取；生成受管内容变更及可选移动后
  统一调用 `apply`。把入口文件白名单改为原模板命令路径，用原模板常量构造路径；
  同名初始化不创建移动记录。旧根布局缺少命令入口时给出明确中文迁移提示。
- [ ] 移动实际入口及信号测试，同步 fixture 中的 `main.go` 路径和故障拦截点。
  来源核验的 `HEAD:main.go` 改为原模板命令路径；写 `TestOriginUsesCommandEntry`，
  用临时 Git 仓库确认可信模板提交被记录，派生仓库身份或来源不符时仍为 `unknown`。
- [ ] 将 `build`、模板中的 build/list/schema、MCP 诊断和进程测试的构建目标改为
  `"./" + project.CommandPath`；GoReleaser `main` 改为 `./cmd/go-cli-starter`。
  在模板副本初始化后重新 `Inspect`，不能继续使用初始化前路径。
- [ ] 实现 `verifyCommandInstall`：独立临时目录作为该子进程的 `GOBIN`，执行
  `go install ./cmd/<name>`，验证 `<name>`（Windows 为 `<name>.exe`）存在并运行其 schema。
  从 `verifyTemplate` 调用此函数；更新模板构建 fixture 与元数据测试。README 使用新源码
  运行和安装入口，AGENTS 改为“程序入口”，受管清单增加本计划及新增应用标识引用。
- [ ] gopls 诊断无错误后运行 `mise exec -- go test ./internal/scaffold ./tools/dev ./internal/cli ./cmd/go-cli-starter`、
  `mise run build`、`mise run template:check`、`mise run mcp:check`；期望全部退出 0。
  仅暂存本任务文件，提交 `refactor: 将程序入口迁入 cmd 并统一路径使用`。

### Task 4：集中集成测试与契约数据，保留历史比较

**文件：** 移动 `internal/cli/process_test.go` 及其脚本到 `test/integration/`；
移动 `testdata/contracts/{initial,v1}.json` 到 `test/testdata/contracts/`；
修改 `tools/dev/contracts.go`、`contracts_test.go`、`internal/scaffold/scaffold.go`、
`docs/compatibility.md`；新建 `tools/dev/contracts_base.go`、`contracts_base_test.go`。

**接口：** 使用任务 1 的 `Inspect`。新增
`readContractBase(ctx context.Context, ref string) ([]byte, bool, error)`，第二返回值表示
两个历史路径均不存在、使用本地 initial 基线。常量 `contractFile` 为
`test/testdata/contracts/v1.json`，`initialContractFile` 为
`test/testdata/contracts/initial.json`，`legacyContractFile` 为 `testdata/contracts/v1.json`。

- [ ] 写 `TestReadContractBaseAcrossLayoutMigration`：临时 Git 仓库分别提交仅旧路径、
  仅新路径、两者同时存在且内容不同、两者均缺失四类基准；断言返回旧/新/新/initial 字节，
  只有最后一种 `require.True(t, initial)`，其余 `require.False(t, initial)`。
  fixture 使用临时仓库的本地 `git -c user.name=... -c user.email=... commit`，不改用户配置。
- [ ] 写 `TestReadContractBaseRejectsInvalidHistory`：无效 ref、以 `-` 起始的 ref、
  非 Git 目录、已取消 context 导致 Git 执行失败、快照为目录或符号链接均返回错误。写
  `TestInvalidNewContractDoesNotFallback`：新路径内容损坏或类型不合法，旧路径有效，
  `checkContract` 仍失败。测试用 `t.Chdir` 隔离当前目录，不调用 `t.Parallel`。
- [ ] 运行 `mise exec -- go test ./tools/dev -run 'TestReadContractBase|TestInvalidNewContract'`，
  确认历史路径读取能力缺失。
- [ ] 实现 `readContractBase`：先把 ref 解析为确定的 commit，再用 `git ls-tree -z` 判断
  两个精确路径的存在性和普通文件类型，优先新路径；按确定的 SHA 读取 blob。
  任何 Git 故障都返回错误；内容损坏交由契约解析失败，不尝试旧路径。
  接入 `checkContract`，保留现有变更记录过滤与校验；`contract:update` 仅写新路径。
- [ ] 原样移动两份快照；将进程测试改为 `package integration`，保留任务 3 的动态入口构建，
  脚本内容与退出码断言保持原样。更新受管清单的进程测试路径和当前兼容性文档。
- [ ] gopls 诊断无错误后运行 `mise exec -- go test ./tools/dev ./internal/contract ./test/integration`、
  `mise run contract:check`、`mise run contract:check -- --against 5d03813`；期望全部退出 0。
  比较迁移前后两份快照的字节，确认没有刷新内容。
- [ ] 仅暂存本任务文件，提交 `refactor: 集中集成测试并保留历史契约比较`。

### Task 5：迁移配置、验证发布归档并完成文档

**文件：** 移动 `config.example.yaml` 到 `configs/config.example.yaml`；修改
`.goreleaser.yaml`、`.template-version`、`tools/dev/release.go`、`release_test.go`、
`internal/scaffold/scaffold.go`、README、`docs/development.md`、`docs/template-upgrades.md`、
`docs/backlog.md`；新建 `tools/dev/release_archive.go`、`release_archive_test.go`。

**接口：** 保留 `checkReleaseSnapshot(dist string) error`；新增
`validateArchive(path, binary string) error`，支持当前 tar.gz/zip，检查根目录的二进制、
`README.md` 与非空 `config.example.yaml`。Windows 传入含 `.exe` 的二进制名。

- [ ] 把 `releaseFixture` 改为生成真实 tar.gz/zip，保留现有 SBOM 与摘要构造。
  写 `TestReleaseArchiveContents`：六平台正常归档通过；缺二进制、缺 README、缺配置、
  配置为空、配置仅放在 `configs/`、重复必要文件或损坏归档均失败。
  通过 `checkReleaseSnapshot` 的坏用例重新计算校验和，证明失败来自归档内容检查。
- [ ] 运行 `mise exec -- go test ./tools/dev -run 'TestReleaseArchive|TestReleaseSnapshot'`，
  确认当前校验不能发现缺失的归档内容。
- [ ] 用标准库 `archive/tar`、`archive/zip` 与 `compress/gzip` 实现 `validateArchive`，
  只读取归档，不解压到磁盘；在现有摘要和目标格式检查后调用。
  必要条目须为根目录普通文件并唯一，保留现有六目标、版本、SPDX 与 SHA-256 检查。
- [ ] 移动配置，更新受管清单；GoReleaser 映射使用
  `src: configs/config.example.yaml`、`dst: .`、`strip_parent: true`，README 从新源码路径
  介绍复制配置，发布包示例继续使用包根目录文件。单元测试不为纯文件移动增加快照。
- [ ] 模板版本改为 `0.2.0`，更新 README 目录图及开发说明，增加 0.2.0 人工迁移步骤，
  明确新源码入口、旧事务先由原工具恢复、初始化不等于升级。应用发布版本和机器快照保持原值。
- [ ] gopls 诊断无错误后依次执行 `mise run fmt`、`mise run check`、`mise run test:race`、
  `mise run mcp:check`、`codex mcp list`、`mise run release:snapshot`、`mise run release:verify`。
  `check` 已包含 `template:check`，通过后不重复；确认 MCP 列表包含 gopls。
  解读六目标真实归档，比较包根目录配置与 `configs/config.example.yaml` 字节一致。
- [ ] 运行 `mise run contract:check -- --against 5d03813` 和 `git diff --check`，检查当前代码与
  历史基准兼容，当前操作文档没有遗留根入口用法；历史设计、计划和迁移说明中的旧路径保留。
  记录实际 Linux 验证结果，macOS/Windows 运行情况以对应 CI 为准，不声称本机已验证。
- [ ] 更新 BL-017 状态、日期、验证和实际 PR；没有 PR 时仍写“尚未关联”。
  完成独立代码审查并处理影响验收的问题后，仅暂存本任务文件，提交
  `refactor: 迁移配置并验证参考布局的完整交付`；按分支收尾技能处理集成。

## 计划自查

- 设计中的目录、源码入口、入口发现由任务 1、3、4、5 覆盖。
- 初始化目录移动、格式 2、旧日志提示和失败恢复由任务 2、3 覆盖。
- 历史契约比较由任务 4 覆盖；配置、六目标发布内容和模板升级由任务 5 覆盖。
- 五项审查重点均有明确的测试任务；跨任务使用的类型和函数签名已统一。
- 当前只编写和审阅计划，所有实施复选框保持未勾选；执行方式确定后再开始代码修改。
