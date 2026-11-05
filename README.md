# go-cli-starter

可直接复制复用的 Go CLI 项目模板。主要面向智能体调用，兼顾手动使用。开发和维护由智能体执行，维护者负责需求、合并和发布决策。

默认输出文本，通过 `--json` 输出结构化结果。包含命令发现、配置、日志、测试、本地环境、项目改名和跨平台发布。

项目交流和文档使用中文。CLI 默认 en，可显式选择 zh-CN 或 zh，不随系统区域改变；命令名、选项名、机器字段和退出码保持稳定。设计取舍与扩展示例见[设计指南](docs/design-guidelines.md)。

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

mise、git、gh、Codex、RTK、superpowers 由用户维护，项目不安装或更新。
`doctor` 要求 mise、git 可用；gh、Codex、RTK 仅报告状态，缺失或探测失败不影响检查结果，
superpowers 以实际会话技能列表为准。

六个项目工具统一由 `mise.toml` 声明，`setup` 按显式清单安装，`doctor` 检查全部固定版本。

<!-- tool-versions:start -->

预装工具的版本门槛：

| 工具 | 固定版本 |
| --- | --- |
| mise（本地最低版本、CI 固定版本） | 2026.10.21 |

开发工具：

| 工具 | 固定版本 |
| --- | --- |
| Go | 1.27.3 |
| golangci-lint | 2.15.0 |
| GoReleaser | 2.19.0 |
| govulncheck | 1.9.0 |
| Syft | 1.55.0 |
| gopls | 0.24.0 |

<!-- tool-versions:end -->

有官方 LTS 时使用最新 LTS 补丁版，否则使用最新稳定版；核验后固定具体版本。本地和 CI 读取同一份工具配置。Go 没有单独的 LTS 渠道，模板采用上表固定的稳定版本。

首次使用及工具版本或 Go 模块变更后运行 `setup`：只按显式清单安装六个项目工具并下载模块，不安装用户全局配置中的其他工具。
普通任务和 exec 均关闭自动安装，
`GOTOOLCHAIN=local` 防止隐式下载；日常检查不会安装、升级或修复环境。

项目关闭 mise 锁文件机制，准确版本来自上述 TOML；安装时由各后端解析来源并执行其校验，
仓库不再保存六平台工具下载 URL 和校验和。`go.sum` 继续校验 Go 模块，平台支持通过原生 CI 与发布构建验证。

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
./bin/go-cli-starter doctor --json
./bin/go-cli-starter help completion bash
GO_CLI_STARTER_HOME="$(mktemp -d)" ./bin/go-cli-starter greet --name Codex --json
./bin/go-cli-starter version --json
```

示例结果：

```json
{"message":"Hello, Codex!"}
```

- 默认文本；`--json` 不随终端环境改变，也不由配置或环境变量隐式开启。
- 成功结果写 stdout，错误和诊断告警写 stderr；日志默认写应用私有状态目录，显式配置后可写 stderr。执行或序列化失败时 stdout 为空，错误输出为文本。doctor 的已完成失败报告例外，见下文。底层写入失败可能留下部分字节，消费方须先检查退出码。
- 缺少必需参数直接失败。
- `schema [命令路径...]` 总是输出 JSON，包含命令树、用法、示例、参数类型、必填标记、默认值和退出码。greet/version/doctor 的 result_schema 公开 Draft 2020-12 结果结构。消费方应忽略新增字段；配置文件仍拒绝未知键。
- 帮助输出文本，补全输出 shell 脚本；它们不受 `--json` 影响。
- 检查退出状态后再解析 JSON。测试真实退出码时使用构建后的程序；`go run` 会包装非零退出码。

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 运行或配置错误 |
| 2 | 命令或参数错误 |
| 130 | 信号或 context 取消 |

## doctor 与 shell 补全

`doctor` 是供派生项目扩展的空入口：默认文本说明尚未配置检查，`--json` 返回
`{"required_available":true,"checks":[]}`，退出码为 0。它不探测开发工具、不读取应用配置、
不创建日志或 Trace。开发环境检查使用 `mise run doctor`。

扩展后，已完成检查但必需项不可用时，doctor 仍输出完整报告并返回 1；可选项失败不影响整体成功。
执行或渲染失败仍保持 stdout 为空，写入失败可能留下部分字节。消费方须先检查退出码，再解析报告。

`completion bash|zsh|fish|powershell` 生成对应 shell 的脚本，各 shell 都是独立子命令：
`help completion bash` 和 `schema completion bash` 可查看专属帮助和元数据。
帮助包含双语示例；语言、日志级别和多级 help 主题支持候选补全。

Bash 先加载系统已有的 bash-completion，再加载生成脚本；其他 shell 按其标准方式加载脚本。
CLI 只输出脚本，不自动安装或修改 shell 配置。真实使用示例：

```sh
./bin/go-cli-starter completion bash > go-cli-starter.bash
source ./go-cli-starter.bash
```

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

语言独立于配置：`--lang` > `GO_CLI_STARTER_LANG` > en；中文使用 zh-CN，zh 是别名。
帮助、schema 描述、结果正文、错误和日志消息使用所选语言，第三方诊断可保留原文。
帮助与成功结果在完整渲染成功后才提交；这些消息加载或渲染失败会报告错误。诊断本身发生故障时
使用安全兜底，避免输出部分渲染文本，日志和 Trace 故障仍告警并继续业务。
帮助、version、completion 和 schema 不读取配置、不创建诊断目录，损坏配置下仍可发现命令。
消息集中维护在 `internal/locales/` 的双语 TOML 中，暂不支持运行时翻译覆盖或 `[i18n]` 配置。

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
GO_CLI_STARTER_HOME="$example_home" ./bin/go-cli-starter greet --name Codex --lang zh-CN --json
```

## Codex 开发

先准备环境，再从项目根目录启动 Codex：

```sh
mise run setup
mise run doctor
codex
```

`AGENTS.md` 记录仓库约定和验证命令。`.codex/config.toml` 为受信任项目配置 gopls MCP，
通过 `mise exec -- gopls mcp` 使用固定版本，启动失败不会阻塞 Codex。验证连接：

```sh
mise run mcp:check
codex mcp list
```

`mcp:check` 真正完成初始化、发现工具，并对当前项目运行 Go 诊断。Codex 需重新启动或重新连接，才能加载新增配置。

RTK 和 superpowers 使用用户目录中的安装。模板不复制全局技能，不写个人路径、模型、认证或权限偏好。
统一 `doctor` 要求固定版本的 gopls 可用；gh、Codex、RTK 不限定版本、不检查认证，
缺失、执行失败、超时或输出截断均仅报告，superpowers 是否加载以实际会话技能列表为准。
基础检查无需 gh、Codex、RTK 或 superpowers；CI 的独立 `agent-mcp` job 复用统一 setup，验证真实 MCP 连接。

当前仅支持 Codex。Claude Code 和 OpenCode 支持见[项目待办](https://github.com/zhYou/go-cli-starter/issues?q=is%3Aissue+state%3Aopen+label%3Abacklog)；TUI 和 CLI 自身的 MCP 服务暂不在范围内。

隔离 worktree、分支命名、提交署名和 PR 交付规则见 [协作约定](AGENTS.md#隔离开发与交付)。

## 开发命令

mise 任务保留项目流程、聚合检查和自定义工具：

| 命令 | 用途 |
| --- | --- |
| `mise run setup` | 安装六个固定版本的项目工具，下载模块 |
| `mise run doctor` | 检查项目环境，报告用户工具状态 |
| `mise run lint` | 聚合双语消息、应用配置、包文档和 18 项静态检查 |
| `mise run i18n:check` / `config:check` | 校验双语消息目录 / 受管应用配置；均纳入 lint 和 check |
| `mise run doc:check` | 检查正式 Go 包的 doc.go 与包说明位置；纳入 lint 和 check |
| `mise run rules:check` | 用真实类型边界 fixture 验证静态规则 |
| `mise run build` | 构建到 bin，注入版本信息 |
| `mise run check` | 与 CI 相同的完整源码检查 |
| `mise run template:check` | 在临时副本中验证改名、测试、构建和发布配置 |
| `mise run init --module <模块路径> --name <命令名>` | 初始化模板身份 |
| `mise run contract:check` / `contract:update` | 检查 / 显式更新机器契约 |
| `mise run versions:check` | 检查工具、模块、CI 和文档版本一致性 |
| `mise run mcp:check` | 验证 gopls MCP 的实际连接和诊断 |
| `mise run release:verify` | 验证发布归档，并隔离安装宿主程序 |

原生工具通过 `mise exec --` 执行，无需 shell 激活；这些命令不另设任务别名：

| 命令 | 用途 |
| --- | --- |
| `mise exec -- golangci-lint fmt --color=never` | 使用 gofmt 和 goimports 格式化 |
| `mise exec -- golangci-lint fmt --diff --color=never` | 只读检查格式；有差异时失败 |
| `mise exec -- go test ./...` | 单元与命令测试 |
| `mise exec -- go test -race ./...` | 竞态检测，需要 C 编译器 |
| `mise exec -- go vet ./...` | Go vet 检查 |
| `mise exec -- go mod tidy -diff` | 只读检查模块元数据 |
| `mise exec -- govulncheck ./...` | 联网扫描 Go 漏洞 |
| `mise exec -- goreleaser check` | 验证发布配置，需要 Git remote |
| `mise exec -- goreleaser release --snapshot --clean --parallelism 2` | 本地构建全部发布压缩包，不发布 |

Linux CI 另外执行有界模糊测试和开销测量，后者没有性能门槛：

```sh
mise exec -- go test ./internal/app/cli -run=^$ -fuzz=FuzzArgv -fuzztime=5s
mise exec -- go test ./internal/app/cli -run=^$ -bench=. -benchmem
```

`check` 包含上述只读格式、模块、单元测试和 vet 检查，以及表中的聚合检查和构建。
`lint` 额外聚合双语消息、配置与包文档检查；单独运行 `mise exec -- golangci-lint run --allow-serial-runners` 只执行工具自身的静态检查。

目录按入口、内部包、配置、测试和工具分工：

```text
cmd/<name>/                 程序入口与信号测试
internal/app/               CLI、配置与业务逻辑
internal/base/              错误、消息、路径、校验、IO、日志、Trace、诊断、schema
internal/locales/           集中维护的双语 TOML 消息目录
internal/command/           运行时外部命令与有界并行
configs/config.example.toml 配置示例
test/integration/           真实进程测试与脚本
test/testdata/contracts/    机器契约基线
tools/dev/                 开发与发布验收入口
tools/init/                模板初始化入口
tools/internal/            模板、契约及配置和语言检查的工具共享包
docs/                      使用和维护说明
```

`cmd/<name>` 入口负责信号和退出；`internal/app/cli` 组织命令，`internal/app/config` 加载配置，`internal/app/greeting` 演示业务逻辑；`internal/base` 提供不依赖应用、开发工具或 Cobra 的公共能力；`internal/base/schema` 负责运行时结果结构，`tools/internal/contract` 负责开发期快照比较。`internal/command` 提供业务可复用的外部进程执行能力。根目录 `tools` 提供初始化与开发任务，共享实现集中在 `tools/internal`；运行时代码及其测试不依赖这些开发包。

初始化后的命令目录随 `--name` 改名；模板版本为 0.3.0，已有项目迁移见[模板升级](docs/template-upgrades.md)。发布包仍在根目录提供 `config.example.toml`。

`check` 在临时仓库中验证发布配置，无需为内部工具配置 remote。真正发布或运行 `mise exec -- goreleaser check` 前，为目标项目设置自己的 GitHub origin。

包文档与 IDE 文件折叠配置、新增命令、工具升级和发布步骤见 [开发说明](docs/development.md)，后续工作见[项目待办](https://github.com/zhYou/go-cli-starter/issues?q=is%3Aissue+state%3Aopen+label%3Abacklog)。

## 发布

GitHub Actions 只对目标为 main 的 PR 和 main 更新执行检查；开发分支普通 push 和开发分支之间的 PR 不执行。
PR 改目标到 main、标题或描述编辑也会触发检查，同一 PR 的新运行会取消旧运行；main、定时扫描与 tag 检查互不取消。
Security 保留每日扫描。

Linux 的源码检查、race、fuzz/benchmark、六目标 snapshot 打包与安装验收分组并行执行；
macOS、Windows 继续运行原生测试和构建，gopls MCP 在独立任务验证。Go 模块、编译结果和 lint 结果使用跨运行缓存，
Windows 仅缓存 Go 模块，编译结果在当前 runner 内生成；实测恢复跨运行编译缓存后 Windows 全量检查耗时增加。
缓存未命中时完整执行。`check` 汇总全部 CI 任务，任一失败、取消或跳过都不能通过。
外部进程的运行保障须以对应平台 CI 为证据，Linux 交叉编译仅证明可构建。

推送 `v*` 标签后，Release 先复用该提交的完整 CI 和 Security；全部成功才生成并发布正式资产，
随后验收实际发布产物并生成来源证明。普通检查只有读取仓库的权限，发布任务单独申请写入权限。

确认检查和本地 snapshot 通过后，在目标项目创建并推送标签：

```sh
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser 使用 workflow 的 `GITHUB_TOKEN` 创建 Release，生成 Linux、macOS、Windows 的 amd64/arm64 产物和校验文件。Linux/macOS 使用 tar.gz，Windows 使用 zip；纯 Go 二进制包含版本、提交和构建时间。

模板没有预选许可证；发布自己的项目时按需要添加许可证文件。

模板版本与后续合并方式见[模板升级](docs/template-upgrades.md)。

`mise exec -- govulncheck ./...` 独立访问 Go 漏洞数据库，在目标为 main 的 PR、main 更新、版本 tag 发布前和每日定时执行；发现漏洞、数据库不可达或工具缺失均失败。源码 check 不请求漏洞数据库。

发布包含每个归档的 SPDX JSON 和最终 checksums.txt。下载同一 Release 的资产后先验证校验和，再检查 SPDX 与来源证明：

```sh
sha256sum -c checksums.txt
# macOS 可用 shasum -a 256 -c checksums.txt
gh attestation verify <asset> --repo <owner/repo>
```

将占位符替换为实际资产与目标仓库。每个 `<archive>.spdx.json` 的 name 对应归档，packages 列出分析到的依赖；它描述最终构建依赖，不等于源码许可证合规结论。
`mise exec -- goreleaser release --snapshot --clean --parallelism 2` 与 `mise run release:verify` 仅在本地构建和验证，不生成或上传远端证明。
私有仓库须先确认套餐和组织策略支持 artifact attestations。证明失败会使 release workflow 失败；若资产已发布，重试时使用已发布的原始字节生成证明，不能重新构建或覆盖资产。

维护者从[维护清单](docs/maintenance.md)配置审查、安全报告、依赖更新和托管发布；开发扩展见[开发说明](docs/development.md)，机器接口变更见[兼容性规则](docs/compatibility.md)。

安装已验证的发布归档后可直接解包运行。使用 mise 时按[开发说明中的固定 URL 与 SHA 安装流程](docs/development.md#安装发布程序)
选择宿主归档，不要把应用 HOME 当作二进制安装目录。`release:verify` 还会用隔离 mise 验证错误 SHA 被拒绝、正确 SHA 安装后运行
version/help/schema/TOML；该本地安装验收只证明实际执行的宿主平台，其他平台由原生 CI 验证。
