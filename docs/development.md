# 开发说明

默认使用中文交流，文档、注释、帮助和项目提供的提示也使用中文。命令、选项、环境变量、JSON 字段及枚举值保持稳定。当前 CLI 仅提供中文，不读取系统语言设置；未来国际化见 [项目待办](backlog.md) 中的 BL-001。

## 协作与文档

修改默认在隔离 worktree 中进行，完成验证后提交、推送并创建 PR。基线选择、命名和提交署名
遵循 [协作约定](../AGENTS.md#隔离开发与交付)。

仓库文档记录当前能力、约束、使用和维护方法。superpowers 设计和实施计划只保留在本地忽略
目录 `docs/superpowers/`，交付时将已实现行为同步到正式文档；历史过程通过 Git 追溯。

## 添加命令

1. 参考 greet，创建接收 context 和选项的业务函数，返回带 JSON 标签的结果。
2. 注册参数，在 Args 阶段检查数量和值，必填参数同时调用 MarkFlagRequired。
3. 显式传入依赖，避免修改包级配置或日志对象。
4. 根据 --json 选择输出方式，错误直接返回，由入口统一打印和映射退出码。
5. 注册根命令，补帮助、示例和行为测试。运行 schema 新命令核对自动生成的说明。
6. 运行 mise run fmt 和 mise run check。

新增配置字段时同步修改结构、环境变量覆盖、参数覆盖、校验、示例和测试。通过 Changed 检查显式 flag，避免参数默认值覆盖环境或文件配置。

greet 和对应业务包是可删除示例，替换时同步修改测试。帮助、version、completion 和 schema 始终能在配置损坏时使用。

## 修改模板初始化

初始化工具维护文件白名单，不扫描替换整个仓库。新文件包含模块导入、命令名或环境变量前缀时，将它加入白名单。保留初始化实现中的原模板身份及测试 fixture，避免新项目重复初始化时误判。

mise run template:check 使用 Git 文件清单复制项目，跳过被忽略的缓存、产物和本地配置。它验证独立项目的改名、幂等性、测试、构建、schema 和发布配置；在已初始化项目中验证现有身份。

## 更新工具

有官方 LTS 时选择最新 LTS 补丁版，否则选择最新稳定版。核对 Go、gopls 和 golangci-lint 的兼容要求，再同步 mise、Go 模块和工具链、doctor 检查、README、CI 和相关稳定版 Actions。

```sh
mise lock --platform linux-x64,linux-arm64,macos-x64,macos-arm64,windows-x64,windows-arm64
mise run setup
mise run fmt
mise run check
mise run test:race
mise run mcp:check
mise run release:snapshot
```

提交工具锁文件。日常 setup 不更新版本或锁文件。RTK、superpowers 和 Codex 使用用户级安装，维护者通过各自官方稳定渠道更新，仓库不修改全局配置。

## 验证与发布

测试使用注入 IO、临时配置目录和可控环境变量，避免读取用户配置或请求业务网络。输出测试检查可解析 JSON 和独立 stderr。

check 不改写源码，但会写 Go 缓存、临时测试文件和 bin 下的产物。fmt 和 setup 是显式修改或安装操作。本地 snapshot 需要 Git 仓库，不需要发布令牌；创建 Release 由目标项目的标签 workflow 完成。

## 维护待办

后续需求集中记录在 [项目待办](backlog.md)，不分散创建多个状态清单。新增条目分配唯一 ID，填写优先级、状态、工作量和日期，并说明背景、范围及验收条件。

待办不代表已授权实施。确认范围后再开始，开始时更新为进行中；交付时更新状态、更新日期并关联 PR。暂缓条目保留，已完成条目移除，历史记录通过 Git 追溯。

结果经 writeJSON/writeText 完整渲染后写出。执行和序列化失败保持 stdout 为空；底层写入失败可能已有部分字节，调用者应先检查退出码。

业务选项通过 `validate` 与 `label` 标签声明规则，CLI Args 和业务入口调用同一个 Validate 方法。`validation.New` 的可选注册 callback 在实例使用前配置自定义和条件规则；不要在并发调用后修改注册。

配置由严格 YAML 解码和 koanf 分层合并；环境变量为空时忽略，YAML null 沿用默认值，显式选项仅在 Changed 时覆盖。禁止弱类型转换。

每次调用创建独立 zap logger，默认 warn，诊断写 stderr；logger 不关闭调用者 writer，也不替换全局实例。

命令使用独立构造函数，依赖由 Options.Dependencies 显式注入；装配复制依赖并补默认值，每次调用不共享选项、配置或日志。测试可替换加载器、业务函数与 logger 工厂。

有类型的结果通过 registerResult 注册到本次命令树，使用 google/jsonschema-go 生成 Draft 2020-12。json.Marshaler、encoding.TextMarshaler、字节切片、json string 标签和递归类型须显式覆盖并验证实际 JSON；format 不提供语义校验，业务约束交给 validator。

`test/integration` 的进程测试构建真实二进制，用 testscript 验证 0/1/2、输出流与损坏配置下的发现命令。`cmd/<name>` 入口测试等待就绪后发 Unix 中断，检查真实退出 130；Windows 使用注入 notify 的受控取消，辅助分支只在测试代码中。

初始化持有 `.template-init.lock` 的系统锁；失败时根据 `.template-init/` 日志回滚。相同参数重试会先恢复，不同参数或人工修改会保留现场并报错。不要删除活跃进程的锁文件；锁文件常驻。单文件以同目录临时文件替换，不承诺跨文件或断电原子性。

工具升级后更新 mise.toml、mise.lock、Go 声明、README 当前工具表及 CI 引导版本，再运行 versions:check。二进制工具校验六平台归档；Go 后端锁定同一模块来源版本，跨平台本地编译，不生成虚假的平台归档记录。Renovate 配置需由仓库管理员启用；受信任执行权限不足时维护者自行补齐六平台锁，检查不会放宽。

发布使用固定 Syft 对六个本地归档生成 SPDX JSON，配置关闭 enrichment、远程许可证查询和更新检查；GoReleaser 在 SBOM 后生成最终 checksums.txt。release:verify 校验实际文件，不上传证明。发布证明只在受信任 release job 生成。

提交 PR 时使用仓库模板记录实际行为、兼容记录和验证。安全报告及维护责任按[维护清单](maintenance.md)配置；托管权限和平台功能须由管理员实际启用。

提交完成或回滚完成后先持久记录终态，再逐个清理备份，最后删除 journal。清理中断可继续；仅剩空恢复目录时安全移除，未知内容仍保留并报错。


## 目录与入口

程序入口位于 `cmd/<name>/`，配置示例位于 `configs/`，集成测试及其脚本位于
`test/integration/`，共享契约基线位于 `test/testdata/contracts/`。单元测试留在对应包内。
`tools/` 保留 Go 开发程序，只有需要独立脚本时才增加 `scripts/`。

开发工具通过 `scaffold.Inspect` 静态识别唯一入口，不根据模块路径末段推测命令名。
新增文件若包含应用标识，更新初始化受管清单。迁移或删除受管文件后先暂存文件移动，
再运行使用 Git 索引清单复制源码的 `template:check`。

初始化事务格式为 2，命令目录改名与内容替换一起恢复。源命令目录只允许受管入口文件；
额外文件、既有目标目录、人工修改或不明确的目录状态会保留现场并报错。格式 1 的日志
须先由原版本工具恢复。模板初始化不会自动升级旧布局项目。

事务的目标写入临时路径由持久记录的受管文件路径唯一推导，开始前必须不存在。
真实进程退出后只按日志确认的路径清理临时文件，不按文件名前缀删除其他内容。
`v2`、`v10` 等会被 Go 当作版本段的名称在初始化写入前拒绝，以保证命令目录和源码安装产物同名。
