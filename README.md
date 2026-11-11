# go-cli-starter

可直接复制复用的 Go CLI 项目模板，主要面向智能体调用，兼顾手动使用。
默认文本输出，通过 `--json` 提供结构化结果；包含命令发现、严格配置、双语消息、日志、测试、项目改名和跨平台发布。
开发和维护由智能体执行，维护者负责需求、合并和发布决策。

## 创建新项目

在 GitHub 使用 **Use this template** 创建仓库并克隆，或复制文件后执行 `git init`。
准备好 Git 和下表要求的 mise，在新项目根目录执行：

```sh
mise trust
mise run setup
mise run template:init --module github.com/you/mycli --name mycli
mise run check
```

初始化只更新受管文件中的模块路径、Go 导入、命令名、环境变量前缀及构建发布配置，
不修改 remote、提交历史或用户级配置。同参数重复运行不改文件，已初始化项目拒绝不同身份覆盖。
命令名以字母开头，使用小写字母、数字和连字符；拒绝会改变 Go 安装产物名的 `v2`、`v10` 等版本段名称，
`v0`、`v1`、`v02` 可用。已有项目升级见[模板升级](docs/template-upgrades.md)，不要用初始化覆盖业务。

## 本地环境

[安装 mise](https://mise.jdx.dev/getting-started.html)。mise、Git、gh、Codex、RTK 和 superpowers 由用户维护；
mise 安装声明的运行时、Go 和原生工具；文本工具、Serena 与 Graphify 通过固定版本的 npx/uvx 在外部缓存按需准备。

<!-- tool-versions:start -->

预装工具的版本门槛：

| 工具                              | 固定版本   |
| --------------------------------- | ---------- |
| mise（本地最低版本、CI 固定版本） | 2026.10.21 |

开发工具：

| 工具                 | 固定版本 |
| -------------------- | -------- |
| Go                   | 1.27.4   |
| golangci-lint        | 2.15.0   |
| actionlint           | 1.7.12   |
| GoReleaser           | 2.19.0   |
| govulncheck          | 1.9.0    |
| Syft                 | 1.55.0   |
| gopls                | 0.24.0   |
| Node.js              | 26.13.0  |
| Python               | 3.15.2   |
| uv                   | 0.13.7   |
| Taplo                | 0.10.0   |
| Prettier             | 3.9.10   |
| @prettier/plugin-xml | 3.4.2    |
| prettier-plugin-toml | 4.1.0    |
| markdownlint-cli2    | 0.24.0   |
| yamllint             | 1.38.0   |
| pre-commit-hooks     | 6.0.0    |
| Serena               | 2.1.0    |
| Serena Python        | 3.14.10  |
| Graphify             | 0.9.106  |
| Graphify Python      | 3.14.10  |

<!-- tool-versions:end -->

首次准备环境、工具版本或 Go 模块变更后执行 `mise run setup`，再运行 `mise run doctor`；声明未变时无需重复 setup。
mise 普通任务和 exec 关闭运行时自动安装，`GOTOOLCHAIN=local` 防止隐式下载 Go 工具链。
npx/uvx 可按需下载固定版本，检查不改写项目文件，不创建本地 `node_modules`、`.venv` 或 npm 工程。
项目不使用 mise 锁文件，`go.sum` 校验模块。doctor 的必需项和可选报告见[开发说明](docs/development.md#协作与环境)。

```sh
mise run build
GO_CLI_STARTER_HOME="$(mktemp -d)" mise exec -- go run ./cmd/go-cli-starter greet --name Codex --json
```

输出：

```json
{"message":"Hello, Codex!"}
```

发布程序不依赖 mise 或 Go。源码安装入口是 `cmd/go-cli-starter`：

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

- 默认文本，`--json` 不由终端、配置或环境隐式开启；缺少必需参数立即失败，不交互询问。
- 成功结果写 stdout，错误和告警写 stderr；`--json` 时诊断为带 type 的 JSONL，日志使用独立出口。执行或序列化失败时 stdout 为空；底层写入失败可能留下部分字节，须先检查退出码。
- `schema [命令路径...]` 始终输出 JSON，包含命令树、参数、默认值和退出码；greet、version、doctor 提供 Draft 2020-12 结果结构。结果允许新增字段，配置文件仍拒绝未知键；diagnostic_schema 描述结构化错误和告警。
- 帮助是文本，补全是原生 shell 脚本，不受 `--json` 影响。验证真实退出码请用构建后的程序，`go run` 会包装非零退出码。

| 退出码 | 含义                |
| ------ | ------------------- |
| 0      | 成功                |
| 1      | 运行或配置错误      |
| 2      | 命令或参数错误      |
| 130    | 信号或 context 取消 |

## doctor 与 shell 补全

`mise run doctor` 检查开发环境；产品 CLI 的 `doctor` 是供派生项目扩展的空检查入口：
默认文本说明尚未配置检查，JSON 为 `{"checks":[],"required_available":true}`，退出 0。
空入口不读取配置或创建诊断资源；[扩展方式](docs/design-guidelines.md#扩展-doctor)见设计指南。
已完成的报告中，必需项失败时仍输出完整报告并退出 1，可选失败不改变成功状态；执行或渲染失败保持空 stdout。

支持 `completion bash`、`zsh`、`fish`、`powershell`，可对具体 shell 请求 help 或 schema；`completion <shell>` 调用保持可用。
Bash 先加载系统已有的 bash-completion；其他 shell 按其要求加载依赖。CLI 只生成脚本，不安装或修改 shell 配置：

```sh
./bin/go-cli-starter completion bash > go-cli-starter.bash
source ./go-cli-starter.bash
```

## 配置、语言和诊断

只读取一个 `config.toml`，示例见 [configs/config.example.toml](configs/config.example.toml)。
`--config` 指定显式文件，否则使用应用私有配置目录。默认文件缺失可继续，显式文件缺失、未知键、类型或范围错误会失败。
配置优先级为显式 flag > 非空环境变量 > TOML > 默认值，不自动读取或合并 YAML。

```toml
[logging]
level = "info"
output = "file" # file、stderr、none

[telemetry]
exporter = "none" # none、file
```

模块环境变量如 `GO_CLI_STARTER_LOGGING_LEVEL`、`GO_CLI_STARTER_LOGGING_OUTPUT`、`GO_CLI_STARTER_TELEMETRY_EXPORTER`。
兼容 `GO_CLI_STARTER_LOG_LEVEL` 优先于新的级别变量，显式 `--log-level` 优先于两者；未给出的 flag 不覆盖文件或环境值。

语言独立于配置：`--lang` > `GO_CLI_STARTER_LANG` > en，中文使用 zh-CN 或别名 zh，不读取系统区域。
帮助、schema 描述、结果、错误及日志使用所选语言；命令、选项、JSON 字段、枚举和退出码保持稳定，第三方诊断可保留原文。
帮助、version、completion、schema 不读取配置或创建诊断目录，坏配置下仍可发现命令。不支持运行时翻译覆盖或 `[i18n]` 配置。

<!-- prettier-ignore -->
| 平台 | 配置目录 | 数据 / 状态 / 缓存 |
| --- | --- | --- |
| Linux | `$XDG_CONFIG_HOME/go-cli-starter`，默认 `~/.config/go-cli-starter` | 对应 XDG 根；默认 `~/.local/share`、`~/.local/state`、`~/.cache` 下的应用目录 |
| macOS | `~/Library/Application Support/go-cli-starter` | 配置目录下 data、state；`~/Library/Caches/go-cli-starter` |
| Windows | `%APPDATA%/go-cli-starter` | `%LOCALAPPDATA%/go-cli-starter` 下 data、state、cache |

`GO_CLI_STARTER_HOME` 可指定绝对私有根：配置文件位于根下，数据、状态和缓存分别位于 data、state、cache。
改名为 mycli 后使用 `MYCLI_HOME`。相对应用 HOME 会报错；Linux 相对 XDG 变量回退到 HOME。此目录与程序安装位置无关。

日志默认写 `<state>/logs/<run_id>/events.jsonl`；Trace 默认关闭，file 写同次运行的 `trace.jsonl`，只供本地诊断，不承诺 OTLP 或重放。
诊断故障告警并继续业务。错误可包含失败阶段、已确认副作用和恢复建议；JSONL 消费及敏感输入边界见[结构化诊断](docs/design-guidelines.md#结构化诊断)。轮转和清理边界见[诊断资源所有权](docs/design-guidelines.md#日志trace-和异步工作的所有权)。
验证示例时隔离应用配置与状态：

```sh
example_home=$(mktemp -d)
cp configs/config.example.toml "$example_home/config.toml"
GO_CLI_STARTER_HOME="$example_home" ./bin/go-cli-starter greet --name Codex --lang zh-CN --json
```

## Codex 开发

环境准备完成后，从仓库根目录启动 `codex`。当前仅支持 Codex，RTK 和 superpowers 使用用户级安装。
受信任项目通过 `.codex/config.toml` 和 `mcp:serve`、`graphify:serve` 启动 Serena 与 Graphify，服务启动失败不阻塞 Codex。
Serena 使用独立兼容 Python 并关闭 dashboard，覆盖 Go、JavaScript、JSON、YAML、TOML 和 Markdown。
Graphify 使用独立兼容 Python 建立本地代码图，CLI 负责建图和更新，MCP 查询关系；不需要 API Key。
创建或切换 worktree 后，以绝对路径激活 Serena 并核对根目录；Graphify 每次查询传当前 checkout 的绝对 `project_path`。
未覆盖的 Go 能力使用当前 checkout 的 gopls CLI，精确文本使用 rg；职责与回退见[开发说明](docs/development.md#graphify-与工具分工)。

需要代码图时执行 `mise run graphify:build`，修改后使用 `mise run graphify:update`；索引位于被忽略的 `graphify-out/`，各 checkout 独立维护。
单次会话不需要这两个 MCP 时：

```sh
codex -c 'mcp_servers.serena.enabled=false' -c 'mcp_servers.graphify.enabled=false'
```

连接验收见[开发说明](docs/development.md#实现与检查)，协作、分支及交付规则见 [AGENTS.md](AGENTS.md)。

## 开发命令

优先使用已有 task；直接调用项目纳管工具使用 `mise exec -- <命令>`，Git、gh 等其他命令直接执行。
task 内由 mise 提供工具环境，无需额外包装。

| 命令                                                         | 用途                                             |
| ------------------------------------------------------------ | ------------------------------------------------ |
| `mise run setup` / `mise run doctor`                         | 显式准备并检查环境                               |
| `mise run build`                                             | 构建到 bin，注入版本信息                         |
| `mise run fmt`                                               | 显式格式化 Go 和 JSON、XML、TOML、Markdown、YAML |
| `mise run lint`                                              | 只读检查 Go、五类文本、工作流语义、资源和声明    |
| `mise run check`                                             | 完整源码、测试、构建、契约与模板验收             |
| `mise run template:init --module <模块路径> --name <命令名>` | 初始化模板身份                                   |

完整任务表、直接工具命令和按改动选择的检查见[开发说明](docs/development.md#开发命令)。
本地检查不等于全部 CI；平台运行、发布、安全和托管要求见[维护清单](docs/maintenance.md)。

## 发布

维护者确认发布后推送 `v*` 标签，Release 先运行完整 CI 和 Security，再发布 Linux、macOS、Windows 的 amd64/arm64 产物。
归档附带配置示例、SPDX、校验和及来源证明；验证命令、权限和失败重试见[发布流程](docs/maintenance.md#升级和发布)。
目标项目发布前须配置自己的 GitHub origin、安全报告入口和托管规则。模板没有预选许可证，按项目需要添加。

## 文档导航

| 文档                                  | 内容                                 |
| ------------------------------------- | ------------------------------------ |
| [协作约定](AGENTS.md)                 | 智能体入口、命令、工作区、交付与保护 |
| [设计指南](docs/design-guidelines.md) | 架构、CLI 契约、API 示例与所有权     |
| [开发说明](docs/development.md)       | 完整命令、验证、测试组织与工具维护   |
| [兼容性规则](docs/compatibility.md)   | 机器接口与变更记录                   |
| [模板升级](docs/template-upgrades.md) | 来源记录、旧事务和迁移步骤           |
| [维护清单](docs/maintenance.md)       | 托管配置、CI、发布与验收证据         |
| [安全报告](SECURITY.md)               | 目标项目须填写的私密报告入口         |
| [项目待办](docs/backlog.md)           | 明确要求记录的探索想法               |

模板版本为 0.4.1；Claude Code 和 OpenCode 支持列在待办中，TUI 和 CLI 自身的 MCP 服务暂不在范围内。
