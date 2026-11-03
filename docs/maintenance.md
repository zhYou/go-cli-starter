# 维护清单

本文列出需要在目标仓库落实的设置。文件已提供不代表平台功能已启用；后续待办状态以带 `backlog` 标签的 GitHub Issues 为准；当前已授权任务无需先创建 Issue，实施与审查进展由 PR 记录。

## 责任与审查

维护责任人：**待填写实际团队或账号**。发布负责人和安全报告接收人：**待填写**。

填写 [.github/CODEOWNERS.example](../.github/CODEOWNERS.example) 后复制为 CODEOWNERS，并确认团队有仓库访问权限。默认不启用占位账号。
为 CLI 契约、初始化事务、安全 workflow 和发布流程指定审查责任。

## 仓库管理员设置

- 首次启用 GitHub Issues 时创建 `backlog` 标签；Issue 表单和智能体查询依赖这个稳定标识，不把待办写回仓库 Markdown。
- 为 main 设置规则集，要求 PR、审查通过、分支更新及必需检查。按实际 Actions 检查名称启用 `CI / check`、`CI / agent-mcp`、`CI / platforms (macos-latest)`、`CI / platforms (windows-latest)`、`Security / vulnerability`，首次运行后核对名称。
- 保护发布标签 `v*`，限制创建和修改权限；采用不可变发布资产策略。
- 按 [SECURITY](../SECURITY.md) 填写私密报告入口并启用对应功能；发布支持版本和响应安排。
- 安装或授权 Renovate。现有配置覆盖 Go、Actions SHA 和两份 mise 配置，禁用自动合并及预发布更新。维护者审查准确版本声明并完成版本检查，项目不维护 mise 锁文件。
- 确认 release job 能申请 OIDC 和 attestations 写权限，满足所在 GitHub 套餐、组织与仓库策略。普通 PR 和定时扫描只读，不使用发布凭据。
- 确认定时安全扫描持续运行，并指定失败通知的接收人；停用或归档仓库前交接维护责任。

## 升级和发布

源码检查见[开发说明](development.md)，模板迁移见[模板升级](template-upgrades.md)，接口政策见[兼容性规则](compatibility.md)。
工具升级同步当前版本表、两份 mise 配置、Go 声明和 CI；历史变更通过 Git 追溯。

发布前按[开发说明的发布与交付步骤](development.md#发布与交付)完成源码检查、竞态与漏洞扫描、
本地 snapshot 和发布验收。检查六目标归档、各自 SPDX 与最终摘要。
macOS/Windows 的运行结果以对应 CI 为准，Linux 上的交叉编译不证明这些系统上的运行行为。

Release 创建后证明失败时，workflow 失败；重试必须针对同一批已发布资产的原始字节。核对下载摘要后，在受信任 workflow 中重做证明，不重新构建或覆盖 Release。
本地 snapshot 不上传来源证明。消费方用 checksums.txt 与 `gh attestation verify <asset> --repo <owner/repo>` 验证下载资产。

配置完成后在 PR 中记录已启用的实际设置及证据，不能仅凭配置文件宣称托管设置生效。

## 环境、诊断与验收证据

首次使用及工具版本或 Go 模块变更后显式 setup，再 doctor；智能体环境使用 setup:agent 和 doctor:agent。
普通 check 不安装、升级或修复环境。README、两份 mise 版本声明、Go 和 CI 保持一致；不提交个人模型、凭据、权限或插件配置。
mise、git、gh、Codex、RTK、superpowers 属于预装层，项目不安装或更新；基础检查只要求其中的 mise、git。
doctor:agent 额外要求 gh、Codex、gopls，RTK 仅报告，superpowers 由会话确认。Serena/Graphify 尚在待办。
智能体配套不作为基础 CI 前置条件，gopls MCP 在独立 job 验证；平台支持通过原生 CI 和六目标发布构建验证。

日志默认 info/file，Trace 默认 none；file Trace 只供本地诊断，不承诺 OTLP 或重放。
诊断文件属于各次运行，由系统锁与元数据保护；未知内容和活跃运行不清理，总量为软限制。
发生诊断告警时核对路径、权限和剩余空间，保留业务退出码；不要为了消除告警清空整个用户状态目录。
调查只收集必要且脱敏的事件字段，不公开密钥、完整 argv、环境和原始工具输出。

提交 PR 时分别列出实际完成的 Linux 原生测试、macOS/Windows 原生 CI、六目标交叉构建及宿主安装。
三 OS workflow 已配置不等于三 OS 运行已验收，未完成的项目写为待验证。
每次最终提交后重建 snapshot，release:verify 拒绝 commit 不匹配、混合版本、未列宿主归档或字节变化。
它从最终 checksums 使用真实摘要，以隔离 mise 错 SHA/正确 SHA 验证及绝对安装路径运行检查，
不修改用户全局 mise，也不把应用 HOME 当安装目录。

升级旧模板之前，用原版本工具完成所有未终结初始化事务，包括 format 2，备份现场并确认清理完成；
之后人工合并。模板 0.3.0 与应用兼容决策 v0.1.0→v0.2.0 独立，记录本身不创建标签。
新工具或托管配置的验收必须有实际结果，不能以配置、启动或 mock 成功替代。
