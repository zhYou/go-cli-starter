# CLI 设计指南

本指南供新增命令、模块和维护工具时使用。先找到使用场景，再沿当前实现扩展。
标为“项目选择”的内容是本仓库契约；链接的库文档说明库本身的能力，两者不能互相替代。

## 先确定边界，再增加目录

一个命令需要 CLI、业务函数和运行资源时，项目选择由 `cmd/<name>` 处理信号与退出，
`internal/app/cli` 处理参数和呈现，应用模块定义自己的选项、结果、错误码和消息目录，
`internal/app/session` 组合运行资源。这样业务可直接调用并校验输入，无需导入 Cobra。
公共机制在 `internal/base`，不得反向依赖 app、tools、dev 或 Cobra；外部进程在
`internal/tools/command`，模板、快照及工具探测在 `internal/dev` 和根 `tools`。

例如问候逻辑位于 `internal/app/greeting`，结果 schema 的生成机制位于 `internal/base/schema`，
开发期快照比较位于 `internal/dev/contract`。不要把新业务错误码加入 base 的全局目录，
也不要为符合目录图而创建空包。构造函数应完成默认值、校验、依赖组合或资源建立；
简单选项值可以直接使用字面量。嵌套组合较复杂时逐字段分行，便于审查所有权。

[Go 官方布局指南](https://go.dev/doc/modules/layout)说明 internal 等布局方法；
[project-layout](https://github.com/golang-standards/project-layout)是社区模式集合，不是 Go 官方标准。
[Effective Go](https://go.dev/doc/effective_go)用于命名、格式等惯用法，页面说明它始于 2009 年且未持续更新，
不覆盖模块、泛型等现代特性；这些内容以当前 Go 文档和所锁定 Go 源码为准。

## 新命令先校验，再产生副作用

自动化调用缺少参数时应立即失败。项目选择保留非交互运行：参数不足不询问、不自动分页，
不添加进度动画或默认 TUI。这是对 [CLI Guidelines](https://clig.dev/) 的输出、错误、参数和信号建议
结合智能体场景所作的取舍；该指南包含交互建议，并不要求所有 CLI 都使用本项目策略。

新增命令参照 `internal/app/cli/greet.go`：每次构造独立命令和局部 flag，Args 校验位置参数及
有类型业务选项，必填 flag 同时 `MarkFlagRequired`，RunE 调用业务并在成功后写结果。
根命令额外在配置入口前检查必填和组合 flag。Cobra 自带检查执行顺序不等于这里的副作用约束，
不能仅依赖它在 PreRun 之后的检查。

当前调用的核心形状如下，`ctx`、`localizer` 和校验器由调用入口提供：

```go
opts := greeting.Options{Name: "Codex", Localizer: localizer}
if err := opts.Validate(ctx, validator); err != nil {
    return err
}
result, err := greeting.New(validator).Run(ctx, opts)
if err != nil {
    return err
}
// result.Message 用于文本输出；result 的 JSON 字段为 message。
```

在 CLI 包内，greet 还通过以下真实调用登记必填参数与结果：

```go
greet.Flags().StringVar(&name, "name", "", "问候对象的姓名")
if err := greet.MarkFlagRequired("name"); err != nil {
    panic(err) // 固定命令定义错误，在构造时暴露。
}
if err := registerResult[greeting.Result](&call.results, greet, nil); err != nil {
    panic(err) // 结果结构无法登记，同样属于定义错误。
}
```

业务入口仍调用相同 Validate，避免直接调用绕过 CLI。新命令需补双语帮助、示例、参数错误测试、
成功结果和真实退出码测试，并用 `schema <command>` 核对必填标记。结果经 `registerResult[T]`
登记到本次树，由 jsonschema-go 生成 Draft 2020-12；特殊 JSON 编码需显式类型覆盖并验证真实 JSON。
format 不能替代业务语义校验。不要把输出写到一半后再执行可能失败的业务。

库依据：[Cobra v1.10.2](https://pkg.go.dev/github.com/spf13/cobra@v1.10.2)、
[pflag Changed v1.0.9](https://pkg.go.dev/github.com/spf13/pflag@v1.0.9#FlagSet.Changed)、
[jsonschema-go v0.4.3](https://pkg.go.dev/github.com/google/jsonschema-go@v0.4.3/jsonschema)。
Cobra 提供命令与元数据；前置顺序、schema 输出及退出码是项目选择。

## 配置使用严格的模块结构

要增加运行参数时，先在所属模块定义有类型 Config，组合到 `internal/app/config.Config`，
再扩展 defaults、环境映射、TOML 示例和校验测试。不要只加一个字符串 getter 或隐含环境开关。
通用执行器的 Dir、Env、捕获及并行选项保持有类型 API；doctor 的探测限额属于开发工具，
无需为填充模板配置而新增产品选项。

项目只读取一个 TOML，顺序为显式 flag > 非空环境变量 > 文件 > 默认值。`Changed` 表示用户
明确给过 flag，即使值恰好等于默认值也生效；未设置的 flag 不能覆盖文件。当前级别覆盖写法：

```go
var override *string
if cmd.Flags().Changed("log-level") {
    override = &logLevel
}
cfg, err := config.Load(config.Options{
    AppName: "go-cli-starter", Path: configPath,
    Explicit: cmd.Flags().Changed("config"), LogLevel: override,
    LookupEnv: lookupEnv, Context: cmd.Context(), Validator: validator,
})
```

加载器先严格解码 TOML 原始嵌套 map，拒绝 `"logging.level"` 这种字面键伪装嵌套字段；
随后 koanf 合并并整体解码。合法 TOML 点号语法 `logging.level = "info"` 会被解析成嵌套结构。
mapstructure 使用 `ErrorUnused=true`、`WeaklyTypedInput=false`、精确名称及受控 duration hook，
最后 validator 校验范围。不要把 koanf 的 StrictMerge 当作未知键或值域校验，也不要把 TOML 的
数字字符串自动转成数字。koanf 只可在精确的 `internal/app/config/load.go` 导入，业务只接收结构体。

例如 `[telemetry] batch_size = 4096` 在默认 queue_size=2048 下应失败；
`GO_CLI_STARTER_TELEMETRY_BATCH_INTERVAL=1s` 是显式的 duration 文本转换；
兼容 `GO_CLI_STARTER_LOG_LEVEL` 在新 `GO_CLI_STARTER_LOGGING_LEVEL` 之后应用，flag 最后应用。
空环境变量视为未设置。非法显式文件立即报错，默认文件缺失可继续；不自动迁移 YAML。

validator 实例可复用；项目在使用前完成自定义规则注册，之后只校验。
不要在并发调用期间变更规则。库依据：
[koanf v2.4.1](https://pkg.go.dev/github.com/knadh/koanf/v2@v2.4.1)、
[TOML v2.4.3](https://github.com/pelletier/go-toml/tree/v2.4.3)、
[mapstructure v2.4.0](https://pkg.go.dev/github.com/go-viper/mapstructure/v2@v2.4.0#DecoderConfig)、
[validator v10.30.5](https://pkg.go.dev/github.com/go-playground/validator/v10@v10.30.5)。

## 错误保留结构，文案在边界渲染

业务失败需要同时给自动化和用户诊断时，在所属模块定义稳定 fault.Code、Kind 及参数；
使用 Wrap 保留系统原因。项目不依赖错误字符串判断取消、文件缺失或子进程退出。
以下片段展示实际 fault API 的用法，`readErr` 是调用者取得的错误：

```go
problem := fault.Definition{Code: "example.read", Kind: fault.Runtime}
err := problem.Wrap(readErr, fault.WithParams(fault.Params{"Path": file}))
// errors.Is(err, os.ErrNotExist) 与 errors.As(err, &exitError) 仍能遍历原始原因。
```

新模块同时提供 `locales/active.zh-CN.json`、`locales/active.en.json` 与只读 Catalog。
入口把模块 Catalog 组合成 Bundle，冻结后为每次调用建立 Localizer：

```go
bundle, err := messages.NewBundle(greeting.Catalog(), command.Catalog())
if err != nil {
    return err
}
localizer := bundle.Localizer("en")
text := localizer.Text("greeting.message", map[string]any{"Name": "Codex"})
```

缺译返回可诊断的默认文案；项目不用 MustLocalize 处理运行错误。CLI 按 `--lang` >
`<PREFIX>_LANG` > zh-CN 选语言，zh 是别名，en 是英语，不从 OS 区域推断。
稳定命令、选项、JSON 字段、错误码和状态枚举不翻译；第三方错误叶子保留原文。
Help、version、completion、schema 在坏配置下仍可运行，不创建诊断目录。

退出码为成功 0、运行/配置 1、调用无效 2、调用者取消 130。stdout 只提交成功结果；
`--json` 为一个文档，序列化失败保持空输出，底层写失败可能留下部分字节。消费方先检查退出码。
错误和告警写 stderr；日志按独立出口写入。库依据：
[Go errors](https://pkg.go.dev/errors@go1.27.3)、
[go-i18n v2.7.0](https://pkg.go.dev/github.com/nicksnyder/go-i18n/v2@v2.7.0/i18n)。
错误分类、目录组织和语言优先级是项目契约；go-i18n 的 Bundle 在 Localizer 读取时不能继续修改。

## 日志、Trace 和异步工作的所有权

一次命令可能有业务 span、工具 span 和诊断文件。项目只在参数、配置校验后创建 Session，
以 run_id 串联命令生命周期。日志默认 info/file，写 `<state>/logs/<run_id>/events.jsonl`，
也可选 stderr/none；Trace 默认 none，file 独立写 `trace.jsonl`。file 是本地诊断 JSON 流，
不承诺 OTLP、采集服务接入或重放；无需修改全局 provider，也不会通过环境隐式启用 exporter。
SDK 环境仍有版本相关行为，不能将此概括为“忽略所有 OTEL_*”。

在已创建的调用中增加业务 span，参照 greet：

```go
ctx, span := call.session.Tracer.Start(cmd.Context(), "greeting.run")
defer span.End()
result, err := call.deps.Greet(ctx, greeting.Options{Name: name, Localizer: call.localizer})
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, call.localizer.Text("session.failed", nil))
    return err
}
call.logger.WithContext(ctx).Debugw(call.localizer.Text("cli.greet_log", nil), "event", "greeting_finished")
```

`WithContext` 只绑定 span 关联字段，logger 仍显式传递。重要结束事件使用
`Logger.Record(logging.CommandResult{...})` 或 `logging.ToolCall`，普通临时诊断才用 Debugw/Infow。
工具事件只记录操作、程序名、结果、耗时与结构化错误，不记录 argv、环境和原始输出。
新领域事件可实现 `logging.Event` 的 Emit 方法。日志门面没有 Fatal/Panic，业务不得绕过入口退出进程。

并发工作通过 `Session.Go(func(context.Context)) bool` 登记；Finish 开始后拒绝新任务。
工作须尊重 Context 并自行结束。Finish 幂等等待已登记工作，记录结束事件，结束根 span，
用 `context.WithoutCancel` 加独立 shutdown_timeout 排空 Trace，再 Sync 自有 logger、关闭自有文件。
不能在业务取消后直接复用已取消 ctx 进行导出，也不能丢下工作后提前关闭 writer。
借用 logger、reader、writer 不 Sync/Close；借用且共享的输出需由调用者提供并发安全包装。
CLI 自有 stderr 路径串行化错误、日志和告警，但不能替嵌入方接管独立的外部写入。
任意永久阻塞的自定义 IO 不保证硬截止，工作不结束也会阻止 Finish 返回。

资源创建、写入、导出或收尾故障按类别告警并继续业务，不覆盖业务结果；无可用出口时降级到空日志/Trace。
文件轮转软阈值默认 10 MiB，单条超大记录不拆分，默认 5 备份；按 14 天、200 MiB 软总量清理。后两项与备份数可设 0 关闭。
清理要求元数据、目录身份和普通文件清单有效，并成功取得运行系统锁；PID 或锁文件存在不证明活跃。
未知文件、符号链接、活跃运行保留，不递归删除目录；标记在数据删除后清除，中断后仍可识别。

库依据：[zap v1.28.0](https://pkg.go.dev/go.uber.org/zap@v1.28.0)、
[OTel SDK v1.47.0](https://pkg.go.dev/go.opentelemetry.io/otel/sdk@v1.47.0/trace)、
[stdouttrace WithWriter](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/stdout/stdouttrace@v1.47.0#WithWriter)、
[context](https://pkg.go.dev/context@go1.27.3)、[flock v0.13.1](https://pkg.go.dev/github.com/gofrs/flock@v0.13.1)。
库提供记录、导出、锁与取消机制；上述生命周期、失败继续策略和删除白名单是项目选择。

## 调外部工具时保留 argv、错误和取消原因

需要 Git 等外部程序时，使用 command.Spec 构建独立 argv，禁止拼 shell 字符串。
空参数、空格和 Unicode 必须保留。以下是可放入仓库内部包的实际 API 示例，省略 import：

```go
type revisionInput struct{ Revision string }
spec := command.Spec[revisionInput]{
    Operation: "git.revision", Program: "git",
    Args: []command.Arg[revisionInput]{
        command.Literal[revisionInput]("rev-parse", "--verify"),
        command.Value(func(p revisionInput) string { return p.Revision }),
    },
}
invocation, err := spec.Build(revisionInput{Revision: "HEAD"})
if err != nil {
    return err
}
execution := command.Execution{
    Dir: repoDir, Localizer: localizer, Logger: logger, Tracer: tracer,
}
result := command.Capture(ctx, invocation, execution, command.CaptureOptions{Limit: 2048})
if result.Err != nil {
    return result.Err
}
// 检查 result.StdoutTruncated，再使用原始 result.Stdout；这里不向 CLI stdout 写入。
```

Env=nil 继承父环境，空切片清空；WithEnv 复制并追加覆盖，不修改父选项。有效 PATH/PATHEXT 来自
本次环境，拒绝隐含当前目录命中（exec.ErrDot）。Run 流式输出借用 IO；Capture 只保留输入并独立捕获输出，
默认每流 1 MiB，达到上限继续排空，返回截断标记。错误保留 exec.ExitError、context 错误和 Cause。

批量使用 `command.RunBatch(ctx, calls, execution, command.BatchOptions{Concurrency: 4, ItemTimeout: 3*time.Second})`。
默认 concurrency=1、收集全部，按输入顺序返回；ItemTimeout 从取得名额后开始，零值不加超时。
FailFast=true 固定首先触发终止的原因并取消其他工作，收尾期间稍后的父取消不覆盖先前业务失败；
collect-all 遇到父取消仍中止批量。自身 deadline 记 Canceled+Err，尚未开始就因批量取消的项记 Skipped。
并发共用输入时调用方提供并发安全 reader，输出按项隔离。

Linux/macOS 的自有进程组清理普通后代并 Wait 直接子进程；未收养僵尸由 OS 回收。
setsid/detach 或交给外部服务的进程不在保证内。Windows 10/Server 2016+ 在创建时绑定私有 Job、
限制继承句柄，以挂起主线程准备监控后恢复；不支持嵌套 Job 时启动前失败。
Windows 终止失败先关闭 Job 再等待根进程，返回前验证活动进程清空，异常与超时保留错误。
WaitDelay 默认 250ms，控制取消终止及根进程退出后的管道排空宽限；永久阻塞的借用 IO 不保证被打断。

库依据：[os/exec](https://pkg.go.dev/os/exec@go1.27.3)、
[errgroup v0.24.0](https://pkg.go.dev/golang.org/x/sync@v0.24.0/errgroup)、
[Windows Job 创建属性](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)。
CommandContext 默认取消单个进程，并不承诺清理整棵树；errgroup.WithContext 的首错取消也不是 collect-all。
不要以库名或交叉编译成功替代上述平台原生 helper 验收。

## 让证据匹配交付声明

修改 Go 语义先用绑定实际 checkout 的 gopls 查符号、引用和诊断；等价普通操作按需用 CLI。
同 checkout 串行修改，禁止只调用工具填清单。测试应观察真实结果：临时 Git 仓库、真实子进程、
文件锁竞争、可控故障和取消。配置存在、进程启动、mock 返回成功都不足以证明安装或清理成功。

每个示例与测试同时隔离父、子进程的应用 HOME/配置/状态；本模板用 GO_CLI_STARTER_HOME，
改名后按 paths.EnvPrefix 派生，例如 mycli 使用 MYCLI_HOME。不要写真实用户日志。
检查只检查，安装由 setup 显式完成。固定工具表、18 lint、2 formatter 与规则 fixture 的运行入口见
[开发说明](development.md)；平台运行证据、发布摘要与托管设置见[维护清单](maintenance.md)。
Serena 和 Graphify 尚未集成，范围仅在[项目待办](backlog.md)，不能作为开发前置依赖。
