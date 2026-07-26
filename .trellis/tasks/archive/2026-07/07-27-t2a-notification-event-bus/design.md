# T2a 通知事件总线与站内 sink — 技术设计

## 1. 边界与定位

本任务在 `internal/notify/` 建立一个**独立于调度器**的事件总线包。调度器只负责「发生了什么」，总线负责「谁关心、怎么投递」。

```text
internal/scheduler/run_lifecycle.go        internal/notify/
  finalizeRun()      ──publish──▶  Bus ──┬──▶ StoreSink（本任务）
  finalizeFailed()   ──publish──▶        ├──▶ DesktopSink（D2）
  recordSkipped()    ──publish──▶        └──▶ WebhookSink/EmailSink（T4）
                                          │
                                          ▼
                                    ent.Notification ──▶ /api/notifications（本任务）
```

**关键边界**：`internal/notify` **不 import** `internal/scheduler`。依赖方向是 scheduler → notify，单向。否则 T4/D2 会被迫拖进调度器依赖。

## 2. 事件模型（C-2 核心）

### 2.1 事件名

采用 `<domain>.<entity>.<action>` 三段式，全小写，点分隔：

```
task.run.succeeded
task.run.failed
task.run.timeout
task.run.cancelled
task.run.skipped
scheduler.lifecycle.started
scheduler.lifecycle.stopped
```

事件名定义为**具名常量**，不接受字符串字面量。新增只能追加常量，禁止改名。

### 2.2 严重级别

有限枚举，参考 slog 级别但独立定义（通知级别与日志级别是两回事）：

```go
type Severity string

const (
    SeverityInfo     Severity = "info"
    SeverityWarning  Severity = "warning"
    SeverityError    Severity = "error"
    SeverityCritical Severity = "critical"
)
```

`Severity` 有 `Valid()` 方法；构造事件时非法值直接返回错误，不落库。

### 2.3 Event 结构

```go
type Event struct {
    ID        string            // 事件唯一标识，用于 sink 去重
    Name      Name              // 具名常量
    Severity  Severity
    OccurredAt time.Time
    Subject   Subject           // 关联实体
    Title     string            // 人类可读标题
    Body      string            // 人类可读正文
    Detail    map[string]any    // 结构化补充，供 Webhook 消费
}

type Subject struct {
    Kind string // "task" | "run" | "scheduler"
    ID   string // 实体 ID，scheduler 类为空
}
```

**`Detail` 的约束**：只放已脱敏的结构化值。命令行、环境变量、URL query 一律不进 `Detail`。构造函数内做白名单过滤，不依赖调用方自觉。

### 2.4 脱敏

复用 `internal/httpapi/redaction.go` 的规则。若该包不便被 notify 依赖（避免 httpapi ← notify 反向依赖），把脱敏规则下沉到一个共享的小包，两边都依赖它。**这个决策在实现第一步就要定，不要留到后面**。

## 3. Notifier 接口与 Bus

### 3.1 接口

```go
type Sink interface {
    Name() string
    Enabled() bool
    Deliver(ctx context.Context, event Event) error
}
```

三个方法各有用途：`Name()` 用于日志与状态查询，`Enabled()` 让配置开关生效且可运行时变化，`Deliver` 是投递本体。

### 3.2 Bus

```go
type Bus struct { /* sinks, config, logger, status */ }

func (b *Bus) Register(sink Sink)
func (b *Bus) Publish(ctx context.Context, event Event)
func (b *Bus) SinkStatuses() []SinkStatus
```

**`Publish` 不返回 error** —— 这是刻意设计。通知失败不应该让调用方（调度器）有任何理由改变自己的行为。

### 3.3 投递模型

`Publish` 把事件投入带缓冲的 channel，由独立 goroutine 消费并分发到各 sink。

| 决策点 | 选择 | 理由 |
|---|---|---|
| 同步 vs 异步 | **异步** | R2 要求不阻塞调度器主循环 |
| 缓冲满时 | **丢弃并计数 + 打日志** | 通知不是关键路径；阻塞调度器是更坏的结果 |
| sink 并发 | 各 sink 独立 goroutine | 单 sink 慢不拖累其他 sink |
| panic 处理 | 每个 sink 调用包 recover | 第三方 sink（T4 的网络调用）panic 不能拖垮进程 |

缓冲区大小可配置，默认值在 design 评审时定（建议 256）。

### 3.4 状态记录

每个 sink 维护：最近一次投递结果、最近失败原因、累计失败计数、最近成功时间。通过 `SinkStatuses()` 暴露，供 W1 设置页与排障使用。

**状态存内存即可**，不持久化 —— 重启后清零是可接受的。

## 4. 站内 sink 与持久化

### 4.1 Ent schema

新增 `internal/data/ent/schema/notification.go`（独立文件，路线图 §8.3）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `event_id` | String, Unique | 对应 `Event.ID`，用于幂等 |
| `name` | String | 事件名 |
| `severity` | Enum | info/warning/error/critical |
| `subject_kind` | String, Optional | |
| `subject_id` | String, Optional | |
| `title` | String | |
| `body` | Text, Optional | |
| `detail` | JSON, Optional | |
| `read_at` | Time, Optional, Nillable | null 表示未读 |
| `occurred_at` | Time | 事件发生时间 |
| `created_at` | Time, Immutable | 落库时间 |

索引：`(read_at, occurred_at)` 支持「未读 + 时间倒序」，`(severity, occurred_at)` 支持级别筛选。

**`event_id` 唯一约束**是幂等保障：同一事件重复投递不产生重复通知。

### 4.2 保留策略

参考 `internal/audio/service.go:408 pruneHistory` 的既有模式：

- 按条数上限淘汰（默认 200），配置 0 为不限
- 按天数淘汰（默认 30 天），配置 0 为不限
- 两个维度**同时生效**，任一触发即淘汰
- 淘汰时机：每次写入后异步触发，不在写入事务内

## 5. HTTP API

路由注册追加到 `internal/httpapi/router.go`（append-only，标注 `// T2a`）。新文件 `notification_routes.go` + `notification_dto.go`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/notifications` | 分页列表，query: `page`/`pageSize`/`read`/`severity` |
| GET | `/api/notifications/unread-count` | 未读计数 |
| POST | `/api/notifications/{id}/read` | 单条已读 |
| POST | `/api/notifications/read` | 批量已读，body: `{ids: []}` |
| POST | `/api/notifications/read-all` | 全部已读 |
| DELETE | `/api/notifications/{id}` | 删除单条 |
| DELETE | `/api/notifications/read` | 清空已读 |
| GET | `/api/notifications/sinks` | sink 状态（供设置页） |

全部走管理员认证，沿用 `auth_middleware.go`。

### 错误码（`NOTIFY_*`）

| 码 | 场景 |
|---|---|
| `NOTIFY_NOT_FOUND` | 通知不存在 |
| `NOTIFY_INVALID_SEVERITY` | 筛选参数非法 |
| `NOTIFY_INVALID_PAGE` | 分页参数非法 |
| `NOTIFY_SINK_UNAVAILABLE` | sink 状态查询失败 |

## 6. 配置

追加到 `internal/config/types.go` 尾部（append-only）：

```go
type NotifyConfig struct {
    BufferSize int                    // 事件缓冲区
    Store      NotifyStoreConfig      // 站内 sink
    // T4 追加 Webhook / Email
    // D2 追加 Desktop
}

type NotifyStoreConfig struct {
    Enabled     bool
    MaxRecords  int   // 0 = 不限
    RetainDays  int   // 0 = 不限
    MinSeverity string
}
```

按 C-1 分级：全部为**热生效**（`Enabled`、`MinSeverity` 通过 `Bus.UpdateConfig` 立即生效，参考 `audio.Service.UpdateConfig` 模式）。`BufferSize` 为**需重启**。

## 7. 接入点

在 `internal/scheduler/run_lifecycle.go` 的三处终态函数发布事件：

| 函数 | 事件 |
|---|---|
| `finalizeRun` | 按 `runner.Status` 映射到 succeeded / failed / timeout |
| `finalizeFailed` | `task.run.failed` |
| `recordSkipped` | `task.run.skipped` |

**注入方式**：`scheduler.Options` 追加一个 `Publisher` 接口字段（只含 `Publish` 方法），默认为 noop。这样 scheduler 的测试不需要构造完整 Bus，也避免 scheduler → notify 的硬依赖。

```go
// internal/scheduler/service.go 的 Options 追加
type Publisher interface {
    Publish(ctx context.Context, event notify.Event)
}
```

> 若为了彻底解耦不想让 scheduler import notify 的 Event 类型，可在 scheduler 侧定义最小事件结构由 app 层转换。**这个取舍在实现第一步定，写进代码注释。**

装配在 `internal/app/serve.go`，与现有 audio service 的装配位置一致。

## 8. 兼容性与回滚

- 纯新增，不改现有 API 与表结构 → 无破坏性变更
- 回滚方式：配置 `notify.store.enabled = false` 即可停用；代码回滚只需 revert，Ent 新表留空不影响其他功能
- 迁移：Ent 自动 migration 新增表，无数据迁移需求

## 9. 风险

| 风险 | 处置 |
|---|---|
| 循环依赖 scheduler ↔ notify | §7 的 Publisher 接口 + noop 默认值 |
| 脱敏包位置未定导致返工 | 实现第一步先定，见 §2.4 |
| 与 T6 争抢 Ent 生成物 | 路线图 §8.3：**T2a 优先**，T6 后合并时重跑 `pnpm generate:go` |
| 缓冲满静默丢事件 | 丢弃必须打日志 + 计数，并在 sink 状态中可见 |
| 契约定得太窄，T4/D2 不够用 | design 评审时邀请 T4/D2 视角审查 `Event` 与 `Sink` 是否够用 |
