# T2a 通知事件总线与站内 sink（后端）

> **轨**：Track T ｜ **依赖**：无（W1 波次，可立即开工）
> **契约冻结点**：**C-2 通知事件 schema**。W2、T4、D2、G6 都消费本任务产物，冻结前它们不开工。

## Goal

建立 taskdaemon 的统一通知事件总线：一次定义事件模型与 `Notifier` 接口，让后续所有通知通道（站内、Desktop、Webhook、邮件）成为可并行开发的 sink 实现。

本任务自带**站内 sink**作为第一个实现与参考样板。

## Context

路线图 v1 曾把站内通知（1b）、Desktop 通知（1c）、Webhook/邮件（2a）拆到不同 Wave。三者本质是**同一事件总线的三个 sink**，分开做会导致事件模型被返工两次，且三个任务争抢同一批文件。本任务是那次重构的产物。

- 一期 Out of Scope 中「完整通知中心」的需求来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`
- 事件来源：`internal/scheduler/`（任务成功/失败/超时/取消）
- 错误码前缀：`NOTIFY_*`（路线图 §9.1 已分配）

## Requirements

### R1 事件模型（契约核心）

- 事件名采用命名空间形式，至少覆盖：任务运行成功、失败、超时、被取消、调度器启停
- 事件 payload 有稳定字段集：事件 ID、事件名、发生时间、严重级别、关联实体（taskId / runId）、标题、正文、结构化 detail
- 严重级别是**有限枚举**，不接受自由字符串
- 事件名与 payload 字段一旦冻结，新增只能追加，不能改名或改语义
- payload 中**禁止**出现 token、密码、完整命令行等敏感内容（沿用 `internal/httpapi/redaction.go` 的脱敏边界）

### R2 `Notifier` 接口与 sink 注册

- 定义 `Notifier` 接口，sink 通过注册方式接入，新增 sink 不改总线代码
- sink 失败语义明确：单 sink 失败不影响其他 sink，不阻塞业务主流程
- sink 失败必须留下可观测痕迹（日志 + 可查询状态），不静默吞掉
- 每个 sink 可独立开关（配置驱动）
- 总线投递不得阻塞调度器主循环

### R3 站内 sink（第一个实现）

- 通知持久化到数据库，支持已读/未读状态
- 保留条数或保留天数可配置，有默认值，配置为 0 表示不限
- 超出保留策略时按时间淘汰

### R4 站内通知查询 API

- 列表查询：分页、按已读状态筛选、按严重级别筛选、按时间倒序
- 未读计数查询（供前端铃铛角标）
- 标记单条已读、批量已读、全部已读
- 删除单条、清空已读
- 统一 envelope、traceId、稳定错误码（`NOTIFY_*` 前缀）

### R5 契约文档化（C-2 冻结产物）

- 事件名清单、payload 字段表、严重级别枚举、sink 注册与失败语义写入 `.trellis/spec/backend/` 对应规范
- API 形状写入 `.trellis/spec/backend/api-contracts.md`
- 冻结完成后回写路线图 §9 表格的产物路径

### R6 Ent schema 约定

- 新增 entity 独立成文件 `internal/data/ent/schema/notification.go`
- 遵守路线图 §8.3：与 T6 不得同时处于「已改 schema 未合并」状态，本任务优先

## Acceptance Criteria

- [ ] 事件模型定义完成，事件名与 payload 字段表写入 spec
- [ ] 严重级别为有限枚举，非法值在编译期或构造期被拒绝
- [ ] `Notifier` 接口定义完成，站内 sink 通过注册方式接入
- [ ] 新增一个空 sink 无需修改总线代码即可接入（用测试证明）
- [ ] 单 sink 失败不影响其他 sink，不阻塞调度器主流程（用测试证明）
- [ ] sink 失败有日志且状态可查询
- [ ] 每个 sink 可通过配置独立开关
- [ ] 任务成功、失败、超时、取消四类事件能正确产生站内通知
- [ ] 通知持久化，已读/未读状态可变更
- [ ] 保留策略生效，配置为 0 时不限
- [ ] 列表 API 支持分页、已读状态筛选、严重级别筛选
- [ ] 未读计数 API 可用
- [ ] 单条已读、批量已读、全部已读、删除、清空已读均可用
- [ ] 全部错误路径返回 `NOTIFY_*` 稳定错误码与 traceId
- [ ] 通知内容不含 token、密码、完整命令行等敏感信息（用测试证明）
- [ ] C-2 契约文档已写入 spec，路线图 §9 产物路径已回写
- [ ] `pnpm test:go`、`pnpm vet:go`、`pnpm typecheck`、`pnpm lint` 全绿

## Out of Scope

- Desktop 原生通知（归 D2）
- Webhook / 邮件出站（归 T4）
- 通知中心前端 UI（归 W2）
- 通知规则引擎、订阅策略、静默时段等高级能力
- 跨设备通知同步

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**产出** | C-2 通知事件 schema + `Notifier` 接口 + 站内查询 API |
| 本任务**消费** | 无 |
| 下游 | W2（通知中心 UI）、T4（出站 sink）、D2（Desktop sink）、G6（agent 事件） |

**冻结即解锁**：C-2 写入 spec 后 W2/T4/D2 即可开工，无需等本任务完整交付。

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §7.2 状态与 §9 产物路径已回写
- 下游任务可仅凭 spec 文档开始开发

## Notes

- 本任务是**契约任务**，接口设计的稳定性优先于实现的完备性
- 站内 sink 是样板，它的代码结构会被 T4/D2 直接模仿，要写得可读
