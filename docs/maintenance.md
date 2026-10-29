# 维护清单

本文列出需要在目标仓库落实的设置。文件已提供不代表平台功能已启用；任务状态仍以[项目待办](backlog.md)为准。

## 责任与审查

维护责任人：**待填写实际团队或账号**。发布负责人和安全报告接收人：**待填写**。

填写 [.github/CODEOWNERS.example](../.github/CODEOWNERS.example) 后复制为 CODEOWNERS，并确认团队有仓库访问权限。默认不启用占位账号。
为 CLI 契约、初始化事务、安全 workflow 和发布流程指定审查责任。

## 仓库管理员设置

- 为 main 设置规则集，要求 PR、审查通过、分支更新及必需检查。按实际 Actions 检查名称启用 `CI / check`、`CI / platforms (macos-latest)`、`CI / platforms (windows-latest)`、`Security / vulnerability`，首次运行后核对名称。
- 保护发布标签 `v*`，限制创建和修改权限；采用不可变发布资产策略。
- 按 [SECURITY](../SECURITY.md) 填写私密报告入口并启用对应功能；发布支持版本和响应安排。
- 安装或授权 Renovate。现有配置覆盖 Go、Actions SHA 和 mise，禁用自动合并及预发布更新。若平台限制锁更新所需的受信任命令，维护者本地补齐六平台锁，不能跳过版本检查。
- 确认 release job 能申请 OIDC 和 attestations 写权限，满足所在 GitHub 套餐、组织与仓库策略。普通 PR 和定时扫描只读，不使用发布凭据。
- 确认定时安全扫描持续运行，并指定失败通知的接收人；停用或归档仓库前交接维护责任。

## 升级和发布

源码检查见[开发说明](development.md)，模板迁移见[模板升级](template-upgrades.md)，接口政策见[兼容性规则](compatibility.md)。
工具升级同步当前版本表、Go 声明、锁和 CI；历史设计快照保持原值。

发布前运行 check、test:race、vuln、release:snapshot、release:verify。检查六目标归档、各自 SPDX 与最终摘要。
macOS/Windows 的运行结果以对应 CI 为准，Linux 上的交叉编译不证明这些系统上的运行行为。

Release 创建后证明失败时，workflow 失败；重试必须针对同一批已发布资产的原始字节。核对下载摘要后，在受信任 workflow 中重做证明，不重新构建或覆盖 Release。
本地 snapshot 不上传来源证明。消费方用 checksums.txt 与 `gh attestation verify <asset> --repo <owner/repo>` 验证下载资产。

配置完成后在 PR 中记录已启用的实际设置及证据，不能仅凭配置文件宣称托管设置生效。
