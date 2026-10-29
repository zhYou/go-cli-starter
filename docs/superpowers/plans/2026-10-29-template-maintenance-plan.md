# 初始化恢复与模板维护实施计划

> 面向智能体实施者：必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。

**目标：** 交付 BL-010/016，初始化故障可恢复，派生项目具备来源和迁移路径。
**架构：** Afero 隔离文件操作，OS 文件锁保护事务，恢复日志记录原/目标摘要；来源信息作为最后应用的受管内容。
**技术栈：** Go、Afero、gofrs/flock、Testify、真实文件系统子进程测试。
**设计：** [已确认设计](../specs/2026-10-29-cli-engineering-design.md)。前置为第一批；最终模板检查同时包含第二批产物。

## 全局约束

遵循[实施总览](2026-10-29-cli-engineering-plan.md)全部约束及版本表。
保持原 Initialize API、白名单、路径校验、权限和幂等行为；不修改 remote、提交或发布。
不同目标参数、人工修改或损坏恢复记录均不能触发无条件覆盖。

## 审查重点

- `TestConcurrentInitializationDoesNotRecoverLiveTransaction`：活跃进程持锁时立即失败，任务 10。
- `TestRecoverBeforeIdentityCheck`：go.mod 已改、main.go 未改时仍可按相同参数恢复，任务 10。
- `TestRecoveryRejectsEditedOrEscapingPaths`：用户修改、绝对/上层路径或符号链接均保留现场，任务 10。
- `TestRollbackFailureKeepsRecoveryData`：回滚也失败时不删除备份，任务 10。
- `TestOriginWithoutGitAndLegacyProject`：复制包缺少 Git 来源可用，旧项目缺少元数据仍可验证，任务 11。

## 文件结构

新增 `internal/scaffold/filesystem.go`、`transaction.go`、`origin.go` 及对应测试。
修改 scaffold.go 作为身份与受管文件入口、tools/init 作为 CLI 适配。
新增根目录 `.template-version`、初始化生成的 `.template-origin.json` 和 `docs/template-upgrades.md`。

### Task 10: 文件适配、互斥与恢复事务

**文件：** 新建 `internal/scaffold/filesystem.go`、`filesystem_test.go`、`transaction.go`、`transaction_test.go`；修改 scaffold.go、scaffold_test.go、tools/init/main.go、go.mod/go.sum、.gitignore、tools/dev/template.go。
**输入：** 现有 Identity、参数规则和受管白名单；Afero 与 flock 的总览版本。
**输出：** `Initializer{FS afero.Fs, RunCommand func(context.Context,string,...string)([]byte,error), Lock func(string)(func()error,error)}`；`(*Initializer).Initialize(ctx context.Context,root,module,name string)([]string,error)`；保留包函数 `Initialize(root,module,name string)([]string,error)`，装配真实依赖后调用。模块路径预检使用 `go -C <临时目录> mod init <模块>`，真实 runner 固定 `GOWORK=off`。

- [x] 引入 Afero 和 flock；写故障文件系统包装器，仅包装必要 Write/Rename/Chmod/Remove 操作，不给生产程序增加测试开关。
- [x] 写上述四个恢复/并发重点测试，断言第 N 次写入失败后旧文件逐字恢复；恢复前检查原/目标摘要，摘要不符必须错误且用户文件不变。使用受控子进程真实持锁/退出，确认进程结束后锁自动释放。
- [x] 执行 `mise exec -- go test ./internal/scaffold -run 'TestConcurrentInitialization|TestRecoverBeforeIdentity|TestRecoveryRejects|TestRollbackFailure'`，确认当前逐文件写入和身份检查顺序不能满足新要求。
- [x] 实现 `.template-init.lock` 的非阻塞 OS 锁和 `.template-init/` 恢复目录。先成功取得锁，再读取旧事务；仅目录排他创建不能证明旧进程已停止。锁文件保留并忽略，避免删除锁文件导致两把锁并存。
- [x] 实现 format_version=1 的 journal，包含目标/原身份、阶段、文件相对路径、mode、before/after SHA-256 和备份位置。阶段为 prepared/applying/committed；预先准备全部数据和备份，journal 完整写好才改目标。对原本不存在的文件记录存在性，回滚时删除新建文件而非恢复空文件。
- [x] 普通失败回滚；崩溃后相同参数在身份检查前恢复，确认全部目标仍匹配 before/after 后回滚并重做。prepared 不应有受管写入；committed 只清理现场；记录损坏、参数冲突或人工修改则停止。使用同目录临时文件替换单文件，不承诺全文件系统断电原子性。
- [x] 对真实路径与 journal 路径保留根目录、白名单、Lstat/EvalSymlinks 检查；Afero 内存测试不能替代这些检查。同步 Go 注释中“仅标准库”的旧表述，并从模板复制和发布中排除恢复目录及锁。
- [x] 执行 `mise exec -- go test ./internal/scaffold ./tools/init ./tools/dev`、`mise run template:check`、`mise run test:race`，三平台 CI 保留真实 FS 测试；提交 `feat: 为模板初始化增加互斥与失败恢复`。

### Task 11: 模板来源、版本与迁移

**文件：** 新建 `.template-version`、`internal/scaffold/origin.go`、`origin_test.go`、`docs/template-upgrades.md`；修改 transaction.go、scaffold.go、tools/dev/template.go、template_test.go、README 和开发说明。
**输入：** 任务 10 的事务和“文件原本是否存在”记录；`.template-version` 首版值 `0.1.0`，与应用版本独立。
**输出：** `Origin{FormatVersion int, Template string, TemplateVersion string, SourceCommit string, Module string, Name string}`；JSON 字段依次为 format_version、template、template_version、source_commit、module、name；`ReadOrigin(afero.Fs,string)(Origin,error)`、`makeOrigin(root,module,name string)(Origin,error)`。

- [x] 写 `TestOriginWithoutGitAndLegacyProject`：SourceCommit 缺失为 `unknown`，旧项目缺少来源记录仍可 build/check；`TestOriginIsIdempotent`：再次同参初始化记录逐字不变；`TestOriginFailureRollsBackRename`：来源写入失败不留下部分改名。在 tools/dev/template_test.go 新增 `TestTemplateMetadataAndGuidesSurviveRename`，验证来源元数据及升级指南在新项目中有效。
- [x] 执行 `mise exec -- go test ./internal/scaffold ./tools/dev -run 'TestOrigin'`，确认缺失来源机制时失败。
- [x] 将 `.template-origin.json` 作为事务最后应用的文件，Template 使用原始 TemplateModule 来源标识，不能被目标名称替换。SourceCommit 仅在可确定原模板来源时记录；旧定制项目不根据自己的 HEAD 伪造来源。未知来源可继续工作，不回填猜测数据。
- [x] 明确身份已匹配时的行为：已有记录保持字节幂等；旧项目无记录仍报告未知来源；来源身份明确的新模板第一次初始化才创建记录。原始模板身份与来源标识的固定内容不进入目标名称替换。
- [x] 为本次 0.1.0 迁移写具体说明：CLI 执行基础、Schema 增量、开发任务、发布与托管配置；标清业务示例如何人工合并，不提供自动覆盖命令。更新受管清单及模板副本检查，使新副本的元数据、契约 fixture、模块导入和工具配置均独立有效。
- [x] 执行 `mise run fmt`、`mise run check`、`mise run test:race`、`mise run template:check`；提交 `feat: 记录模板来源并提供升级迁移路径`，按验收更新 BL-010/016。
