# 安全检查、发布与协作实施计划

> 面向智能体实施者：必须使用 `superpowers:subagent-driven-development` 或
> `superpowers:executing-plans`，逐项执行并维护复选框。

**目标：** 交付 BL-004/006/008/009，补齐版本维护、漏洞检查、发布验证和维护者入口。
**架构：** tools/dev 提供可注入的只读检查；mise 锁定工具；CI 分离源码检查与联网扫描；发布为最终文件生成 SBOM 和来源证明。
**技术栈：** go-toml、x/mod、govulncheck、Syft、GoReleaser、GitHub Actions、Renovate。
**设计：** [已确认设计](../specs/2026-10-29-cli-engineering-design.md)。最终验收包含前三批。

## 全局约束

遵循[实施总览](2026-10-29-cli-engineering-plan.md)全部约束及版本表。
新增工具锁定六平台；PR/main 保持只读权限，写入与 OIDC 权限限于 release job。
验证使用 snapshot，不触发真实发布、标签推送、Renovate 安装或仓库设置变更。
联网检查失败不能报告为通过，本地 snapshot 与远端证明分别报告。

## 审查重点

- `TestVersionsRejectSubstringAndMissingPlatform`：1.27.30 不匹配 1.27.3，锁文件缺平台失败，任务 12。
- `TestHistoricalVersionTextIsNotCurrentDeclaration`：历史计划中的版本快照不影响当前声明检查，任务 12。
- `TestVulnerabilityCheckFailureModes`：受影响漏洞、数据库不可达和工具缺失均失败，任务 13。
- `TestReleaseSnapshotRejectsMissingOrMismatchedSBOM`：产物、SBOM 与校验清单必须对应，任务 14。
- `TestTemplateMetadataAndGuidesSurviveRename`：新项目标识、来源信息和维护者清单在副本中有效，任务 15 复用模板集成测试。

## 文件结构

新增 `tools/dev/versions.go`、`vulnerability.go`、`release.go` 及对应测试，
`renovate.json`、`.github/workflows/security.yml`、PR 模板、CODEOWNERS 示例、SECURITY 和维护说明。
修改 mise、工具锁、doctor、CI、发布配置、README、开发说明与 scaffold 受管清单。

### Task 12: 工具一致性与依赖更新配置

**文件：** 新建 `tools/dev/versions.go`、`versions_test.go`、`renovate.json`；修改 tools/dev/main.go、main_test.go、mise.toml、README、AGENTS、CI、go.mod/go.sum 和 scaffold 白名单。
**输入：** mise.toml、mise.lock、go.mod、当前 README 工具表和 CI 引导版本。
**输出：** `VersionSet{MiseMinimum string,Tools map[string]string,Platforms []string}`；`loadVersions(root string)(VersionSet,error)`、`checkVersions(root string) error`；保持 `doctor(agent bool) error`，从 VersionSet 读取预期值。

- [ ] 引入总览版本的 go-toml/v2 和 x/mod，解析配置与模块元数据；不使用全文字符串包含判断版本。
- [ ] 写 `TestVersionsRejectSubstringAndMissingPlatform`：go1.27.30 与锁定 go1.27.3 不等，删除 windows-arm64 锁记录失败。写 `TestHistoricalVersionTextIsNotCurrentDeclaration`：历史规格版本不参与当前声明；修改当前受管 README 工具表则失败。
- [ ] 执行 `mise exec -- go test ./tools/dev -run 'TestVersions|TestHistoricalVersion'`，确认缺失精确检查时失败。
- [ ] 实现 TOML/Go 模块解析及逐项比较：工具精确版本相等，Go 最低版本 1.27.0 与 toolchain 1.27.3 分别按关系检查，CI mise 引导等于当前最低声明。README 用 tool-versions 起止注释界定受管表，AGENTS 的 mise 最低版本也检查；不扫描历史计划/规格的版本快照。
- [ ] Doctor 用各工具专用、带边界的版本提取器；新增 `versions:check` 接入 check。Renovate 固定依赖、忽略预发布、关闭自动合并，覆盖 Go、Actions SHA、mise，并分组相关工具链变更；锁文件自动更新受平台权限约束时由维护者按文档补齐，检查照常失败。
- [ ] 运行 `mise run versions:check`、`mise run doctor`、`mise exec -- go test ./tools/dev`、`mise run mcp:check`、`mise run template:check`，确认受信任会话中 `codex mcp list` 包含 gopls；提交 `feat: 检查工具版本一致性并配置依赖更新`。

### Task 13: 独立漏洞扫描与定时 CI

**文件：** 新建 `tools/dev/vulnerability.go`、`vulnerability_test.go`、`.github/workflows/security.yml`；修改 tools/dev/main.go、mise.toml、mise.lock、README、开发说明、版本期望测试和 scaffold 白名单。
**输入：** 任务 12 的 VersionSet；govulncheck 模块 v1.9.0。
**输出：** `type ToolRunner func(context.Context,string,...string)([]byte,error)`；`checkVulnerabilities(context.Context,ToolRunner) error`，调用 `govulncheck ./...` 并保留失败原因。

- [ ] 写 `TestVulnerabilityCheckFailureModes`：fake runner 对成功返回 nil；漏洞非零、数据库访问失败和找不到可执行文件均返回错误，不能使用“无漏洞”消息掩盖错误。断言实际命令参数为 `govulncheck ./...`。
- [ ] 执行 `mise exec -- go test ./tools/dev -run TestVulnerabilityCheckFailureModes`，确认缺失任务时失败。
- [ ] 在 mise 增加精确工具 `go:golang.org/x/vuln/cmd/govulncheck = 1.9.0`，纳入显式 setup 工具列表和 doctor；锁定 linux-x64/linux-arm64/macos-x64/macos-arm64/windows-x64/windows-arm64。真实 runner 使用 exec.CommandContext，取消不会遗留子进程。
- [ ] 提供 `mise run vuln`，从 check 中保持独立。security workflow 对 PR、main 和每日 `17 3 * * *` 执行；contents: read，不接收发布凭据。文档说明数据库访问、失败处理和定时扫描作用。
- [ ] 执行 `mise run setup`、`mise run doctor`、`mise run versions:check`、`mise exec -- go test ./tools/dev`、`mise run vuln`；真实网络不可用时记录失败，不以 fake runner 通过替代。提交 `feat: 增加持续漏洞检查与定时扫描`。

### Task 14: SBOM、来源证明与产物校验

**文件：** 新建 `tools/dev/release.go`、`release_test.go`；修改 .goreleaser.yaml、release.yml、mise.toml、mise.lock、tools/dev/main.go、README、开发说明和 scaffold 白名单。
**输入：** 当前六目标压缩包、checksums.txt、Syft v1.55.0；任务 12 的版本检查。
**输出：** `checkReleaseSnapshot(dist string) error`、`mise run release:verify`；为每个压缩包生成 `<archive-name>.spdx.json` 并纳入 checksums.txt。

- [ ] 写 `TestReleaseSnapshotRejectsMissingOrMismatchedSBOM`：压缩包缺 SBOM、校验摘要错误、重复/越界路径均失败，六目标完整且正确时通过；fixture 使用已知字节和对应摘要，不请求网络。
- [ ] 执行 `mise exec -- go test ./tools/dev -run TestReleaseSnapshotRejectsMissingOrMismatchedSBOM`，确认缺失产物验证器时失败。
- [ ] 将 Syft 作为 mise 独立工具安装，不加入应用 go.mod；官方 v1.55.0 已提供六平台归档。固定版本并纳入 setup/doctor/六平台锁。GoReleaser 使用 archive 级 SBOM、SPDX JSON 和显式本地分析参数，取消默认外部元数据 enrichment；文件命名随目标应用自动变化。
- [ ] 实现 snapshot 验证器：安全读取校验文件，校验六目标归档及各自 SBOM 的实际 SHA-256，检查 SPDX 版本与根文档结构；对最终文件验证，不能只断言 YAML 中出现 sboms 字段。确保 SBOM 生成先于最终校验清单生成。
- [ ] 发布 workflow 在 GoReleaser 之后运行 `actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6`（v4.2.2），subject-checksums 指向最终 checksums.txt；release job 只增加所需 id-token/attestations 写权限。证明失败使 workflow 失败；若 Release 已创建，维护者对已发布原始资产重试证明，不重新构建或覆盖字节。
- [ ] 文档提供 checksum、SPDX 和 `gh attestation verify <asset> --repo <owner/repo>` 步骤，说明私有仓库平台前提与本地 snapshot 边界；所有 Actions 保持完整 SHA 锁定。
- [ ] 执行 `mise run setup`、`mise run doctor`、`mise run versions:check`、`mise exec -- go test ./tools/dev`、`mise run release:snapshot`、`mise run release:verify`、`mise run template:check`。本地不运行 attestation 上传；记录远端集成尚需受信任 workflow 验证。提交 `feat: 为发布包生成 SBOM 与构建来源证明`。

### Task 15: 维护者入口与最终验收

**文件：** 新建 `.github/PULL_REQUEST_TEMPLATE.md`、`.github/CODEOWNERS.example`、`SECURITY.md`、`docs/maintenance.md`；修改 README、开发说明、template-upgrades、scaffold 白名单和 docs/backlog.md。
**输入：** 前十四项的真实任务名、验证结果、模板元数据和迁移说明。
**输出：** 可复制的维护清单，关联 BL-004 至 BL-016 的实际交付与 PR。

- [ ] 写 PR 模板，要求说明机器接口变化、兼容记录、验证及无法执行的检查；复用开发说明，不建立第二份待办状态表。
- [ ] 写安全报告和维护责任说明，清楚标识目标维护者需填写的私密报告地址与责任人。提供 CODEOWNERS.example，只有维护者填写后才能启用；不写固定个人账号、凭据或用户目录。
- [ ] 写托管配置清单：必需检查、审查、分支/标签规则、Renovate 启用及锁文件更新权限、来源证明的仓库前提。明确哪些是配置文件，哪些仍需管理员在平台启用。
- [ ] 验证本地 Markdown 链接和副本标识；扩展已有 `TestTemplateMetadataAndGuidesSurviveRename` 集成场景覆盖新文件，避免为静态措辞增加镜像测试。所有包含目标应用标识的新文件进入受管白名单，来源身份和历史记录按语义保留。
- [ ] 执行 `mise run fmt`、`mise run check`、`mise run test:race`、`mise run template:check`、`mise run mcp:check`、`mise run vuln`、`mise run release:snapshot`、`mise run release:verify`；确认 `codex mcp list`。未执行的平台及远端证明列为未验证，不声称通过。
- [ ] 按十三项逐一核对设计验收，更新对应待办日期、状态及实际 PR；只有全项交付才将本轮标记完成。提交 `docs: 补齐维护入口与工程化验收说明`，按选定工作流完成整分支审查。

参考：[GoReleaser SBOM](https://goreleaser.com/customization/sbom/)、
[发布来源证明](https://goreleaser.com/customization/publish/attestations/)、
[固定的 attest 发布](https://github.com/actions/attest/releases/tag/v4.2.2)。
