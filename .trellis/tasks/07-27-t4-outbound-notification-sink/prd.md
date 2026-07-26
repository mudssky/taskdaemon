# T4 Webhook 与邮件出站 sink

> **轨**：Track T ｜ **依赖**：**C-2 冻结**（T2a 的事件契约），非 T2a 完整交付
> **性质**：纯后端。配置 UI 并入 W1 设置页，本任务不做前端。

## Goal

实现通知事件的两个外部投递通道：HTTP Webhook 与邮件（SMTP），作为 T2a 事件总线的 sink。

## Context

T2a 已冻结 `Notifier` 接口与事件 schema（C-2），本任务只实现两个 sink，**不改事件模型**。站内 sink 是代码结构的参考样板。

- 一期 Out of Scope 列了「Webhook / 邮件」，来源：`.trellis/tasks/archive/2026-05/05-14-cross-platform-scheduler-daemon/prd.md`
- 错误码前缀：`NOTIFY_*`（与 T2a 共用）
- 独占路径：`internal/notify/sink_webhook*.go`、`internal/notify/sink_email*.go`

**明确不做平台**：路线图 §12 已裁定「T4 只做出站通道，不做平台」。

## Requirements

### R1 Webhook sink

- 可配置目标 URL 列表，每个目标可独立开关
- 请求方法、超时、重试次数可配置
- payload 采用 C-2 事件 schema 的 JSON 序列化，不发明新格式
- 支持自定义 header（用于目标系统的鉴权）
- 支持签名：对 payload 计算签名并放入 header，供接收方验签

### R2 邮件 sink

- SMTP 配置：host、port、加密方式、认证凭据、发件人
- 收件人列表可配置
- 邮件主题与正文由事件渲染，格式可读（不是裸 JSON）
- 支持按严重级别过滤（如只发失败事件）

### R3 投递可靠性

- 投递失败重试，退避策略可配置，有次数上限
- 达到上限后放弃并记录，不无限重试
- 单个 sink 失败不影响其他 sink（C-2 已约定，本任务必须遵守）
- 投递不阻塞事件总线与调度器主流程
- 投递状态可查询：最近一次投递结果、失败原因、失败计数

### R4 安全（重点）

- **SSRF 防护**：Webhook 目标 URL 的协议白名单、内网地址策略可配置，默认保守（沿用 T0 音频 URL 下载的策略模型，保持一致）
- SMTP 密码、Webhook 签名密钥、自定义 header 中的凭据**永不明文回显、永不进日志**
- 敏感配置落盘方式与 C-1 契约一致
- 事件 payload 投递前经过脱敏，与 `internal/httpapi/redaction.go` 边界一致
- 重定向次数限制，避免被引导到内网

### R5 配置

- 配置项按 C-1 契约分级：URL 列表、开关、过滤级别属热生效；SMTP 凭据属敏感，落盘为加密/哈希形式
- 配置结构追加到 `internal/config/types.go` 尾部（append-only，路线图 §8.2）

### R6 可测试性

- 投递逻辑与真实网络调用解耦，单元测试不发真实请求、不发真实邮件
- 重试与退避逻辑可用假时钟测试

## Acceptance Criteria

- [ ] Webhook sink 可配置多目标，每个目标可独立开关
- [ ] 方法、超时、重试次数可配置
- [ ] payload 使用 C-2 事件 schema，未发明新字段
- [ ] 支持自定义 header
- [ ] 签名功能可用，接收方可验签（附验签示例）
- [ ] 邮件 sink SMTP 配置完整可用
- [ ] 邮件主题与正文可读，非裸 JSON
- [ ] 按严重级别过滤可用
- [ ] 投递失败按退避策略重试，达上限后放弃并记录
- [ ] 单 sink 失败不影响其他 sink（用测试证明）
- [ ] 投递不阻塞调度器主流程（用测试证明）
- [ ] 投递状态可查询：最近结果、失败原因、失败计数
- [ ] Webhook URL 协议白名单生效，默认拒绝内网地址
- [ ] 内网地址可通过显式配置放行
- [ ] 重定向次数受限
- [ ] SMTP 密码、签名密钥、header 凭据不明文回显、不进日志（用测试证明）
- [ ] 事件 payload 投递前脱敏
- [ ] 配置项分级符合 C-1 契约
- [ ] 单元测试不发真实网络请求与邮件
- [ ] 重试退避逻辑有测试覆盖
- [ ] 全部错误路径返回 `NOTIFY_*` 稳定错误码
- [ ] `pnpm test:go`、`pnpm vet:go` 全绿

## Out of Scope

- 通知规则引擎、订阅管理、静默时段
- 邮件模板编辑器
- 第三方 IM 集成（Slack / 钉钉 / 飞书）—— 有需求时作为新 sink 单独立项
- 投递历史的完整审计与重放 UI
- 前端配置界面（并入 W1 设置页）

## Dependencies & Contracts

| 方向 | 内容 |
|---|---|
| 本任务**消费** | **C-2**（事件 schema + `Notifier` 接口）、**C-1**（配置写入契约，用于配置分级） |
| 本任务**产出** | 无契约冻结职责 |
| 共享文件 | `internal/config/types.go` / `defaults.go`（append-only） |

**开工条件**：C-2 写入 spec 即可开工，无需等 T2a 完整交付。

## Design / Implement 补齐条件

`design.md` 与 `implement.md` **待 C-2 冻结后补**。原因：sink 接口形状、事件 payload 字段、失败语义全部来自 C-2；契约未定时写设计必然返工。

## Definition of Done

- Acceptance Criteria 全部勾选
- 路线图 §7.2 状态已回写
- SSRF 防护策略与 T0 音频 URL 下载策略保持一致，差异有说明

## Notes

- **SSRF 是本任务最大的安全风险**：Webhook 目标由用户配置，默认必须保守
- 站内 sink（T2a）是代码结构样板，直接模仿，不另创模式
- 邮件与 Webhook 共享重试/退避基础设施，避免写两遍
