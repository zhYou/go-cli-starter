# 模板升级

模板版本记录在 `.template-version`，首版为 0.1.0，当前为 0.2.0，与应用发布版本独立。
新副本初始化时生成 `.template-origin.json`，记录原模板、模板版本、目标标识及可确认的来源提交。
复制包或未知 Git 来源使用 unknown；旧项目没有元数据仍可构建和检查，不使用派生项目自己的 HEAD 冒充模板来源。
已有来源记录在同参初始化时保持字节不变。初始化不是升级工具，不会覆盖业务文件。

## 迁移到 0.1.0

1. 在干净分支备份并对照模板与业务项目的差异；先阅读来源记录，再选择已审查的模板提交。
2. 合并 apperror、validation、logging 和 CLI 输出入口。保留业务 Result、参数及退出码；业务直接调用也须校验。greet 是可移除示例，按现有业务人工调整工厂和依赖。
3. 合并结果注册与 JSON Schema；特殊编码须提供覆盖并验证实际 JSON。审查兼容基线与变更记录，禁止用刷新快照代替迁移说明。
4. 合并开发任务、工具精确版本、六平台锁和 Renovate 配置；运行 setup、doctor、versions:check、check 与 test:race。
5. 合并发布 SBOM、校验与证明配置，运行 snapshot 和 release:verify；在目标仓库按维护清单启用必需检查、安全报告和来源证明权限。
6. 验证业务行为后记录所采用的模板版本和提交。不自动覆盖来源历史或业务文件，不将本地缓存、凭据和个人智能体配置带入项目。

初始化中断时用相同参数重试；人工修改、参数冲突或损坏日志会停止恢复并保留现场。先备份 `.template-init/`，核对摘要和目标后再人工处理，不能删除现场后盲目重试。


## 迁移到 0.2.0

1. 在干净分支备份业务文件。若存在格式 1 的 `.template-init/` 日志，先用原版本工具以
   原参数完成恢复；新工具不会转换旧日志，不能通过删除恢复现场绕过检查。
2. 将根目录 `main.go`、`main_test.go` 移到 `cmd/<name>/`，保留入口的信号处理和版本变量。
   源码运行改为 `go run ./cmd/<name>`，安装改为 `go install <module>/cmd/<name>@latest`。
3. 合并入口识别及格式 2 目录事务，更新受管文件路径、开发构建、模板副本安装、MCP
   诊断和 GoReleaser 的 `main`。命令目录名与应用名一致，模块末段可以不同；同参数初始化
   仍幂等。初始化只用于原始模板改名，已有项目按本清单人工合并。
4. 将配置示例移入 `configs/`，归档映射去掉父目录，发布包根目录继续包含
   `config.example.yaml`。用户配置目录、`--config` 和配置优先级保持原样。
5. 将进程测试和脚本移到 `test/integration/`，将共享基线原样移到
   `test/testdata/contracts/`。合并历史快照读取逻辑，新路径缺失时检查旧 Git 路径；
   损坏、非普通文件和 Git 错误必须失败。不要刷新快照来掩盖行为差异。
6. 暂存文件移动后运行 `mise run fmt`、`mise run check`、`mise run test:race`、
   `mise run mcp:check`、`mise run release:snapshot` 和 `mise run release:verify`。
   核对六目标归档的二进制、README、根目录配置、SPDX 和校验清单。
7. 记录采用的模板版本与提交，保留来源历史；应用版本、CLI 机器字段和退出码无需因本次
   目录调整而改变。macOS/Windows 的运行行为由对应平台 CI 验证。
