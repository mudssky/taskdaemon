# T3 Desktop 原生通知 sink — 技术设计

## 1. 边界

```text
scheduler/bus ──Publish──▶ notify.Bus
                              │
                              ├── StoreSink / Webhook / Email
                              └── DesktopSink ──DesktopSender──▶ desktop.notificationCapability
                                                                      │
                                                                      ▼
                                                          wails notifications.Service
                                                                      │
                                                            click DEFAULT_ACTION
                                                                      ▼
                                                          main window Show/Focus
```

- **不改** `bridge.go` 注册框架、`Bus` 分发框架。
- 事件契约照搬 C-2（`Event.Title` / `Event.Body` / `Severity`）。
- capability 契约照搬 C-5（`Capability` + `DESKTOP_*`）。

## 2. Go：Desktop capability（C-5）

**文件**：`internal/desktop/capability_notification.go`（独占）

- 名称：`desktop.notification`（已预留常量）
- 后端：`github.com/wailsapp/wails/v3/pkg/services/notifications`
- `Available()`：
  - 后端未注入 → `false, platform, "notification service unavailable"`
  - `CheckAuthorization` 失败 → `false, error, ...`
  - 未授权 → `false, permission, ...`（**不**自动弹系统授权框）
  - 已授权 → `true`
- `Invoke` payload（JSON action 分发）：
  - `{"action":"status"}` → `{authorized, canRequest}`
  - `{"action":"requestPermission"}` → 显式请求；成功后刷新可用态
  - `{"action":"show","title","body","id?"}` → 发送系统通知
- 错误码：`DESKTOP_PERMISSION_DENIED` / `DESKTOP_CAPABILITY_UNAVAILABLE` / `DESKTOP_INVOKE_FAILED`
- 点击：`OnNotificationResponse` 中 DEFAULT_ACTION → 主窗口 `UnMinimise` + `Show` + `Focus`

**注册**：`desktop.go` `newApp` 与 `Run` append-only：

1. `notifications.New()` 作为 Wails Service 注册
2. `registry.Register(NewNotificationCapability(...))` 标注 `// TD2`
3. `notify.BindDesktopSender(cap)`；`Run` 退出时 `Unbind`

## 3. Go：DesktopSink（C-2）

**文件**：`internal/notify/sink_desktop.go`（独占）

```go
type DesktopSender interface {
  SendNotification(ctx context.Context, title, body, id string) error
}

func BindDesktopSender(sender DesktopSender)
func UnbindDesktopSender()
```

- `Name()` = `"desktop"`
- `Enabled()` = `cfg.Enabled && resolveSender() != nil`  
  → 纯 server 无 Bind → 自动禁用、不报错
- `Deliver`：严重级别过滤后调用 `SendNotification(title, body, event.ID)`
- 失败返回错误给 Bus（已有 recover + 隔离），**不阻塞**其他 sink
- 实现 `configUpdater` 热更新 `notify.desktop.*`

## 4. 配置（append-only）

```go
type NotifyDesktopConfig struct {
  Enabled     bool
  MinSeverity string // 空=不限
}
```

- 键：`notify.desktop.enabled` / `notify.desktop.minSeverity`
- 默认：`Enabled=true`（Desktop 壳内开箱可用；server 仍因无 sender 禁用）
- 环境变量：`NOTIFY_DESKTOP_ENABLED` / `NOTIFY_DESKTOP_MIN_SEVERITY`
- 热生效（走 `Bus.UpdateConfig`）

## 5. 前端（C-5 消费）

- 常量已存在：`DesktopCapability.Notification`
- 新增 `DesktopNotificationPanel.tsx`：权限状态、请求授权、发送测试通知
- 非 Desktop：`useDesktopCapability` → `not-desktop` 禁用说明
- **禁止**业务代码读 Wails 全局对象
- 设置页 `/settings` append 面板

## 6. 权限策略（R3）

| 时机 | 行为 |
|---|---|
| 列表/Available | 只 `Check`，不 `Request` |
| 用户点「请求授权」 | 才 `RequestNotificationAuthorization` |
| 被拒后 | `Available=permission`；不再自动弹窗 |
| sink 投递时未授权 | 返回 `DESKTOP_PERMISSION_DENIED`，记 sink 失败 |

## 7. 风险

| 风险 | 处置 |
|---|---|
| macOS 未签名包通知失败 | 错误折叠为 `DESKTOP_INVOKE_FAILED`，手测记录 |
| Linux 无通知守护进程 | 记录不支持环境，不 panic |
| desktop ↔ notify 循环依赖 | sender 接口与 Bind 放在 notify；desktop 单向依赖 |
| 单元测试拉起原生通知 | capability/sink 注入 stub backend |
