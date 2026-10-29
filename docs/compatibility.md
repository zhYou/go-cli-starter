# 机器接口兼容性

schema_version 保持 1。命令、选项、JSON 字段、枚举和退出码是机器接口；中文帮助和说明可调整。
消费者应忽略新增字段，结果允许额外属性，配置未知键仍失败。删除、改名、改变字段类型、增加必填要求或改变退出码应视为潜在破坏变更。
命令废弃先保留旧入口并说明迁移和计划移除版本，再在适当版本移除。

修改接口后运行 `mise run contract:update` 审查 testdata/contracts/v1.json 差异，并在 docs/compatibility/changes 新建 JSON 记录。
记录必须包含 id、compatibility、summary、paths、previous_version、target_version、migration。
compatibility 为 compatible、breaking 或 review-required；复杂 Schema 变化要求人工审查。
破坏变化不能声明为兼容，0.x 须提升次版本，1.x 以后须提升主版本。
首次基线的 previous_version 为 null，不推测不存在的发布。

`mise run contract:check` 检查当前程序和快照；CI 另用 `--against` 比较 PR base 或 push before，并只接受相对基准新增或修改的记录。
刷新当前快照不会跳过与 Git 基准的比较。检查只给出结构差异，不能证明任意 JSON Schema 的语义兼容。
initial.json 来自设计确认的代码 54d9dcd，在临时副本捕获；派生项目的日常检查只依赖已提交文件。
