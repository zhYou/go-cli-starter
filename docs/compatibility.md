# 机器接口兼容性

schema_version 保持 1。命令、选项、JSON 字段、枚举和退出码是机器接口；所选语言的帮助和说明可调整；问候正文等用户文案随语言变化，JSON 字段不翻译。
消费者应忽略新增字段，结果允许额外属性，配置未知键仍失败。删除、改名、改变字段类型、增加必填要求或改变退出码应视为潜在破坏变更。
命令废弃先保留旧入口并说明迁移和计划移除版本，再在适当版本移除。

修改接口后运行 `mise run contract:update` 审查 test/testdata/contracts/v1.json 差异，并在 docs/compatibility/changes 新建 JSON 记录。
记录必须包含 id、compatibility、summary、paths、previous_version、target_version、migration。
compatibility 为 compatible、breaking 或 review-required；复杂 Schema 变化要求人工审查。
破坏变化不能声明为兼容，0.x 须提升次版本，1.x 以后须提升主版本。
首次基线的 previous_version 为 null，不推测不存在的发布。

`mise run contract:check` 检查当前程序和快照；CI 另用 `--against` 比较 PR base 或 push before，并只接受相对基准新增或修改的记录。
刷新当前快照不会跳过与 Git 基准的比较。检查只给出结构差异，不能证明任意 JSON Schema 的语义兼容。
initial.json 来自设计确认的代码 54d9dcd，在临时副本捕获；派生项目的日常检查只依赖已提交文件。

## 当前迁移决策

[2026-10-30-cli-foundations.json](compatibility/changes/2026-10-30-cli-foundations.json) 记录
应用 v0.1.0→v0.2.0 的破坏变更决策：根及各子命令新增 lang，log-level 默认由 warn 改为 info，
共 12 项快照路径。schema_version 仍为 1，结果字段和退出码保持；该记录不表示已创建标签或发布。

配置从 YAML 迁到唯一严格 TOML、默认日志改为 info/file、私有目录及显式语言选择也影响使用，
即使没有进入机器快照仍需人工迁移，步骤见[模板升级](template-upgrades.md)。
旧 LOG_LEVEL 兼容入口保留；Trace 默认 none，file 为本地诊断 JSON，不声明 OTLP 或重放兼容。
模板版本 0.3.0 独立于上述应用版本，来源记录和初始化事务格式保持。
升级前所有未终结旧初始化事务（包括 format 2）须用旧工具完成，不通过删除日志或刷新基线跳过。
