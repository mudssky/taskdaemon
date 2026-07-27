# 通知事件总线契约（C-2）

> **冻结任务**：T2a（`07-27-t2a-notification-event-bus`）  
> **代码锚点**：`services/taskdaemon-go/internal/notify/`  
> **消费方**：W2（通知中心 UI）、T4（Webhook/邮件 sink）、D2（Desktop sink）、G6（agent 事件）  
> **规则**：冻结后只追加、不改名、不改语义。下游不得发明第二套事件模型。

---

## 1. 包边界

| 包 | 职责 |
|---|---|
| `internal/notify` | 事件模型、Bus、Sink 接口、站内 StoreSink、安全构造函数 |
| `internal/scheduler` | 通过 `Publisher` 发布终态事件；**不** import Bus 实现细节以外的 sink |
| `internal/httpapi` | 站内通知查询 API、sink 状态查询 |
| `internal/app` | 装配 Bus + StoreSink + 注入 scheduler.Publisher |

**依赖方向**：`scheduler → notify`，`httpapi → notify`，`app → notify`。  
**禁止**：`notify` import `scheduler` 或 `httpapi`。

### 决策记录

- **决策 A（脱敏）**：Detail 白名单过滤放在 `notify` 包内，不 import `httpapi/redaction.go`，避免反向依赖。HTTP 日志脱敏与事件 payload 脱敏是两套边界。
- **决策 B（Publisher）**：scheduler 直接使用 `notify.Event`，通过 `Publisher` 接口注入；未注入时为 noop。

---

## 2. 事件名（Name）

采用 `<domain>.<entity>.<action>` 三段式，全小写，点分隔。具名常量定义于 `event.go`。

| 常量 | 值 | 说明 |
|---|---|---|
| `NameTaskRunSucceeded` | `task.run.succeeded` | 任务运行成功 |
| `NameTaskRunFailed` | `task.run.failed` | 任务运行失败 |
| `NameTaskRunTimeout` | `task.run.timeout` | 任务运行超时 |
| `NameTaskRunCancelled` | `task.run.cancelled` | 任务运行被取消 |
| `NameTaskRunSkipped` | `task.run.skipped` | 重叠策略跳过 |
| `NameSchedulerStarted` | `scheduler.lifecycle.started` | 调度器启动 |
| `NameSchedulerStopped` | `scheduler.lifecycle.stopped` | 调度器停止 |

新增事件：只追加常量，禁止改名。

---

## 3. 严重级别（Severity）

有限枚举，与日志级别独立：

| 值 | 含义 |
|---|---|
| `info` | 一般信息 |
| `warning` | 需要关注 |
| `error` | 错误 |
| `critical` | 严重错误 |

`Severity.Valid()` 校验；构造事件时非法值返回错误，不落库。

---

## 4. Event 字段表

| 字段 | 类型 | 说明 |
|---|---|---|
| `ID` | string | 事件唯一标识；sink 幂等键 |
| `Name` | Name | 具名常量 |
| `Severity` | Severity | 有限枚举 |
| `OccurredAt` | time.Time | 事件发生时间（UTC） |
| `Subject` | Subject | 关联实体 |
| `Title` | string | 人类可读标题 |
| `Body` | string | 人类可读正文 |
| `Detail` | map[string]any | 结构化补充（仅白名单键） |

### Subject

| 字段 | 说明 |
|---|---|
| `Kind` | `"task"` \| `"run"` \| `"scheduler"` |
| `ID` | 实体 ID；scheduler 类可为空 |

### Detail 白名单键

仅允许：`taskId`、`runId`、`status`、`exitCode`、`durationMs`、`trigger`、`errorKind`。

**禁止进入 Title/Body/Detail 的内容**：token、密码、完整命令行、环境变量、URL query、stdout/stderr 原文。

任务终态幂等 ID 约定：`<name>:run:<runId>`（见 `TaskRunEventID`）。

---

## 5. Sink 接口与 Bus

```go
type Sink interface {
    Name() string
    Enabled() bool
    Deliver(ctx context.Context, event Event) error
}

type Bus struct { /* ... */ }
func (b *Bus) Register(sink Sink)
func (b *Bus) Publish(ctx context.Context, event Event) // 不返回 error
func (b *Bus) SinkStatuses() []SinkStatus
func (b *Bus) UpdateConfig(cfg config.NotifyConfig)
func (b *Bus) Start()
func (b *Bus) Shutdown()
```

### 投递语义

| 点 | 行为 |
|---|---|
| 同步 vs 异步 | 异步：`Publish` 入缓冲 channel，独立 goroutine 分发 |
| 缓冲满 | 丢弃 + 计数 + 日志；不阻塞调用方 |
| sink 并发 | 各 sink 独立 goroutine |
| 单 sink 失败 | 记日志 + 更新 `SinkStatus`；不影响其他 sink |
| 单 sink panic | recover；不影响进程与其他 sink |
| `Enabled()==false` | 不调用 `Deliver` |
| `Publish` 返回值 | **无 error**；通知失败不得改变业务主流程 |

### SinkStatus（内存态，重启清零）

`name`、`enabled`、`lastSuccessAt`、`lastFailureAt`、`lastError`、`successCount`、`failureCount`、`lastDeliveredAt`。

### 新增 sink 步骤（T4/D2 样板）

1. 实现 `notify.Sink`（参考 `StoreSink`）。
2. 在 `app.Serve` 装配处 `bus.Register(yourSink)`。
3. 配置段 append 到 `config.NotifyConfig`（热生效字段走 `UpdateConfig`）。
4. **不要**修改 `Bus` 分发逻辑。

---

## 6. 站内 sink 与已读模型

- 表：`notifications`（Ent schema：`internal/data/ent/schema/notification.go`）
- `event_id` 唯一：重复投递幂等
- `read_at` null = 未读
- 保留：`maxRecords`（默认 200）+ `retainDays`（默认 30）；`0` = 该维度不限；写入后异步淘汰

---

## 7. 配置键

| 键 | 默认 | 热更新 |
|---|---|---|
| `notify.bufferSize` | 256 | 否（需重启） |
| `notify.store.enabled` | true | 是 |
| `notify.store.maxRecords` | 200 | 是 |
| `notify.store.retainDays` | 30 | 是 |
| `notify.store.minSeverity` | `""`（不限） | 是 |
| `notify.webhook.enabled` | false | 是 |
| `notify.webhook.defaultTimeoutSec` | 10 | 是 |
| `notify.webhook.defaultMaxRetries` | 2 | 是 |
| `notify.webhook.defaultBackoffMs` | 200 | 是 |
| `notify.webhook.allowedSchemes` | `[https]` | 是 |
| `notify.webhook.allowPrivateNetworks` | false | 是 |
| `notify.webhook.allowedHosts` | `[]` | 是 |
| `notify.webhook.maxRedirects` | 3 | 是 |
| `notify.webhook.targets` | `[]` | 是 |
| `notify.email.enabled` | false | 是 |
| `notify.email.minSeverity` | `""` | 是 |
| `notify.email.from` / `to` | 空 | 是 |
| `notify.email.timeoutSeconds` | 15 | 是 |
| `notify.email.maxRetries` | 2 | 是 |
| `notify.email.backoffMs` | 200 | 是 |
| `notify.email.smtp.*` | host 空 / port 587 / encryption starttls | 是（敏感字段不明文回显） |

环境变量别名：`TASKDAEMON_NOTIFY_*`（见 `config/env.go`）。

---

## 8. 代码入口

| 能力 | 路径 |
|---|---|
| 事件模型 | `internal/notify/event.go` |
| Sink / 状态 | `internal/notify/sink.go` |
| Bus | `internal/notify/bus.go` |
| 站内 sink | `internal/notify/sink_store.go` |
| Webhook sink（T4） | `internal/notify/sink_webhook*.go` |
| Email sink（T4） | `internal/notify/sink_email*.go` |
| 安全构造 | `internal/notify/builders.go` |
| 调度发布 | `internal/scheduler/run_lifecycle.go` |
| HTTP API | `internal/httpapi/notification_routes.go` |
