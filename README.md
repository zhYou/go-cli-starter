# go-cli-starter

可直接复制复用的 Go CLI 项目模板。主要面向智能体调用，兼顾手动使用。开发和维护由智能体执行，维护者负责需求、合并和发布决策。

默认输出文本，通过 `--json` 输出结构化结果。包含命令发现、配置、日志、测试、本地环境、项目改名和跨平台发布。

项目交流和文档使用中文。CLI 默认 zh-CN，可显式选择 en，不随系统区域改变；命令名、选项名、机器字段和退出码保持稳定。设计取舍与扩展示例见[设计指南](docs/design-guidelines.md)。

## 创建新项目

在 GitHub 使用 **Use this template** 创建仓库并克隆，或直接复制项目文件后执行 `git init`。准备好 Git 和 mise 2026.10.21 或更新的稳定版，然后在新项目根目录执行：

```sh
mise trust
mise run setup
mise run init --module github.com/you/mycli --name mycli
mise run check
```

初始化更新模块路径、Go 导入、命令名、环境变量前缀和构建发布配置。同参数重复执行不改动文件，已初始化项目拒绝不同身份覆盖。新项目可以独立构建、测试和发布。

初始化仅改写模板管理的文件，不修改 Git remote、提交历史和用户级智能体配置。命令名使用小写字母、数字和连字符，并以字母开头。`v2`、`v10` 等无前导零且至少为 2 的版本段名称会改变 Go 的安装产物名，因此初始化会提前拒绝；`v0`、`v1`、`v02` 不受此限制。

## 本地环境

[安装 mise](https://mise.jdx.dev/getting-started.html)。已有 mise 按其安装方式升级；独立安装的版本可运行 `mise self-update`。

<!-- tool-versions:start -->

| 工具 | 固定版本 |
| --- | --- |
| mise（本地最低版本、CI 固定版本） | 2026.10.21 |
| Go | 1.27.3 |
| Go 模块最低版本 | 1.27.0 |
| gopls | 0.24.0 |
| golangci-lint | 2.15.0 |
| GoReleaser | 2.19.0 |
| govulncheck | 1.9.0 |
| Syft | 1.55.0 |

<!-- tool-versions:end -->

有官方 LTS 时使用最新 LTS 补丁版，否则使用最新稳定版；核验后锁定具体版本。本地和 CI 读取相同配置。Go 没有单独的 LTS 渠道，模板采用 1.27 系列，模块最低版本为 1.27.0。

首次使用及 mise/模块锁变更后运行 `setup`：只安装项目锁定工具并下载模块，不安装用户全局配置中的其他工具。普通任务和 exec 均关闭自动安装，`GOTOOLCHAIN=local` 防止隐式下载；日常检查不会安装、升级或修复环境。

```sh
mise run doctor
mise run build
GO_CLI_STARTER_HOME="$(mktemp -d)" mise exec -- go run ./cmd/go-cli-starter greet --name Codex
GO_CLI_STARTER_HOME="$(mktemp -d)" mise exec -- go run ./cmd/go-cli-starter greet --name Codex --json
```

发布产物本身不依赖 mise 或 Go。源码安装入口是 `cmd/go-cli-starter`：

```sh
go install github.com/zhYou/go-cli-starter/cmd/go-cli-starter@latest
```

## 给智能体调用

```sh
./bin/go-cli-starter --help
./bin/go-cli-starter schema
./bin/go-cli-starter schema greet
GO_CLI_STARTER_HOME="$(mktemp -d)" ./bin/go-cli-starter greet --name Codex --json
./bin/go-cli-starter version --json
```

示例结果：

```json
{"message":"你好，Codex！"}
```

- 默认文本；`--json` 不随终端环境改变，也不由配置或环境变量隐式开启。
- 成功结果写 stdout，错误和诊断告警写 stderr；日志默认写应用私有状态目录，显式配置后可写 stderr。执行或序列化失败时 stdout 为空，错误输出为文本。底层写入失败可能留下部分字节，消费方须先检查退出码。
- 缺少必需参数直接失败。
- `schema [命令路径...]` 总是输出 JSON，包含命令树、用法、示例、参数类型、必填标记、默认值和退出码。greet/version 的 result_schema 公开 Draft 2020-12 结果结构。消费方应忽略新增字段；配置文件仍拒绝未知键。
- 帮助输出文本，补全输出 shell 脚本；它们不受 `--json` 影响。
- 检查退出状态后再解析 JSON。测试真实退出码时使用构建后的程序；`go run` 会包装非零退出码。

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 运行或配置错误 |
| 2 | 命令或参数错误 |
| 130 | 信号或 context 取消 |

## 配置、语言和诊断

配置只读取一个 `config.toml`，示例在 `configs/config.example.toml`。`--config` 指定任意显式文件，
否则读取应用私有配置目录；默认文件不存在照常运行，显式文件不存在、未知键、类型或范围错误则失败。
不自动读取或合并 YAML，旧版本迁移见[模板升级](docs/template-upgrades.md)。

```toml
[logging]
level = "info"
output = "file" # file、stderr、none

[telemetry]
exporter = "none" # none、file
```

优先级为显式 flag > 非空环境变量 > TOML > 默认值。模块键映射为大写前缀环境变量，
例如 `GO_CLI_STARTER_LOGGING_LEVEL`、`GO_CLI_STARTER_LOGGING_OUTPUT`、
`GO_CLI_STARTER_TELEMETRY_EXPORTER`；兼容入口 `GO_CLI_STARTER_LOG_LEVEL` 优先于新的级别变量，
`--log-level` 优先于两者。没有给出的 flag 不覆盖文件或环境值。

语言独立于配置：`--lang en` > `GO_CLI_STARTER_LANG=en` > zh-CN；zh 是中文别名。
帮助、schema 描述、结果正文、错误和日志消息使用所选语言，第三方诊断可保留原文。
帮助、version、completion 和 schema 不读取配置、不创建诊断目录，损坏配置下仍可发现命令。

| 平台 | 配置目录 | 数据 / 状态 / 缓存 |
| --- | --- | --- |
| Linux | `$XDG_CONFIG_HOME/go-cli-starter`，默认 `~/.config/go-cli-starter` | 对应 XDG 根；默认 `~/.local/share`、`~/.local/state`、`~/.cache` 下的应用目录 |
| macOS | `~/Library/Application Support/go-cli-starter` | 配置目录下 data、state；`~/Library/Caches/go-cli-starter` |
| Windows | `%APPDATA%/go-cli-starter` | `%LOCALAPPDATA%/go-cli-starter` 下 data、state、cache |

配置文件名均为 `config.toml`。`GO_CLI_STARTER_HOME` 可指定绝对私有根：配置文件位于根下，
数据、状态、缓存位于 data、state、cache；改名为 mycli 后变量是 `MYCLI_HOME`。
此目录与 mise 的安装位置无关。相对应用 HOME 会报错；Linux 相对 XDG 变量回退到 HOME。

业务命令默认将结构化日志写入 `<state>/logs/<run_id>/events.jsonl`。Trace 默认关闭，file 写同次运行的
`trace.jsonl`，内容是本地诊断 JSON 流，不承诺 OTLP 兼容或重放。日志出口与 Trace 出口分别配置。
默认分片软阈值 10 MiB，单条超大记录不拆分；5 个备份、保留 14 天、软总量 200 MiB，后三项可设 0 关闭对应限制。
清理只处理元数据有效且成功取得系统锁的非活跃运行，未知内容保留；活跃日志可能超过软限额。
诊断创建、写入或收尾失败会向 stderr 告警并继续业务。不要在日志中写密钥、完整参数、环境或工具原始输出。

以下 POSIX shell 示例隔离配置和状态，适合验证文档：

```sh
example_home=$(mktemp -d)
cp configs/config.example.toml "$example_home/config.toml"
GO_CLI_STARTER_HOME="$example_home" ./bin/go-cli-starter greet --name Codex --lang en --json
```

## Codex 开发

先准备环境，再从项目根目录启动 Codex：

```sh
mise run setup
mise run doctor:agent
codex
```

`AGENTS.md` 记录仓库约定和验证命令。`.codex/config.toml` 为受信任项目配置 gopls MCP，通过 mise 选择固定版本，启动失败不会阻塞 Codex。验证连接：

```sh
mise run mcp:check
codex mcp list
```

`mcp:check` 真正完成初始化、发现工具，并对当前项目运行 Go 诊断。Codex 需重新启动或重新连接，才能加载新增配置。

RTK 和 superpowers 使用用户目录中的安装。模板不复制全局技能，不写个人路径、模型、认证或权限偏好。`doctor:agent` 报告辅助工具状态；superpowers 是否加载，以 Codex 中的技能列表为准。缺少辅助工具不影响基础检查。

当前仅支持 Codex。Claude Code 和 OpenCode 支持见[项目待办](https://github.com/zhYou/go-cli-starter/issues?q=is%3Aissue+state%3Aopen+label%3Abacklog)；TUI 和 CLI 自身的 MCP 服务暂不在范围内。

隔离 worktree、分支命名、提交署名和 PR 交付规则见 [协作约定](AGENTS.md#隔离开发与交付)。

## 开发命令

| 命令 | 用途 |
| --- | --- |
| `mise run setup` | 安装锁定工具，下载模块 |
| `mise run doctor` | 检查工具和版本 |
| `mise run fmt` / `fmt:check` | gofmt + goimports；后者只读检查差异 |
| `mise run test` | 单元与命令测试 |
| `mise run test:race` | 竞态检测，需要 C 编译器 |
| `mise run lint` / `rules:check` | 18 项静态检查及真实类型边界 fixture |
| `mise run test:fuzz` / `test:bench` | 有界 argv 模糊测试 / 开销测量，无性能门槛 |
| `mise run build` | 构建到 bin，注入版本信息 |
| `mise run check` | 与 CI 相同的完整源码检查 |
| `mise run template:check` | 在临时副本中验证改名、测试、构建和发布配置 |
| `mise run release:check` | 验证当前仓库的发布配置和 SCM 信息，需要 remote |
| `mise run release:snapshot` | 本地构建全部发布压缩包，不发布 |

目录按入口、内部包、配置、测试和工具分工：

```text
cmd/<name>/                 程序入口与信号测试
internal/app/               CLI、配置与业务逻辑
internal/base/              错误、消息、路径、校验、IO、日志、Trace、诊断、schema
internal/tools/command/     外部命令与有界并行
internal/dev/               模板初始化与机器契约比较
configs/config.example.toml 配置示例
test/integration/           真实进程测试与脚本
test/testdata/contracts/    机器契约基线
tools/                     初始化与开发任务
docs/                      使用和维护说明
```

`cmd/<name>` 入口负责信号和退出；`internal/app/cli` 组织命令，`internal/app/config` 加载配置，`internal/app/greeting` 演示业务逻辑；`internal/base` 提供不依赖应用、开发工具或 Cobra 的公共能力；`internal/base/schema` 负责运行时结果结构，`internal/dev/contract` 负责开发期快照比较。`internal/dev/scaffold` 和根目录 `tools` 提供初始化与开发任务。

初始化后的命令目录随 `--name` 改名；模板版本为 0.3.0，已有项目迁移见[模板升级](docs/template-upgrades.md)。发布包仍在根目录提供 `config.example.toml`。

`check` 在临时仓库中验证发布配置，无需为内部工具配置 remote。真正发布或运行 `release:check` 前，为目标项目设置自己的 GitHub origin。

新增命令、工具升级和发布步骤见 [开发说明](docs/development.md)，后续工作见[项目待办](https://github.com/zhYou/go-cli-starter/issues?q=is%3Aissue+state%3Aopen+label%3Abacklog)。

## 发布

GitHub Actions 对 PR 和 main 执行检查。Linux 运行完整检查、race、fuzz 和 benchmark，macOS、Windows 运行原生测试和构建。外部进程的运行保障须以对应平台 CI 为证据，Linux 交叉编译仅证明可构建。发布 workflow 在推送 `v*` 标签时执行。

确认检查和本地 snapshot 通过后，在目标项目创建并推送标签：

```sh
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser 使用 workflow 的 `GITHUB_TOKEN` 创建 Release，生成 Linux、macOS、Windows 的 amd64/arm64 产物和校验文件。Linux/macOS 使用 tar.gz，Windows 使用 zip；纯 Go 二进制包含版本、提交和构建时间。

模板没有预选许可证；发布自己的项目时按需要添加许可证文件。

模板版本与后续合并方式见[模板升级](docs/template-upgrades.md)。

`mise run vuln` 独立访问 Go 漏洞数据库，PR、main 和每日定时执行；发现漏洞、数据库不可达或工具缺失均失败。源码 check 不请求漏洞数据库。

发布包含每个归档的 SPDX JSON 和最终 checksums.txt。下载同一 Release 的资产后先验证校验和，再检查 SPDX 与来源证明：

```sh
sha256sum -c checksums.txt
# macOS 可用 shasum -a 256 -c checksums.txt
gh attestation verify <asset> --repo <owner/repo>
```

将占位符替换为实际资产与目标仓库。每个 `<archive>.spdx.json` 的 name 对应归档，packages 列出分析到的依赖；它描述最终构建依赖，不等于源码许可证合规结论。
`mise run release:snapshot` 与 `mise run release:verify` 仅在本地构建和验证，不生成或上传远端证明。
私有仓库须先确认套餐和组织策略支持 artifact attestations。证明失败会使 release workflow 失败；若资产已发布，重试时使用已发布的原始字节生成证明，不能重新构建或覆盖资产。

维护者从[维护清单](docs/maintenance.md)配置审查、安全报告、依赖更新和托管发布；开发扩展见[开发说明](docs/development.md)，机器接口变更见[兼容性规则](docs/compatibility.md)。

安装已验证的发布归档后可直接解包运行。使用 mise 时按[开发说明中的固定 URL 与 SHA 安装流程](docs/development.md#安装发布程序)
选择宿主归档，不要把应用 HOME 当作二进制安装目录。`release:verify` 还会用隔离 mise 验证错误 SHA 被拒绝、正确 SHA 安装后运行
version/help/schema/TOML；该本地安装验收只证明实际执行的宿主平台，其他平台由原生 CI 验证。
