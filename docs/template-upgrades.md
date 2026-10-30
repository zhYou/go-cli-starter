# 模板升级

模板版本在 `.template-version`，当前为 0.3.0，与应用发布版本独立。新副本初始化生成
`.template-origin.json`，记录原模板、版本、目标身份及可确认的来源提交。未知来源使用 unknown，
不拿派生项目 HEAD 冒充来源。同参初始化保留来源字节；没有元数据的旧项目仍可构建检查。
初始化仅用于原始模板改名，不覆盖已有业务、不自动升级布局或配置。

## 升级前完成旧事务

在旧 checkout、旧工具和原参数下，先完成所有未终结的 `.template-init/` 事务，涵盖所有历史支持格式，
包括 format 2；不能仅因为新旧都叫 format 2 就用新工具续做。迁移受管文件路径会使旧 journal
不再对应新白名单。格式 1 同样先用旧工具恢复。备份业务及恢复现场，确认终态清理完成再合并升级。

相同参数重试先恢复；参数不同、摘要冲突、人工修改、未知内容或损坏日志会停下并保留现场。
命令目录移动只接受受管入口文件；额外文件、既有目标目录和不明确目录状态均拒绝，不覆盖或猜测。
不要删除锁或 journal 绕过保护。`.template-init.lock` 是常驻锁文件，是否占用取决于系统锁；
单文件同目录临时替换不等于跨文件/断电原子性。持久化提交完成或回滚完成终态后才逐个清备份，
最后删 journal；清理中断可继续，未知内容保留。临时路径由受管 journal 唯一推导，不能按前缀批量删文件。

## 历史布局迁移

0.1.0 引入校验、日志、输出和结果 schema；保留业务选项、结果及退出码，业务直接调用也校验。
0.2.0 将根入口移到 `cmd/<name>/`、配置示例移到 `configs/`、进程测试移到 `test/integration/`、
共享契约移到 `test/testdata/contracts/`。源码运行和安装改用 `./cmd/<name>`；更新静态入口识别、
受管清单、GoReleaser main 和测试。目录移动本身不要求修改机器接口。

命令目录与应用名一致，模块末段可不同。初始化提前拒绝 v2、v10 等无前导零且至少为 2 的 Go
版本段名称，防止 go install 产物名偏离目录；v0、v1、v02 可用。新增源码若含身份，显式登记白名单。
历史快照缺失时可读取旧 Git 路径，损坏文件和 Git 错误不能当作“没有基线”。

## 迁移到 0.3.0

1. 在干净隔离分支备份业务及来源记录，按上节完成旧事务，再选择已审查的模板提交人工合并。
   迁移到 app/base/tools/dev 分层：业务和 CLI 在 app，共用机制在 base，进程在 tools/command，
   初始化与契约检查在 dev。更新导入、受管清单、测试和工具；不要只搬目录而遗漏资源生命周期。
2. 将旧 `config.yaml` 的 `log_level` 改为唯一 `config.toml` 的 `[logging] level`，
   发布归档根示例改为 `config.example.toml`。不再自动读取、合并或静默转换 YAML。
   新加载器拒绝未知键、弱转换及非法范围；显式 flag > 非空环境变量 > TOML > 默认值。
   旧 `<PREFIX>_LOG_LEVEL` 继续兼容且优先于新 `<PREFIX>_LOGGING_LEVEL`，flag 仍最优先。
3. 默认日志由 warn/stderr 改为 info/file。需要旧出口时显式配置：

   ```toml
   [logging]
   level = "warn"
   output = "stderr"
   ```

   none 可关日志；Trace 默认 none，file 仅本地诊断 JSON，非 OTLP/重放格式。
   合并 Session 的登记、等待与关闭顺序，自有资源由 Session 收尾，借用 logger/IO 不 Sync/Close。
4. 核对私有目录及权限。Linux 使用 XDG，macOS 使用 Application Support/Caches，Windows 使用
   APPDATA/LOCALAPPDATA。`GO_CLI_STARTER_HOME` 为绝对应用根，改名 mycli 后为 `MYCLI_HOME`；
   配置在根下 config.toml，data/state/cache 分开，与安装位置无关。为测试父子进程隔离此变量。
5. 新增 `--lang` 与 `<PREFIX>_LANG`，按 flag > 环境 > zh-CN，zh 是别名、en 是英语。
   合并模块双语 JSON 目录与冻结 Bundle；不读取系统区域，不翻译机器标识、字段及退出码。
6. 审查[本次兼容记录](compatibility/changes/2026-10-30-cli-foundations.json)：
   schema_version 仍为 1，12 项机器快照差异为各命令 lang 与 info 默认值；
   TOML、日志落盘和路径另有手工迁移。应用 v0.1.0→v0.2.0 是兼容决策，本次不创建发布标签。
   模板来源记录和初始化事务格式不因分层迁移升级，不运行 init 覆盖业务。
7. 暂存移动及新增文件，显式 setup/doctor 后运行 fmt、check、race；涉及 MCP 时运行 mcp:check。
   提交最终源码后重新 snapshot 与 release:verify，核对六归档、TOML、SPDX、最终摘要及宿主安装。
   macOS/Windows 运行行为以对应原生 CI 为准。记录采用的模板版本与提交，保留来源历史和个人配置外置。

更多实际 API、示例和所有权边界见[设计指南](design-guidelines.md)，托管安全设置见[维护清单](maintenance.md)。
