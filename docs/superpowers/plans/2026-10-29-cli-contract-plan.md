# Schema 与真实进程测试实施计划

> 面向智能体实施者：必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。

**目标：** 交付 BL-005/007/015，公开 JSON 结果结构并保护实际程序行为。
**架构：** Result 类型生成独立 JSON Schema；每次命令树持有结果注册表；工具任务维护契约基线；子进程测试覆盖真实退出。
**技术栈：** google/jsonschema-go、go-cmp、testscript、testing、Testify。
**设计：** [已确认设计](../specs/2026-10-29-cli-engineering-design.md)。前置为第一批任务 1—6。

## 全局约束

遵循[实施总览](2026-10-29-cli-engineering-plan.md)全部约束及版本表。
保持 `schema_version: 1`，只新增可选 `result_schema`；Schema 不执行业务、不读取配置或网络。
Draft 2020-12；消费方忽略新增字段，结果对象允许额外属性，配置未知键仍失败。

## 审查重点

- `TestResultSchemaMatchesEncoding`：nil/空切片、omitempty、指针、自定义编码与实际 JSON 一致，任务 7。
- `TestSchemaGenerationDoesNotExecuteCommands`：业务、配置和外部加载的计数均为 0，任务 7。
- `TestBaselineRefreshStillRequiresChangeRecord`：重写当前快照不能绕过相对 PR 基线的变化，任务 8。
- `TestProcessExitCodes`：构建后二进制返回真实 0/1/2，任务 9。
- `TestMainSignalCancellation`：等待明确就绪后取消，130 且无业务结果，任务 9。

## 文件结构

新增 `internal/contract/{schema,snapshot}.go` 及测试、`internal/cli/results.go`、
`tools/dev/contracts.go` 及测试、`testdata/contracts/v1.json` 和 `docs/compatibility.md`。
进程测试放在 `internal/cli/process_test.go` 和 `main_test.go`；脚本放在 `internal/cli/testdata/process/`。

### Task 7: 结果类型注册与 Schema 生成

**文件：** 新建 `internal/contract/schema.go`、`schema_test.go`、`internal/cli/results.go`、`schema_test.go`；修改 schema.go、greet.go、version.go、cli.go、go.mod/go.sum、scaffold 白名单、README 和开发说明。
**输入：** 第一批命令工厂、writeJSON 和有类型 greeting.Result；version 的匿名结果改为 `versionResult`，字段及 JSON 标签完全保留。
**输出：** `contract.FromType(reflect.Type,json.RawMessage)(json.RawMessage,error)`，第二参数为可选显式覆盖；`resultRegistry` 保存命令指针对应的类型/覆盖；`registerResult[T any](*resultRegistry,*cobra.Command,json.RawMessage) error`；`newSchemaCommand(*cobra.Command,*resultRegistry)*cobra.Command`。

- [x] 引入总览锁定的 google/jsonschema-go 和 go-cmp。
- [x] 写 `TestResultSchemaMatchesEncoding`：对实际 json.Marshal 结果验证，断言 message 必填、nil 切片允许 null、空切片允许 array、omitempty 可缺省、自定义类型使用显式覆盖；不支持的递归类型无覆盖时明确失败。写 `TestSchemaGenerationDoesNotExecuteCommands`：配置和业务调用数为 0。
- [x] 执行 `mise exec -- go test ./internal/contract ./internal/cli -run 'TestResultSchema|TestSchemaGeneration'`，确认新能力缺失时失败。
- [x] 实现 FromType：默认严格推导、明确 Draft 2020-12、为 Go struct 对应对象开放 additionalProperties，保留 map 的元素约束；显式覆盖必须先解析验证。注册表属于本次 root，schema 增加 `ResultSchema json.RawMessage` 的 `result_schema,omitempty` 字段；greet/version 注册，帮助/补全/schema 自身不注册结果。
- [x] 用 go-cmp 检查规范化结构；额外字段验证可通过，删除 message 或改成数字必须失败；发现入口损坏配置测试继续通过。运行 `mise exec -- go test ./internal/contract ./internal/cli` 和 `mise run template:check`。
- [x] 提交 `feat: 从结果类型公开 JSON Schema`。

### Task 8: 机器接口基线与变更记录

**文件：** 新建 `internal/contract/snapshot.go`、`snapshot_test.go`、`tools/dev/contracts.go`、`contracts_test.go`、`testdata/contracts/v1.json`、`docs/compatibility.md`、`docs/compatibility/changes/2026-10-29-result-schema.json`；修改 tools/dev/main.go、mise.toml、CI、go.mod/go.sum、scaffold 白名单和开发说明。
**输入：** 任务 7 的实际命令描述、Result Schema 和第一批 Run；基线应用名称固定 `contract-cli`，版本字段固定测试值。
**输出：** `contract.Change{Path string,Kind string,Before json.RawMessage,After json.RawMessage}`、`Diff([]byte,[]byte)([]Change,error)`；工具侧 `captureContract(context.Context)([]byte,error)`、`checkContract(context.Context,string) error`、`updateContract(context.Context) error`。check 的字符串为可选 Git 基准引用，默认比较仓库当前快照。新增 `runArgs([]string) error` 解析工具参数，保留 `run(task string) error` 为单任务兼容包装。

- [x] 从设计确认时的代码版本 54d9dcd 在临时副本构建，捕获初始命令与结果基线；不回退工作树。模板副本使用提交的规范化 fixture，不依赖该历史提交。变更记录含 id、compatibility、summary、paths、previous_version、target_version、migration；首次基线 previous_version 为 null，明确记录允许新增 result_schema。
- [x] 写 `TestBaselineRefreshStillRequiresChangeRecord`：旧字段由字符串变数字，即使当前快照同步更新，相对旧基线仍失败；缺少记录失败；已记录的 result_schema 增量可通过。写 `TestContractNormalization`：人类描述文案不作为字段类型，机器标识和必填标记必须保留。
- [x] 执行 `mise exec -- go test ./internal/contract ./tools/dev -run 'TestBaselineRefresh|TestContractNormalization'`，确认新增差异检查先失败。
- [x] 引入总览锁定的 x/mod，实现确定性 JSON 差异及 change record 校验，已知删除/类型变化标记为可能破坏兼容；禁止把它们直接声明为兼容。复杂 Schema 变化输出差异并要求审查，不实现通用兼容性推理器。用 semver 比较记录中的发布版本：0.x 破坏变更提升次版本，1.x 以后提升主版本；首次基线不虚构旧发布版本。
- [x] 增加 `contract:check` 和显式 `contract:update`；check 不改文件，接入 mise check。tools/dev main 通过 runArgs 允许 contract-check 的 `--against`，其他多余参数立即中文报错。CI 另用 `mise run contract:check -- --against <base SHA>` 对 PR base / push before 比较，fetch 足够历史；首个基线不存在时要求明确的初始化记录。Git 参数逐项传递，模板临时仓库默认检查不要求已有 HEAD 或 remote。
- [x] 运行 `mise run contract:check`、上述测试及 `mise run check`；用测试 fixture 重现刷新快照仍失败；提交 `feat: 检查机器接口基线与兼容变更记录`。

### Task 9: 真实二进制与信号测试

**文件：** 新建 `internal/cli/process_test.go`、`internal/cli/testdata/process/{success,usage,configuration,discovery}.txtar`、`main_test.go`；修改 main.go、go.mod/go.sum、CI、scaffold 白名单、README 和开发说明。
**输入：** `cli.Options.Dependencies` 和任务 1 的错误映射。
**输出：** 根入口 `run(args []string,deps *cli.Dependencies,notify func()(context.Context,context.CancelFunc)) int` 与 `signalContext()(context.Context,context.CancelFunc)`；main 只调用 run 后 os.Exit。testscript 使用构建后的真实程序。

- [ ] 引入总览锁定的 go-internal/testscript。写 `TestProcessExitCodes`：真实成功=0、显式损坏配置=1、无效调用=2，失败 stdout 为空；测试临时 HOME/USERPROFILE 和配置目录隔离，PATH 只加入 fixture 程序。
- [ ] 写 `TestMainSignalCancellation`：根包测试子进程注入等待 context 的 Greet，发出“就绪”后再发实际 Unix 信号；期待退出 130、无业务 stdout。Windows 不能发送等价信号时说明具体原因，并通过注入 notify 的受控取消子进程覆盖同一 run 入口。测试辅助分支只存在于 _test.go。
- [ ] 执行 `mise exec -- go test . ./internal/cli -run 'TestProcessExitCodes|TestMainSignalCancellation'`，确认缺少入口依赖注入或取消场景时失败；不使用抢发信号碰运气。
- [ ] 把既有信号生命周期提取为上述根入口函数，生产路径仍使用 os/signal；testscript 场景调用真正构建的程序，脚本独立于 bash，异常状态精确检查 ProcessState.ExitCode，不用 go run 代替。
- [ ] 在 Linux 执行 `mise exec -- go test . ./internal/cli`、`mise run fmt`、`mise run check`、`mise run test:race`；macOS/Windows CI 执行同一测试，明确保留平台跳过原因。
- [ ] 提交 `test: 验证真实进程退出输出与取消行为`，按验收更新 BL-005/007/015。
