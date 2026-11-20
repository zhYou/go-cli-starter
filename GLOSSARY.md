# go-cli-starter

可直接复用的 Go CLI 项目模板，面向智能体开发与维护。

## 工作流

**GitHub issue**：
GitHub Issues 中记录可执行工作、规格和门票的唯一待办来源。
_Avoid_：backlog、本地待办

**开发客户端**：
运行项目开发流程并加载项目规则的智能体客户端；当前包括 Codex 和 OpenCode。
_Avoid_：编码器、Agent

**客户端适配层**：
只承载特定开发客户端配置的仓库内容，不重复共享流程规则。
_Avoid_：客户端规则副本

**OpenCode 支持**：
OpenCode 作为受支持的开发客户端，读取共享项目规则、运行项目命令、使用项目 MCP，并在规定工作区中开发；不要求复制 Codex 专属规则或由项目安装用户级技能。
_Avoid_：OpenCode 兼容

**OpenCode 权限匹配**：
OpenCode 对未匹配操作保持审批，仅自动允许与 Codex 非 RTK 规则相同范围的项目开发命令和只读查询。
_Avoid_：权限完全相同

**Matt Pocock skills**：
由用户维护、可在受支持的开发客户端中加载的工程设计 skills。后续可简称为 Matt skills。
_Avoid_：mattpocock skills（命令和插件命名空间除外）
