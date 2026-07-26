# T5 系统服务安装器

> **轨**：Track T ｜ **依赖**：无（W1 波次，可立即开工）
> **性质**：纯后端 CLI 任务，无前端工作。

## Goal

提供 CLI 子命令，把 taskdaemon 注册为操作系统级服务，实现开机自启与崩溃重启，覆盖 macOS launchd、Linux systemd、Windows 服务三种形态。

## Context

一期 Out of Scope 明确列了「系统服务安装器」，来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`。

- CLI 现有结构：`internal/cli/`（`command.go` 分发，`config_commands.go` / `task_commands.go` 为子命令样板）
- 配置加载顺序见 `.trellis/spec/backend/configuration-runtime-guidelines.md`
- 错误码前缀：`SERVICE_*`（路线图 §9.1 已分配）
- 独占路径：`internal/cli/service_commands.go`（新建）、`internal/service/**`（新建）

## Requirements

### R1 CLI 子命令

- `install`：生成并注册服务单元
- `uninstall`：注销服务并清理单元文件
- `status`：查询服务注册状态与运行状态
- 子命令组织沿用现有 CLI 分发模式，不另起一套

### R2 跨平台实现

| 平台 | 机制 | 关键约束 |
|---|---|---|
| macOS | launchd plist | 用户级 LaunchAgent 优先，明确是否支持系统级 LaunchDaemon |
| Linux | systemd unit | 用户级 `--user` 优先，明确系统级的权限要求 |
| Windows | 服务控制管理器 | 明确是否需要管理员权限 |

- 每个平台的**服务作用域**（用户级 vs 系统级）必须显式决策并文档化，不能隐式选择
- 不支持的平台/发行版返回明确错误，不静默失败

### R3 服务单元内容

- 服务启动命令、工作目录、配置文件路径、日志输出位置必须正确且可被用户预览
- 服务使用的配置文件路径与 CLI 当前解析结果一致，不出现「装完服务读到另一份配置」
- 支持崩溃自动重启
- 支持显式指定 `--config` 路径并写入服务单元

### R4 权限与安全

- 需要提权时**先检测并明确提示**，不静默失败也不擅自提权
- 权限不足返回 `SERVICE_INSTALL_PERMISSION_DENIED` 一类稳定错误码，附可执行的补救指引
- 服务单元中不写入任何 token、密码等敏感值
- 生成的单元文件权限合理，不对其他用户可写

### R5 幂等与可回退

- 重复 `install` 不产生重复注册，行为可预期（覆盖或明确报错，二选一并文档化）
- `uninstall` 对未安装状态是安全的 no-op 或明确提示
- `install` 中途失败必须清理已产生的副作用，不留半装状态
- `uninstall` 后系统中无残留单元文件

### R6 干跑与可观测

- 提供 `--dry-run`：打印将要写入的单元文件内容与路径，不做任何实际变更
- 所有实际写入路径在执行前后可见
- 遵守 `.trellis/spec/backend/logging-guidelines.md` 的脱敏要求

### R7 文档

- README 或 spec 中说明三平台的作用域选择、权限要求与卸载方式

## Acceptance Criteria

- [ ] `install` / `uninstall` / `status` 三个子命令可用，沿用现有 CLI 分发模式
- [ ] macOS launchd 安装、卸载、状态查询可用
- [ ] Linux systemd 安装、卸载、状态查询可用
- [ ] Windows 服务安装、卸载、状态查询可用
- [ ] 每平台的服务作用域已显式决策并写入文档
- [ ] 不支持的平台返回明确错误，不静默失败
- [ ] 服务单元中的配置文件路径与 CLI 解析结果一致
- [ ] 服务支持崩溃自动重启
- [ ] `--config` 指定的路径正确写入服务单元
- [ ] 权限不足时返回 `SERVICE_*` 稳定错误码并给出补救指引
- [ ] 服务单元不含敏感值，文件权限不对其他用户可写
- [ ] 重复 `install` 行为可预期且已文档化
- [ ] `uninstall` 对未安装状态安全
- [ ] `install` 中途失败会清理副作用，无半装状态
- [ ] `--dry-run` 打印完整单元内容与路径且零副作用
- [ ] 单元文件生成逻辑有单元测试（不依赖真实系统注册）
- [ ] 三平台安装/卸载有手动验证记录（含平台版本）
- [ ] `pnpm test:go`、`pnpm vet:go` 全绿

## Out of Scope

- 服务的 Web UI 管理界面
- 自动更新 / 自升级
- 容器化部署（Docker/K8s）
- 多实例服务注册
- 服务运行时的健康检查探针

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**产出** | 无契约冻结职责 |
| 本任务**消费** | C-1（若需要在设置页展示服务状态，本任务只提供 CLI，UI 不在范围内） |
| 共享文件 | `internal/config/types.go` / `defaults.go`（append-only）、`services/taskdaemon-go/package.json`（追加 script） |

## Definition of Done

- Acceptance Criteria 全部勾选
- 三平台手动验证记录归档到任务目录
- 路线图 §7.2 状态已回写

## Notes

- 跨平台差异集中在 `internal/service/`，CLI 层保持薄
- 单元测试聚焦「生成什么内容」，真实注册行为靠手动验证，不在 CI 里装服务
- 权限相关路径是本任务最容易出错的地方，测试与文档要重点覆盖
