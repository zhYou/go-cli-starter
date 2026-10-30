# 项目协作约定

## 从这里开始

- 阅读 README.md，了解环境准备、CLI 约定和仓库结构；实现前按场景查阅
  [设计指南](docs/design-guidelines.md)，Go 官方文档和锁定版本源码是库行为依据。
- 默认使用中文交流，文档和注释使用中文；CLI 的帮助、日志和错误提供中文默认及英文目录。
  命令名、选项名、环境变量、JSON 字段、枚举值和协议标识保持原样。
- 在仓库根目录通过 mise 执行命令。先运行 `mise run setup`，再运行
  `mise run doctor`。首次使用及锁变更后运行 setup；日常检查不得隐式安装、升级或修复环境。
  需要 Git 和 mise 2026.10.21 或更新版本。
- 复用用户级 RTK 和 superpowers，遵循其已安装的说明，不在项目中复制安装，
  不写入个人路径。
- 当前只支持 Codex。个人模型、凭据、权限和插件偏好保留在用户配置中。
- 仓库文档记录当前能力、约束、使用和维护方法。superpowers 设计和实施计划只写入
  本地忽略目录 `docs/superpowers/`，不提交；交付时将已实现行为同步到正式文档。
- 后续工作统一记录在 [项目待办](docs/backlog.md)。待办条目不代表已授权实施；
  开始前确认任务范围，交付时更新状态、更新日期和关联 PR。已完成条目移除，历史通过 Git 追溯。

## 隔离开发与交付

- 默认在独立 worktree 中修改。同一任务已有隔离 worktree 时复用；否则优先使用工具原生
  worktree 能力，手动创建时放在主工作区根目录的 `.worktrees/` 下。仓库内的 worktree
  目录须被 Git 忽略。隔离失败时报告原因，不静默改在主工作区实施。
- 基线优先使用用户指定的引用；否则根据已有 PR 目标、任务上下文和 Git 分支关系确定。
  不能唯一确定时由用户指定，不直接回落到 main。开始时说明基线引用、提交和 PR 目标分支。
- 新分支命名为 `<类型>/<YYMMDD>-<主题>`，手动 worktree 目录命名为
  `.worktrees/<YYMMDD>-<类型>-<主题>/`。日期取创建时的 UTC 日期；类型使用
  `feat`、`fix`、`docs`、`refactor`、`test`、`build`、`ci`、`perf`、`chore` 或 `revert`；
  主题使用小写英文、数字和连字符。例如分支 `docs/261030-cleanup-history` 对应
  `.worktrees/261030-docs-cleanup-history/`。
- 创建前检查本地、远端分支和 worktree 是否重名；冲突时在主题末尾追加 `-2`、`-3`。
  同一任务续做沿用原名。工具托管且无法指定的目录名允许保留，交付分支仍遵守命名规则。
- 修改完成并通过必需检查后，提交、推送任务分支；已有本任务 PR 时更新同一 PR，否则按
  仓库 PR 模板创建指向目标基线分支的 PR，不重复询问交付方式。
- 创建 PR 后，报告 PR 链接、验证结果、任务分支和 worktree 位置，保留工作区用于审查修复；
  合并由维护者决定。
- 确认 PR 已合并、本地无未推送提交后，先检查未提交、未跟踪及被忽略文件，将需要保留的
  内容另存到该 worktree 之外，再进行清理。
- 清理时优先使用工具原生能力移除本任务 worktree，再依次删除本地任务分支及尚未自动删除的
  远端任务分支。仅清理本任务资源；遇到异常时报告原因，不强制删除。
- PR 关闭但未合并时保留分支和 worktree，待维护者明确放弃后，先完成上述文件检查和内容
  保留，再按上述顺序清理。
- 沿用现有 Git 作者身份，不修改个人 Git 配置。提交尾注记录实际执行工具，例如
  `Agent: Codex`、`Agent: Claude Code` 或 `Agent: OpenCode`。智能体执行开发和维护，
  维护者负责需求、合并和发布决策。

## 实现约定

- `cmd/<name>` 负责信号和退出；`internal/app/cli` 负责 Cobra、参数、输入输出和退出码；
  `internal/app` 组合配置、Session 和业务。`internal/base` 不依赖 app、tools、dev 或 Cobra；
  `internal/tools/command` 管外部进程；`internal/dev` 与根 tools 只服务开发维护。
  领域错误、消息、选项和结果由所属模块定义，不集中到 base 的全局表。
- 每次调用创建独立命令树，显式注入依赖。构造函数用于默认值、校验、依赖装配或资源建立，
  简单值直接使用字面量。稳定协议、状态和错误码使用语义常量，同值不同义不强行合并。
- 配置只在 `internal/app/config/load.go` 用 koanf 整体解码，按显式选项 > 非空环境变量 >
  唯一 TOML > 默认值解析；flag 覆盖前检查 `Changed`。拒绝未知键、弱转换、非法范围及
  字面点号键冒充路径；扩展模块的有类型配置、环境映射、示例和校验，禁止业务逐键读取容器。
- 命令仅凭参数完成运行，缺少输入立即失败。不添加交互提示、自动分页、进度动画或默认 TUI。
  Args、必填/组合 flag 校验在配置读取与资源创建前完成；业务直接调用也校验同一选项。
- 默认文本，`--json` 向 stdout 写一个完整渲染的 JSON 文档；执行或序列化失败时 stdout 为空。
  底层写失败可能留下部分字节，消费方先检查退出码。错误与告警写 stderr；日志默认 info/file，
  可显式选择 stderr 或 none。帮助是文本，补全是原生脚本，schema 始终是 JSON。
- CLI 语言按 `--lang` > `<PREFIX>_LANG` > zh-CN，zh 为别名、en 为英语，不读取系统区域。
  冻结各模块嵌入 JSON 消息目录后创建调用级 Localizer，稳定机器标识不翻译；第三方诊断可保留原文。
- 使用结构化 fault 保留领域码、分类、参数与原始原因，通过 errors.Is/As 检查错误链。
  退出码为 0 成功、1 运行/配置失败、2 调用无效、130 调用者取消；不在业务或日志中退出进程。
- Session 仅在校验完成后创建，拥有自建日志、根 span 和诊断文件。工作通过 Session.Go 登记；
  Finish 等待工作后记录结果、结束根 span、有界排空 Trace、同步日志并关闭自有文件。
  借用 logger 和 IO 不 Sync/Close。借用且共享的 writer 由调用者协调并发访问。
- Trace 默认 none，file 只写本地诊断 JSON，使用私有 provider，不隐式启用 exporter 或替换全局 provider。
  日志/Trace/诊断故障告警并继续业务；任意永久阻塞的借用 IO 不承诺硬截止。
- 外部工具使用 `internal/tools/command` 的独立 argv、显式 Dir/Env/IO/Localizer/Logger/Tracer。
  捕获上限后继续排空；默认每流 1 MiB，批量默认串行并收集全部结果，超时从取得并发名额后开始。
  工具错误不直接打印；入口统一本地化并提交成功结果。
- 新命令包含有类型选项和结果、前置校验、双语帮助/示例及行为测试，结果 schema 从本次命令树生成，
  必填选项用 `MarkFlagRequired`。`greet` 和 `internal/app/greeting` 可移除，替换时同步注册与测试。

## Go 工具与验证

- 使用 gopls MCP 查询 Go 符号、引用和诊断。通过
  `mise exec -- gopls mcp -instructions` 阅读说明，发现工作区、检查受影响符号，
  修改后检查诊断。确认查询绑定实际 checkout；等价普通操作按需使用 CLI，同一 checkout 串行修改。
  最终验证以命令行检查结果为准，不用配置成功、启动成功或 mock 代替真实验收。
- 测试和示例验证必须隔离父进程与子进程的应用 HOME、配置及状态，禁止读写真实用户诊断数据。
  本模板使用 `GO_CLI_STARTER_HOME`，改名后按应用前缀派生；不提交密钥、环境转储、日志或缓存。
- 编辑期间运行针对性测试。交付前运行 `mise run fmt` 和 `mise run check`，
  并在支持的主机上运行 `mise run test:race`。
- 修改模板或项目标识时运行 `mise run template:check`。
  新文件包含应用标识时同步更新受管文件清单。
- 修改工具集成时运行 `mise run mcp:check`，并在受信任的项目会话中确认
  `codex mcp list` 包含 gopls。
- 修改发布流程时运行 `mise run release:snapshot`，该命令不会发布。
  无法运行的检查须说明原因，不能声称通过；跨编译不代表 macOS/Windows 原生运行通过。
- 保留用户已有修改，只格式化授权范围；Git 写入失败先只读检查，禁止借机 reset、clean 或强推。

## 依赖与维护

- 使用官方最新 LTS；没有 LTS 的工具使用最新稳定版。
  排除 alpha、beta、RC 和 nightly，核验后锁定具体版本。
- 升级时保持 mise.toml、mise.lock、Go 模块与工具链声明、doctor 预期版本、
  CI 引导版本和文档版本一致。
- 锁定 Linux、macOS、Windows 的 amd64 和 arm64 工具版本。
  基础检查必须能在未安装 Codex、RTK 或 superpowers 时运行。
- 行为变更同步更新测试、帮助和文档。除非明确决定变更，否则保持机器字段和退出码兼容。
- 不把本地密钥、缓存或用户级智能体配置复制到生成的项目。
  初始化不得修改远端、创建提交或发布。
