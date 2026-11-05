# 维护清单

本文列出需要在目标仓库落实的设置。文件已提供不代表平台功能已启用；后续工作统一记录在[项目待办](backlog.md)，开始和交付时同步条目状态、更新日期与关联 PR。当前已授权任务无需补建待办，实施与审查进展由 PR 记录。

## 责任与审查

维护责任人：**待填写实际团队或账号**。发布负责人和安全报告接收人：**待填写**。

填写 [.github/CODEOWNERS.example](../.github/CODEOWNERS.example) 后复制为 CODEOWNERS，并确认团队有仓库访问权限。默认不启用占位账号。
为 CLI 契约、初始化事务、安全 workflow 和发布流程指定审查责任。

## 仓库管理员设置

- 为 main 设置规则集，要求 PR、审查通过、分支更新及必需检查。按实际检查名称启用 `check`、`platforms (macos-latest)`、`platforms (windows-latest)`、`vulnerability`，首次运行后核对名称。`check` 汇总全部 CI 任务，包含独立的 MCP 验收；任务失败、取消或意外跳过时不能通过。
- 保护发布标签 `v*`，限制创建和修改权限；采用不可变发布资产策略。
- 按 [SECURITY](../SECURITY.md) 填写私密报告入口并启用对应功能；发布支持版本和响应安排。
- 安装或授权 Renovate。现有配置覆盖 Go、Actions SHA 和`mise.toml`，禁用自动合并及预发布更新。维护者审查准确版本声明并完成版本检查，项目不维护 mise 锁文件。
- 确认 release job 能申请 OIDC 和 attestations 写权限，满足所在 GitHub 套餐、组织与仓库策略。普通 PR 和定时扫描只读，不使用发布凭据。
- 确认定时安全扫描持续运行，并指定失败通知的接收人；停用或归档仓库前交接维护责任。

CI 与 Security 仅检查目标为 main 的 PR，开发分支之间的 PR 和开发分支普通 push 不执行。
PR 改目标、标题或描述编辑会重新检查，同一 PR 的新运行取消旧运行。main 更新继续检查，
每日 Security 继续运行；这些运行与版本 tag 的验证互相隔离。推送 `v*` 标签时，Release
先复用完整 CI 与 Security，全部成功后才执行发布任务，不必移除 main 的必需检查。

## 升级和发布

源码检查见[开发说明](development.md)，模板迁移见[模板升级](template-upgrades.md)，接口政策见[兼容性规则](compatibility.md)。
工具升级同步当前版本表、`mise.toml`、Go 声明和 CI；历史变更通过 Git 追溯。

发布前按[开发说明的发布与交付步骤](development.md#发布与交付)完成源码检查、竞态与漏洞扫描、
本地 snapshot 和发布验收。检查六目标归档、各自 SPDX 与最终摘要。
macOS/Windows 的运行结果以对应 CI 为准，Linux 上的交叉编译不证明这些系统上的运行行为。

Release 创建后证明失败时，workflow 失败；重试必须针对同一批已发布资产的原始字节。核对下载摘要后，在受信任 workflow 中重做证明，不重新构建或覆盖 Release。
本地 snapshot 不上传来源证明。消费方用 checksums.txt 与 `gh attestation verify <asset> --repo <owner/repo>` 验证下载资产。

配置完成后在 PR 中记录已启用的实际设置及证据，不能仅凭配置文件宣称托管设置生效。

## 环境、诊断与验收证据

首次使用及工具版本或 Go 模块变更后显式 setup，再 doctor。
普通 check 不安装、升级或修复环境。README、mise.toml、Go 和 CI 保持一致；不提交个人模型、凭据、权限或插件配置。
mise、git、gh、Codex、RTK、superpowers 由用户维护，项目不安装或更新。
统一 doctor 要求 mise 最低版本、git 和六个项目工具准确版本，包括 gopls；
gh、Codex、RTK 仅报告状态，缺失或探测失败不影响检查结果，superpowers 由实际会话确认。
CI 无需安装 Codex、RTK 或 superpowers；gopls MCP 在独立 job 复用统一 setup 验证。
平台支持通过原生 CI 和六目标发布构建验证。

CI 将 Linux 源码、race、fuzz/benchmark 和 snapshot/安装验收分组并行，保留 macOS/Windows 原生任务。
Go 缓存按 OS、架构、Go 版本和依赖摘要区分，编译缓存再区分任务、提交与 go.mod/go.sum/mise.toml 摘要；
lint 使用独立缓存，六个项目工具复用统一 mise 缓存。缓存仅保存依赖与检查工具的可重建数据，不包含应用状态、诊断文件或发布资产。
Windows 仅缓存 Go 模块；跨运行恢复编译缓存的实测耗时增加，因此编译结果只在当前 runner 内复用。
排查提速效果时记录冷、热缓存的步骤耗时和命中情况；缓存未命中不能跳过检查。

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
