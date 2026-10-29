# CLI 工程化实施计划总览

> 面向智能体实施者：执行时必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。执行方式由用户审阅本计划后选择。

**目标：** 交付已确认设计中的十三项能力，保持派生项目可独立使用。

**架构：** 按四个子系统拆分计划，每批交付可运行、可测试的改动。
CLI 适配层保留进程语义，业务包接收 context 和有类型选项；Schema 从结果类型生成。
初始化和发布通过独立验证保障恢复与产物完整性。

**技术栈：** Go、Cobra/pflag、koanf、zap、validator、google/jsonschema-go、Afero、
Testify、go-cmp、testscript、mise、GoReleaser、govulncheck、Syft、Renovate。

**设计：** [已确认设计](../specs/2026-10-29-cli-engineering-design.md)。实施前同时阅读 README、AGENTS 和用户级 RTK 说明。

## 全局约束

- 中文用于文档、注释、帮助、日志和错误；机器标识保持原样。
- 保留 0/1/2/130 退出码及 `context.DeadlineExceeded` 映射 130 的行为。
- 当前基线：Go 工具链 1.27.3、模块最低 1.27.0；mise 最低及 CI 固定 2026.10.21；gopls 0.24.0、golangci-lint 2.15.0、GoReleaser 2.19.0。升级必须同步全部受管声明。
- 显式选项 > 非空环境变量 > YAML > 默认值；检查 `Changed`，严格拒绝未知配置键。
- 每次调用创建独立命令树和依赖；帮助、版本、补全、schema 绕过损坏配置。
- 保持 Linux、macOS、Windows 的 amd64/arm64 支持；基础任务不依赖智能体工具。
- `schema_version: 1` 仅增加可选 `result_schema`；不引入 Protobuf、Buf、ozzo 或交互流程。
- 新增应用标识和项目内部导入的受管文件必须加入初始化白名单；每批验证模板副本。
- 实施只用 snapshot 验证发布，不推送、发布或自动修改托管设置、用户配置。
- Go 修改前用 gopls 查受影响符号和引用；修改后先处理诊断，再运行针对性测试。
- 每批结束执行 `mise run fmt`、`mise run check`、`mise run test:race`；工具和发布变更分别增加 MCP 与 snapshot 验证。
- 后续状态以 [待办](../../backlog.md) 为准；任务开始改为进行中，交付后再标已完成并记录实际 PR。

## 审查重点

1. 显式 `--log-level warn` 覆盖环境中的 debug：第一批任务 4 的 `TestExplicitDefaultOverridesEnvironment`。
2. 父命令 hook 被覆盖仍正确分类业务错误：第一批任务 1 的 `TestBusinessErrorWithCustomPreRun`。
3. nil 切片、自定义 JSON 类型与 Schema 一致：第二批任务 7 的 `TestResultSchemaMatchesEncoding`。
4. 活跃初始化进程不能被另一次调用回滚：第三批任务 10 的 `TestConcurrentInitializationDoesNotRecoverLiveTransaction`。
5. 刷新快照不能掩盖机器字段类型变化：第二批任务 8 的 `TestBaselineRefreshStillRequiresChangeRecord`。

## 子计划与依赖

| 顺序 | 计划 | 任务 | 关联待办 | 可独立验收的结果 |
| --- | --- | --- | --- | --- |
| 1 | [CLI 基础](2026-10-29-cli-foundation-plan.md) | 1—6 | BL-011/012/013/014 | 错误、输出、配置、日志、校验和命令构造可复用 |
| 2 | [Schema 与进程测试](2026-10-29-cli-contract-plan.md) | 7—9 | BL-005/007/015 | 结果可发现，契约变化可审查，真实程序有测试 |
| 3 | [初始化与模板维护](2026-10-29-template-maintenance-plan.md) | 10—11 | BL-010/016 | 初始化可恢复，派生项目有来源和迁移说明 |
| 4 | [安全与发布](2026-10-29-security-release-plan.md) | 12—15 | BL-004/006/008/009 | 版本与漏洞检查、可信发布配置和协作入口齐备 |

按表顺序执行；各子计划共享的接口以拥有该接口的任务为准。
第一批完成后第二、三批可分别评审，但第四批最终验证必须包含前三批。
目前只编写计划，所有实施复选框均应保持未完成。

## 文件与职责总览

| 位置 | 职责 |
| --- | --- |
| `internal/apperror`、`internal/validation`、`internal/logging` | 错误类型、输入规则和每次调用的日志实例 |
| `internal/cli/{errors,output,dependencies,greet,version}.go` | Cobra 适配、完整渲染、命令构造与注入 |
| `internal/config`、`internal/greeting` | 严格配置和可直接调用的业务服务 |
| `internal/contract`、`internal/cli/{results,schema}.go` | 结果 Schema、命令注册和公开描述 |
| `internal/cli/testdata`、`testdata/contracts` | 真实程序场景和机器接口基线 |
| `internal/scaffold/{filesystem,transaction,origin}.go` | 文件适配、恢复事务和来源元数据 |
| `tools/dev/{versions,vulnerability,release}.go` | 工具一致性、扫描执行和 snapshot 校验 |
| `.github`、`mise.toml`、`.goreleaser.yaml` | 自动检查、锁定工具、发布证明和协作入口 |

## 依赖版本快照

2026-10-29 只读查询 Go 模块代理的稳定标签及 go.mod；下列版本均满足现有 Go 1.27.3 工具链。
实施时按拥有该依赖的任务引入，保留 go.sum，并检查选中的完整依赖图。
若届时存在更新稳定版，核验后同步本表、模块、工具锁和文档，再继续任务。

| 依赖 | 固定版本 | 用途 / 所属任务 |
| --- | --- | --- |
| `github.com/stretchr/testify` | v1.12.1 | 断言 / 1 |
| `github.com/go-playground/validator/v10` | v10.30.5 | 校验 / 3 |
| `github.com/knadh/koanf/v2` | v2.4.1 | 来源合并 / 4 |
| `github.com/knadh/koanf/providers/confmap` | v1.0.1 | 显式来源映射 / 4 |
| `go.uber.org/zap` | v1.28.0 | 日志 / 5 |
| `github.com/google/jsonschema-go` | v0.4.3 | Schema / 7 |
| `github.com/google/go-cmp` | v0.7.0 | 结构差异 / 7 |
| `github.com/rogpeppe/go-internal` | v1.16.0 | testscript / 9 |
| `github.com/spf13/afero` | v1.15.0 | 文件系统 / 10 |
| `github.com/gofrs/flock` | v0.13.1 | 跨进程文件锁 / 10 |
| `github.com/pelletier/go-toml/v2` | v2.4.3 | mise 配置解析 / 12 |
| `golang.org/x/mod` | v0.42.0 | 兼容版本、Go 模块与语义化版本解析 / 8、12 |
| `golang.org/x/vuln/cmd/govulncheck` | 模块 v1.9.0 | 独立开发工具 / 13 |
| Syft | v1.55.0 | 独立开发工具，不加入应用 go.mod / 14 |

本计划增加的 flock、go-toml 和 x/mod 是实现已确认恢复和版本检查所需的具体选型。
来源：[模块代理](https://proxy.golang.org/)、[flock](https://github.com/gofrs/flock)、
[koanf](https://github.com/knadh/koanf)、[Syft](https://github.com/anchore/syft)。

## 实施与提交约定

- 在执行阶段使用 `superpowers:using-git-worktrees` 准备隔离工作区，保持当前设计提交可追溯。
- 每项任务先建立行为测试并确认失败，再实现、检查 gopls 诊断、验证和提交。
  文档任务只做适当的文档验证，不增加镜像实现的测试。
- 依赖在首次实际使用时引入；测试依赖不进入发布程序的导入图。
- 每次提交只包含该任务的代码、测试、受管清单和必要文档，提交说明使用中文。
- 执行者按所选工作流进行任务审查及整分支审查，结尾核对十三项验收，不以单批通过代替整体完成。

## 计划自检

已逐节对照设计检查十三项覆盖、跨任务签名、五项审查重点和验证命令。
代码正文由实施者编写；本文和子计划只固定接口、行为、失败条件与验证步骤。
计划审阅通过并选定执行方式后再开始任务 1。

## 实施记录（2026-10-29）

十五项任务已按子计划完成，BL-004 至 BL-016 的实现和验证均已交付；PR 尚未创建。
独立整分支审查发现一项数据丢失风险和五项重要问题，已在一次修复中处理：

- 恢复日志必需字段与原文件存在性一致，矛盾日志在修改前拒绝。
- 特殊 JSON 编码要求显式 Schema 覆盖；业务同名字段不再被说明文案的规范化规则删除。
- 退出码变化归入破坏变更，执行版本提升要求。
- 提交、回滚终态的清理可重入；journal 最后删除，空目录残留可恢复。
- Windows 内存文件系统夹具先建立绝对临时路径。

五项行为问题均有新测试见证失败再通过，Windows 路径修复经过源码核验、Linux 回归及 Windows 测试二进制交叉编译。
完整套件通过 11 个包、127 个测试及子测试，check、race、template:check、mcp:check、Git 基准契约比较与联网扫描通过。
六目标 snapshot、对应 SPDX 和最终校验清单通过本地验证。
macOS/Windows runtime、远端来源证明及仓库设置仍未在本机验证或启用。
扫描报告可达漏洞为零；GO-2026-5932 是未调用的 openpgp 模块公告，不能声称全部依赖没有安全公告。

本轮裁量保持：机器 Task 标题供已安装脚本识别；已撤回 mimetype 升级为稳定补丁，并以构造 callback 注册校验规则；Go 后端按精确模块来源覆盖平台，不伪造归档锁。
完整 Schema 语义兼容证明、断电或恶意并发恢复保证、SPDX 全量规范及许可证合规审计均不属于本轮保证；复杂变化和目标平台配置仍需维护者审查。
