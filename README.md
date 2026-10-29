# go-cli-starter

可直接复制复用的 Go CLI 项目模板。主要面向智能体调用，兼顾手动使用；使用 Codex 开发和维护。

默认输出文本，通过 `--json` 输出结构化结果。包含命令发现、配置、日志、测试、本地环境、项目改名和跨平台发布。

项目交流、文档和 CLI 提示默认使用中文，不随系统区域设置改变。命令名、选项名和机器字段保持稳定；多语言支持记录在 [项目待办](docs/backlog.md)。

## 创建新项目

在 GitHub 使用 **Use this template** 创建仓库并克隆，或直接复制项目文件后执行 `git init`。准备好 Git 和 mise 2026.10.21 或更新的稳定版，然后在新项目根目录执行：

```sh
mise trust
mise run setup
mise run init --module github.com/you/mycli --name mycli
mise run check
```

初始化更新模块路径、Go 导入、命令名、环境变量前缀和构建发布配置。同参数重复执行不改动文件，已初始化项目拒绝不同身份覆盖。新项目可以独立构建、测试和发布。

初始化仅改写模板管理的文件，不修改 Git remote、提交历史和用户级智能体配置。命令名使用小写字母、数字和连字符，并以字母开头。

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

`setup` 只安装项目声明的工具并下载 Go 模块，不安装用户全局配置中的其他工具。日常任务不会自动安装或升级工具；修改依赖后重新运行 `setup`。

```sh
mise run doctor
mise run build
mise exec -- go run . greet --name Codex
mise exec -- go run . greet --name Codex --json
```

发布产物本身不依赖 mise 或 Go。源码安装入口是仓库根目录：

```sh
go install github.com/zhYou/go-cli-starter@latest
```

## 给智能体调用

```sh
./bin/go-cli-starter --help
./bin/go-cli-starter schema
./bin/go-cli-starter schema greet
./bin/go-cli-starter greet --name Codex --json
./bin/go-cli-starter version --json
```

示例结果：

```json
{"message":"你好，Codex！"}
```

- 默认文本；`--json` 不随终端环境改变，也不由配置或环境变量隐式开启。
- 成功结果写 stdout，日志和错误写 stderr。执行或序列化失败时 stdout 为空，错误输出为文本。底层写入失败可能留下部分字节，消费方须先检查退出码。
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

## 配置和日志

复制 `config.example.yaml` 到系统用户配置目录下的 `go-cli-starter/config.yaml`，或用 `--config` 显式指定文件。用户配置目录由 Go 的 `os.UserConfigDir()` 决定。

```yaml
log_level: warn
```

日志级别支持 debug、info、warn、error，默认 warn。覆盖顺序为显式 `--log-level`、`GO_CLI_STARTER_LOG_LEVEL`、YAML、默认值；空环境变量按未设置处理。

默认文件不存在时照常运行。显式文件不存在、YAML 格式错误、未知配置项或无效值会报错。帮助、版本、补全和 schema 不读取配置。

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

当前仅支持 Codex。Claude Code 和 OpenCode 支持见 [项目待办](docs/backlog.md)；TUI 和 CLI 自身的 MCP 服务暂不在范围内。

## 开发命令

| 命令 | 用途 |
| --- | --- |
| `mise run setup` | 安装锁定工具，下载模块 |
| `mise run doctor` | 检查工具和版本 |
| `mise run fmt` | 格式化 Go 文件 |
| `mise run test` | 单元与命令测试 |
| `mise run test:race` | 竞态检测，需要 C 编译器 |
| `mise run lint` | 静态检查 |
| `mise run build` | 构建到 bin，注入版本信息 |
| `mise run check` | 与 CI 相同的完整源码检查 |
| `mise run template:check` | 在临时副本中验证改名、测试、构建和发布配置 |
| `mise run release:check` | 验证当前仓库的发布配置和 SCM 信息，需要 remote |
| `mise run release:snapshot` | 本地构建全部发布压缩包，不发布 |

根目录入口负责信号和退出；`internal/cli` 组织命令，`internal/config` 加载配置，`internal/greeting` 演示业务逻辑；`internal/scaffold` 和 `tools` 提供初始化与开发任务。

`check` 在临时仓库中验证发布配置，无需为内部工具配置 remote。真正发布或运行 `release:check` 前，为目标项目设置自己的 GitHub origin。

新增命令、工具升级和发布步骤见 [开发说明](docs/development.md)，后续工作见 [项目待办](docs/backlog.md)。

## 发布

GitHub Actions 对 PR 和 main 执行检查。Linux 运行完整检查和 race，macOS、Windows 运行测试和构建。发布 workflow 在推送 `v*` 标签时执行。

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
