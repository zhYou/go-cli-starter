# 开发说明

先读 [README](../README.md) 准备环境，再按[设计指南](design-guidelines.md)扩展当前模块。
项目文档、注释与交流使用中文，CLI 默认英文并支持中文，机器标识保持稳定。

## 协作与环境

修改默认在独立 worktree 中进行，基线、分支命名、已有修改保护、署名和 PR 交付遵循
[协作约定](../AGENTS.md#隔离开发与交付)。同一 checkout 串行修改。
正式文档记录当前能力；superpowers 设计和计划保留在本地忽略目录 `docs/superpowers/`，不提交。
当前已授权任务无需先建 Issue；已有对应 Issue 时由 PR 关联，完整解决时才使用关闭关联。
尚未安排的后续需求只记带 `backlog` 标签的 GitHub Issues；待办不是实施授权，Issue 编号是唯一待办 ID。

仓库文本文件统一使用 LF。`.editorconfig` 约束编辑器保存，`.gitattributes` 通过
`text=auto eol=lf` 约束 Git 提交和检出，包括 Windows；二进制文件由 Git 自动识别。
调整换行规则时，确认工作区只有本次修改，再执行 `git add --renormalize .`，审查差异后提交。

首次使用及工具版本或 Go 模块变更后显式运行 `mise run setup`，然后运行 `mise run doctor`。
setup 只按显式清单安装开发工具并下载模块。智能体开发使用 `setup:agent`，它先完成 setup，
再安装智能体辅助工具。普通任务与 exec 的自动安装均关闭，
`GOTOOLCHAIN=local` 禁止 Go 隐式下载工具链。缺工具应报错，检查不能兼任安装、升级或修复。
预装层包括 mise、git、gh、Codex、RTK、superpowers，由用户维护；模型、认证、权限和插件偏好不进入仓库。
开发层由 `mise.toml` 声明 Go、golangci-lint、GoReleaser、govulncheck、Syft；
智能体辅助层由 `mise.agent.toml` 声明 gopls，显式选择 agent 环境才加载。
doctor 检查 mise 最低版本、git 和开发层准确版本；doctor:agent 额外要求 gh、Codex、gopls 可用，
RTK 仅报告状态，superpowers 以实际会话技能列表为准。gh、Codex、RTK 只检查可运行性，不检查认证或固定版本。
基础检查无需智能体配套；CI 的独立 agent-mcp job 运行 setup:agent 和 mcp:check，无需这些预装智能体工具。

项目设置 `lockfile = false`，不维护 mise 锁文件。准确版本仍由 TOML 声明，安装时由后端解析下载来源并执行其校验，
不再预先保存六平台工具下载 URL 和 SHA-256。Go 模块继续使用 go.sum；运行时系统锁与此无关，不可删除。

## 目录和依赖

| 位置 | 职责 |
| --- | --- |
| `cmd/<name>/` | 程序入口、信号、退出及入口测试 |
| `internal/app/cli` | 独立命令树、参数、输出和退出码映射 |
| `internal/app/config` | 模块配置组合与严格加载 |
| `internal/app/session` | 自建日志、Trace、诊断及工作收尾 |
| `internal/app/greeting` | 可移除业务示例 |
| `internal/base` | fault、messages、paths、validation、IO、logging、telemetry、diagnostics、结果 schema |
| `internal/locales` | 集中双语 TOML 消息目录与共享 Bundle |
| `internal/tools/command` | 独立 argv、进程所有权、捕获和有界批量执行 |
| `internal/dev`、根 `tools/` | 模板、契约快照、开发与发布验收 |
| `configs/`、`test/integration/`、`test/testdata/contracts/` | 配置示例、真实进程脚本、共享契约基线 |

base 不依赖 app、tools、dev 或 Cobra。领域码及结果由所属模块定义，文案集中维护；新目录按实际职责增加。
开发工具用 `scaffold.Inspect` 静态识别唯一入口，不根据模块路径末段猜测命令名。

## 实现与检查

1. 查阅设计指南对应场景，用 gopls 查询 Go 符号、引用和诊断，确认工作区是实际 checkout。
   先通过 `mise -E agent exec -- gopls mcp -instructions` 阅读说明。普通操作按需使用等价 CLI，不机械调用工具填清单。
2. 参照 greet 分离有类型业务选项、结果及 Validate。CLI 的 Args、必填及组合校验在读取配置和创建资源前完成，
   业务入口也调用同一校验。新命令补双语帮助、示例和结果注册，不新增交互输入。
3. 新配置同步模块 Config、defaults、环境映射、TOML、校验和测试，并使用统一加载入口。
   双语消息在同次修改中维护，分别运行 `mise run config:check` 和 `mise run i18n:check`。
   仅 Changed 的 flag 覆盖，禁止逐键 koanf getter；缺译、系统原因和结构化 fault 的规则见指南。
4. 为行为变化运行针对性测试，观察真实结果，修改后检查 gopls 诊断。保留用户已有修改，格式化限于授权范围。
5. 交付前运行 `mise run fmt`、`mise run check`，支持的宿主运行 `mise run test:race`。
   模板/身份变更增加 `template:check`，工具集成增加 `mcp:check`；受信任会话确认 `codex mcp list` 有 gopls。
   发布变更增加 snapshot 与 verify；无法执行的检查写明原因。

所有测试及文档示例验证都隔离父进程和 helper 子进程的应用配置、状态和缓存：

```sh
example_home=$(mktemp -d)
GO_CLI_STARTER_HOME="$example_home" mise run check
GO_CLI_STARTER_HOME="$example_home" mise run test:race
```

改名为 mycli 后使用 `MYCLI_HOME`，由 `paths.EnvPrefix(name)+"_HOME"` 派生。
嵌入调用也可注入 Options.Directories、LookupEnv 与 IO；只设置 ConfigDir 不会隔离日志状态。
`iostreams.IOStreams.WithDefaults` 返回副本，缺省输入 EOF、输出 io.Discard，不访问或关闭进程标准流。
真实退出码用构建后的程序验证，go run 会包装非零退出。输出测试检查单 JSON 与独立 stderr，
坏配置下发现命令不读配置、不落盘。进程测试见 `test/integration`，信号测试见 `cmd/<name>`。

Session 关闭顺序、借用资源和永久阻塞 IO 的边界见设计指南。工具进程的 Unix 组与 Windows Job
需要对应 OS 原生 helper；交叉编译不证明运行、取消或句柄回收成功。CI 配置了三 OS 原生测试，
报告交付结果时以实际 job 结果为准，不把配置存在写成验收完成。

## 测试组织与可靠性

单个源码文件的单元测试尽量放入同名 `*_test.go`；跨文件契约使用清晰的行为名，
例如 `process_tree_test.go`、`stderr_test.go`。无需为常量文件创建空测试，也不为文件名对应拆分生产源码。
多处使用的 fixture 放在所属包的 `helpers_test.go`，平台专用 helper 保留 `_windows_test.go` 等后缀。
单处使用的测试类型就近保留；真实配置加载与注入 stub 的 fixture 各自保持原有边界。
`fuzz_test.go` 维护 fuzz 入口和 seeds，`benchmark_test.go` 维护 benchmark，普通行为测试按实现归位。

集中消息目录损坏场景由 `test/integration/catalog_test.go` 使用 Go overlay 运行 greeting、Session 和
command 三个包的 `TestCorruptCatalogProcess`。完整测试会执行该集成入口；单独运行所属包时，
这些 worker 会明确跳过。可在隔离应用目录下单独验收：

```sh
test_home=$(mktemp -d)
GO_CLI_STARTER_HOME="$test_home" mise exec -- go test ./test/integration -run '^TestCorruptCatalogEndToEnd$' -count=1
```

进程启动、取消和并发先后使用就绪、释放信号协调，并为等待设置截止时间。资源取得后立即登记清理；
断言中途失败也释放阻塞、取消工作，并等待子进程和 goroutine 收尾。直接子进程由唯一调用方完成 Wait。
心跳检查验证观察期内的写入行为，进程退出和句柄回收使用相应平台的原生测试证据。
时间本身属于被测行为的超时、排空宽限和观察窗口可以保留。

检查成功、失败和取消时同时核对状态、退出码及错误链；根进程成功退出而后代持有管道的场景，
明确验证 `exec.ErrWaitDelay` 的既有边界。事件顺序断言先确认事件存在，无状态场景同时核对预期退出码。

## 静态规则与开发探测

`mise run lint` 启用以下 18 项，生产导出 API 需中文 GoDoc；测试仅豁免 GoDoc，其他边界仍生效：

```text
errcheck govet ineffassign staticcheck unused bodyclose errorlint gosec godoclint
contextcheck nilnesserr durationcheck nolintlint thelper usetesting goconst depguard forbidigo
```

局部 nolint 指明检查器并用中文解释。稳定协议和状态使用语义常量，同值不同义可各自命名。
koanf 只可在精确的 `internal/app/config/load.go` 导入，且仅整体解码；禁用 typed/Must getter、
Get、Raw、All 等绕过结构体的入口。zap 只在 base/logging，OTel SDK/exporter 只在 base/telemetry。
`mise run rules:check` 在临时模块运行真实类型分析，验证违规与合规 fixture，并纳入 check。

`mise run i18n:check` 检查 `internal/locales/active.en.toml` 与 `active.zh-CN.toml`：
目录非空，消息 ID 不重复，字段与消息内容有效，模板语法正确；两种语言的 ID 和模板参数集合一致，
复数形式的参数按并集比较。每条消息必须有 other，允许 description 和 zero、one、two、few、many。
检查器支持直接字段、根别名、括号、`if`/`with`/`range`、被调用的非递归命名模板，以及根对象上
字符串字面量或可静态确定的字面量变量 `index`；管道中的字面量键也可识别。`and`/`or` 只有在各个值
都可确定为根对象时才被视为根别名。根对象的动态 `index` 键、无法确定的字段/链/模板上下文，
控制结构或其分支内的变量赋值，以及嵌套管道中的变量声明或赋值会报错；请改为直接字段或在分支外
确定的变量。子对象上的动态键
不会引入额外的根参数。新增或修改消息时同时维护两种语言，稳定 ID 不随语言改变。

`mise run config:check` 递归检查 `configs/` 应用配置、`internal/locales/` 语言资源，以及根目录
明确登记的 `config.toml`、`config.example.toml` 和同名 JSON/YAML 变体。`configs/config.example.toml`
必须存在；`configs/` 仅接受 TOML，`internal/locales/` 接受 TOML 和 Go 文件。两处直接子目录
`testdata/` 只放测试 fixture，检查会跳过；其他受管文件必须是普通文件，不接受符号链接。扩展名
不区分大小写。应用 TOML 禁用环境变量覆盖并经 `config.Load` 校验语法、未知键、类型和值域；
语言 TOML 检查语法，消息语义由 i18n:check 检查。

新增应用配置放入 `configs/` 并通过 `internal/app/config/load.go` 统一加载；确需其他路径时先登记到
`internal/dev/configpolicy`。mise、`.codex`、CI、golangci 等工具配置沿用工具自身格式，其他位置的
业务 JSON、测试数据和协议/契约数据不纳入扫描。两项检查由 lint 调用并纳入 check；它们不会自动安装
工具或修复文件。

`fmt` 使用 golangci-lint 的 gofmt、goimports；`fmt:check` 只展示差异并失败，不改源码。
check 不改写源码，但会使用 Go 缓存、临时文件和 bin 产物。
`test:fuzz` 对 NUL 分隔的独立 argv 做五秒模糊测试，覆盖空值、空格、Unicode、`--` 和取消；
`test:bench` 测量树构造和 greet 调用，没有性能门槛，两者在 Linux CI 执行。

`doctor` 使用共享执行器，最多四项并行，每项三秒，每流 2 KiB，按固定顺序报告精确版本及
超时、取消、截断。govulncheck 读取本地 Go 构建信息验证命令、模块来源及版本，再执行 `-h`，
不访问漏洞库。真正联网扫描由 `mise run vuln` 独立执行，不可用或数据库失败同样失败。

## 修改模板与升级工具

初始化只重写 `internal/dev/scaffold/scaffold.go` 的 managedFiles 白名单。
新增身份文件须明确登记；保留初始化实现的原模板身份和 fixture，禁止全仓替换。
移动或新增文件先暂存，再运行使用 Git 索引清单复制项目的 `template:check`。
它验证改名、幂等、测试、真实构建与 go install 后的绝对路径二进制、version/help/schema/TOML。
旧项目迁移与事务恢复见[模板升级](template-upgrades.md)，初始化不承担升级。

升级选择官方最新 LTS 补丁，无 LTS 则最新稳定版，排除预发布；先查 Go、gopls、golangci-lint 兼容性。
同步 mise.toml、mise.agent.toml、Go 声明、doctor 预期、README 受检工具表及 CI 引导，再执行：

```sh
mise run setup
mise run doctor
mise run setup:agent
mise run doctor:agent
mise run versions:check
```

随后按受影响范围运行 check、race、mcp:check 与 snapshot/verify。提交两份准确版本声明，
通过原生 CI 和六目标发布构建验证平台支持；gopls 等 Go 后端从固定模块版本在宿主平台编译。
README 的 tool-versions 标记与工具显示标签由 versions:check 解析，修改表格时保留它们。
Renovate 由管理员启用，覆盖两份 mise 配置；权限不足时人工更新版本声明并完成检查，不放宽检查。
预装工具通过各自渠道维护，项目 setup 不安装或更新它们。

## 安装发布程序

源码安装入口是 `go install <module>/cmd/<name>@<version>`，安装产物与命令目录同名。
直接解包前先核对同一 Release 的 checksums.txt、SPDX 和来源证明。发布程序本身无需 mise/Go。
应用 HOME 只存运行数据，独立于 go install、解包或 mise 的二进制安装目录。

若用 mise 管理，参照其 [HTTP 后端](https://mise.jdx.dev/dev-tools/backends/http.html)，
把下列占位符替换为选定版本、宿主平台归档固定 URL 和该归档的真实最终 SHA-256，再显式安装。
本地归档可使用绝对 `file:///...` URL；每个平台的摘要分别填写，不要复用：

```toml
[settings]
auto_install = false
exec_auto_install = false

[tools."http:mycli"]
version = "<version>"
url = "<fixed-release-archive-url>"
checksum = "sha256:<archive-sha256>"
```

```sh
mise install http:mycli@<version>
mise which mycli
MYCLI_HOME="$(mktemp -d)" mise exec -- mycli version --json
```

安装 URL 的 version 只是选择标签，不能替代实际二进制 version/commit 验证。
`release:verify` 先核对六归档和 SPDX 最终清单，再在两个独立 mise data/cache 中验证错 SHA 明确失败与
正确 SHA 安装成功，解析绝对安装路径并运行 version/commit/help/schema/TOML。归档必须属于最终清单、
版本一致且 metadata.commit 等于当前 HEAD；任何提交后都须重新 snapshot，旧产物会被拒绝。
以上占位安装示例是人工部署流程；自动验收使用实际 URL 和摘要，不会修改全局 mise 配置。

## 发布与交付

运行 check、race、vuln、release:snapshot、release:verify，核对六目标归档、配置示例、SPDX 和最终摘要。
Syft 固定版本且关闭 enrichment、远程许可证查询及更新检查，SBOM 完成后才生成最终 checksums。
本地不上传证明；受信任 release workflow 对最终资产生成来源证明。证明失败的重试必须使用同一批
已发布原始字节，不能重构建或覆盖 Release。安全与托管权限见[维护清单](maintenance.md)。

PR 按仓库模板说明实际行为、兼容记录和测试证据；提交加 Agent: Codex 尾注，保留审查 worktree。
合并和发布由维护者决定。机器接口变化先读[兼容性规则](compatibility.md)，不要仅刷新快照。
