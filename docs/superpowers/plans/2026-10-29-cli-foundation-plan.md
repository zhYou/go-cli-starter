# CLI 基础实施计划

> 面向智能体实施者：必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。

**目标：** 交付 BL-011/012/013/014，并接入 koanf、zap 和 validator。
**架构：** 独立错误、校验和日志包提供基础能力；CLI 只做参数适配和输出；业务通过构造函数注入依赖。
**技术栈：** Go、Cobra、koanf、zap、validator、Testify。
**设计：** [已确认设计](../specs/2026-10-29-cli-engineering-design.md)。

## 全局约束

遵循[实施总览](2026-10-29-cli-engineering-plan.md)全部约束及版本表。
保留 0/1/2/130、中文提示、配置优先级、独立命令树和发现命令绕过配置的行为。
每项新增内部导入都检查初始化白名单，Go 修改后先检查 gopls 诊断。

## 审查重点

- `TestBusinessErrorWithCustomPreRun`：hook 扩展不改变业务错误的退出码，任务 1。
- `TestJSONMarshalFailureLeavesOutputEmpty`：序列化失败保持空 stdout，任务 2。
- `TestGreetingDirectCallRejectsWhitespace`：绕过 CLI 也拒绝空白姓名，任务 3。
- `TestExplicitDefaultOverridesEnvironment`：显式默认值也覆盖环境，任务 4。
- `TestTwoInvocationsDoNotShareDependencies`：日志级别、选项和业务替身不串调用，任务 6。

## 文件结构

新增 `internal/apperror/{error,error_test}.go`、`internal/validation/{validation,validation_test}.go`、
`internal/logging/{logging,logging_test}.go`；CLI 新增 errors.go、output.go、dependencies.go、greet.go、version.go 及相应测试。
修改现有 cli.go、help.go、discovery.go、schema.go、config 和 greeting；每项同步 scaffold 受管清单及开发说明。

### Task 1: 错误分类与 Cobra 错误边界

**文件：** 新建 `internal/apperror/error.go`、`error_test.go`、`internal/cli/errors.go`、`errors_test.go`；修改 cli.go、help.go、discovery.go、schema.go、go.mod/go.sum 和 `internal/scaffold/scaffold.go`。
**输入：** 现有 `Run(context.Context, []string, Options) int` 和各命令 callback。
**输出：** `apperror.Kind` 的 Usage/Configuration/Runtime；`Error{Kind,Message,Cause}`；`New(Kind,string) error`、`Wrap(Kind,string,error) error`、`KindOf(error) Kind`；CLI 的 `exitCode(error) int`、`wrapRun(func(*cobra.Command,[]string)error) func(*cobra.Command,[]string)error`。

- [x] 引入总览锁定的 Testify，保留现有测试作为行为基线。
- [x] 写 `TestExitCodeAndUnwrap`：断言 Usage=2、Configuration/Runtime/普通业务错误=1、包装的 Canceled/DeadlineExceeded=130、nil=0；`errors.Is(wrapped,cause)==true`。写 `TestBusinessErrorWithCustomPreRun`：自定义 PreRun 后业务失败仍为 1。
- [x] 执行 `mise exec -- go test ./internal/apperror ./internal/cli -run 'TestExitCodeAndUnwrap|TestBusinessErrorWithCustomPreRun'`，确认因缺失分类能力失败。
- [x] 实现错误类型与适配：所有业务 RunE 的普通错误先由 wrapRun 标为 Runtime；配置/日志准备错误标为 Configuration；Args、flag 解析和剩余 Cobra 框架失败标为 Usage。取消优先匹配原因链。移除以 started 判定业务错误的逻辑，禁止匹配第三方英文错误字符串。
- [x] 运行上述测试及 `mise exec -- go test ./internal/cli ./internal/apperror`；确认必填标记、未知选项、未知命令、错误路径均保持 2。运行 `mise run template:check`。
- [x] 提交：`git commit -m 'refactor: 明确错误分类与退出码边界'`，仅暂存本任务文件。

### Task 2: 完整渲染后统一输出

**文件：** 新建 `internal/cli/output.go`、`output_test.go`；修改 cli.go、schema.go、discovery.go、help.go、README、AGENTS 和开发说明。
**输入：** 任务 1 的 Runtime 分类。
**输出：** CLI 包内 `writeJSON(io.Writer,any) error`、`writeText(io.Writer,func(io.Writer)error) error`；只在渲染成功后写出，短写使用 `io.ErrShortWrite`；每次 Run 创建 `recordingWriter`，提供 `Write([]byte)(int,error)` 和 `Err() error`。

- [x] 写 `TestJSONMarshalFailureLeavesOutputEmpty`：不可编码值返回错误且 `out.Len()==0`；`TestTextRenderFailureLeavesOutputEmpty`：渲染先写局部文本再报错，外部 stdout 仍为空；`TestShortWriteIsFailure`：`n<len(data),err=nil` 映射为 1；`TestHelpWriteFailureIsRuntimeError`：void 帮助 callback 的写错也通过 Run 返回 1。
- [x] 执行 `mise exec -- go test ./internal/cli -run 'TestJSONMarshalFailure|TestTextRenderFailure|TestShortWrite|TestHelpWriteFailure'`，确认失败来自缺失输出入口或现有帮助错误丢失。
- [x] 在 output.go 实现缓冲渲染和一次写出，JSON 追加一个换行；替换各命令分散编码。Run 以 recordingWriter 包装 stdout，执行返回 nil 时检查记录的写入错误，覆盖 SetHelpFunc 无返回值的边界；已存在的业务错误优先保留，不增加全局错误槽。
- [x] 更新文档：执行和序列化失败为空 stdout；底层写入失败可能已有部分字节，消费方先检查退出码。
- [x] 执行 `mise exec -- go test ./internal/cli`，验证文本/JSON、补全和 schema 形式兼容；`mise run template:check` 通过后提交 `refactor: 统一结果渲染与输出失败处理`。

### Task 3: 复用业务校验

**文件：** 新建 `internal/validation/validation.go`、`validation_test.go`、`internal/greeting/greeting_test.go`；修改 greeting.go、cli.go、go.mod/go.sum 和 scaffold 受管清单。
**输入：** `apperror.Usage`；总览中的 validator 版本。
**输出：** `validation.New() *Validator`、`(*Validator).Struct(context.Context,any) error`；`greeting.New(*validation.Validator) *Service`、`(*Service).Run(context.Context,Options)(Result,error)`、`Options.Validate(context.Context,*validation.Validator) error`。

- [x] 引入 validator，写 `TestGreetingDirectCallRejectsWhitespace`：空、空白姓名均产生 Usage；合法姓名保持原值且结果兼容。写 `TestValidationChineseFields`：字段提示能定位 `--name`，自定义条件通过 Go callback 报告。取消原因链可识别。
- [x] 执行 `mise exec -- go test ./internal/validation ./internal/greeting`，确认缺失校验和服务构造导致失败。
- [x] 实现每次构造的校验器，使用 `WithRequiredStructEnabled`；姓名使用 required/notblank，显示名由字段标签提供。注册在使用前结束。业务 Service 和 CLI Args 调用同一 Options.Validate；保留包函数 Run 为创建本地依赖的兼容入口，CLI 注入共享的 Service.Run。
- [x] 执行 `mise exec -- go test ./internal/validation ./internal/greeting ./internal/cli`；确认无效参数在配置加载或业务执行前失败，直接业务调用同样受保护。
- [x] 更新添加业务命令的示例及受管白名单，`mise run template:check` 后提交 `feat: 统一业务选项校验与中文提示`。

### Task 4: 严格配置接入 koanf

**文件：** 修改 `internal/config/config.go`、`config_test.go`、cli.go、go.mod/go.sum、scaffold 受管清单和开发说明。
**输入：** 任务 3 的 `*validation.Validator`。
**输出：** 保留 `config.Load(Options)(Config,error)`；Options 增加可选 Context 和 Validator；内部 `readYAML(io.Reader)(map[string]any,error)` 只返回实际存在且有效的值。

- [x] 引入 koanf/core 与 confmap。写 `TestExplicitDefaultOverridesEnvironment`：环境 debug、显式 warn 得 warn；未 Changed 的 warn 不覆盖环境。增加空环境、空文档、null、未知键、多文档、重复键、错误类型和两次调用隔离表测试，保留旧行为断言。
- [x] 执行 `mise exec -- go test ./internal/config ./internal/cli -run 'TestExplicitDefault|TestStrictYAML|TestConfiguration'`，记录兼容基线；已有实现满足的用例保持通过，不为配置库替换人为制造失败。
- [x] 用 yaml/v3 严格解码并保留字段存在性；koanf 分四层 Load(confmap)：默认、文件、非空环境、显式覆盖，最后按 yaml 字段标签解码有类型 Config 并调用 validator。禁止弱类型转换掩盖错误，null 的处理沿用旧解码语义。
- [x] 执行 `mise exec -- go test ./internal/config ./internal/cli`、`mise run template:check`；确认损坏配置不阻断发现命令。
- [x] 提交 `refactor: 使用 koanf 保持严格配置覆盖规则`。

### Task 5: 每次调用的 zap 日志

**文件：** 新建 `internal/logging/logging.go`、`logging_test.go`；修改 cli.go、go.mod/go.sum、scaffold 受管清单。
**输入：** 任务 4 的有效日志级别和调用者 ErrOut。
**输出：** `logging.New(io.Writer,string)(*zap.Logger,error)`；无全局状态、无 Close 所有权；console 编码，四个现有等级。

- [x] 引入 zap，写 `TestLogLevelsAndSinkOwnership`：warn 屏蔽 debug、debug 可见，Sync 不关闭外部 writer；`TestDebugLogsDoNotPolluteJSON` 保持 stdout 是一个有效 JSON 文档；未知级别返回错误。
- [x] 执行 `mise exec -- go test ./internal/logging ./internal/cli -run 'TestLogLevels|TestDebugLogs'`，确认新日志行为先失败。
- [x] 用 zapcore 创建 console core，封装调用者 writer 的无缓冲、非拥有式 Sync；配置加载后构造 logger。使用注入实例记录中文消息，保留稳定字段，不调用 zap.ReplaceGlobals。
- [x] 执行 `mise exec -- go test ./internal/logging ./internal/cli`，确认无 `slog` 全局状态或新日志格式选项。
- [x] 更新日志说明及受管清单，提交 `refactor: 使用独立 zap logger 输出诊断`。

### Task 6: 命令构造与依赖装配

**文件：** 新建 `internal/cli/dependencies.go`、`dependencies_test.go`、`greet.go`、`version.go`；修改 cli.go、cli_test.go、scaffold 受管清单及开发说明。
**输入：** 任务 1—5 的错误、输出、校验、加载和日志接口。
**输出：** `Dependencies{LoadConfig func(config.Options)(config.Config,error), Validator *validation.Validator, Greet func(context.Context,greeting.Options)(greeting.Result,error), NewLogger func(io.Writer,string)(*zap.Logger,error)}`；Options 增加 `Dependencies *Dependencies`；`newGreetCommand(Options,*invocation)*cobra.Command`、`newVersionCommand(Options,*invocation)*cobra.Command`。invocation 持有本次输出模式、已初始化依赖和 logger。

- [ ] 写 `TestTwoInvocationsDoNotShareDependencies`：两个 root 的 flag、level 和业务替身独立；`TestInvalidArgsNeverLoadConfigOrRunBusiness`：错误参数的调用计数均为 0；`TestDiscoveryNeverLoadsConfiguration`：四类发现入口加载次数为 0。
- [ ] 执行 `mise exec -- go test ./internal/cli -run 'TestTwoInvocations|TestInvalidArgsNever|TestDiscoveryNever'`，确认缺少注入能力导致失败。
- [ ] 从 defaults/newRoot 拆出依赖装配和命令工厂；复制传入依赖再补默认值，避免修改调用者对象。保留 NewRoot 和 Run 对外签名，根构造只注册命令和公共准备流程。
- [ ] 运行 `mise exec -- go test ./internal/cli ./internal/config ./internal/greeting ./internal/logging ./internal/validation ./internal/apperror`，再执行 `mise run fmt`、`mise run check`、`mise run test:race`。
- [ ] 核对 BL-011/012/013/014 的完整验收，提交 `refactor: 拆分命令构造并显式注入依赖`，按实际交付更新待办和 PR。
