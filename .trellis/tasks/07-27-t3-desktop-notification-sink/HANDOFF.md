# HANDOFF · T3 Desktop 原生通知 sink

> 分支：`mudssky/t3-desktop-notification-sink`（**不要 merge**）
> 任务：`.trellis/tasks/07-27-t3-desktop-notification-sink/`
> 依赖契约：C-2（`Sink`/`Event`）+ C-5（`Capability`/`useDesktopCapability`）已冻结并消费

## 做了什么

### Go / notify

1. **配置 append-only** `NotifyDesktopConfig`  
   - 键：`notify.desktop.enabled` / `notify.desktop.minSeverity`  
   - 默认：`enabled=true`（纯 server 仍因无 sender 自动禁用）  
   - env：`NOTIFY_DESKTOP_ENABLED` / `NOTIFY_DESKTOP_MIN_SEVERITY`
2. **`DesktopSink`** `internal/notify/sink_desktop.go`  
   - `Name()=desktop`，实现 C-2 `Sink` + `UpdateConfig`  
   - `Enabled = cfg.Enabled && sender!=nil` → 纯 server 不报错、不投递  
   - 标题/正文来自 `Event.Title`/`Body`；空标题回退事件名  
   - 严重级别过滤复用 `severityMeetsMinimum`  
   - 进程级 `BindDesktopSender` / `UnbindDesktopSender`
3. **装配** `internal/app/serve.go` 注册 DesktopSink（与 T4 同层）

### Go / desktop

1. **`desktop.notification` capability** `capability_notification.go`  
   - Wails `notifications.NotificationService`  
   - actions：`status` / `requestPermission` / `show`  
   - 实现 `notify.DesktopSender`（不自动弹授权框）  
   - 错误码：`DESKTOP_PERMISSION_DENIED` / `UNAVAILABLE` / `INVOKE_FAILED`
2. **`desktop.Run`**  
   - 注册 notifications Service  
   - `// TD2` Register capability  
   - Bind sender；退出 Unbind  
   - 点击 DEFAULT_ACTION → 主窗口 `UnMinimise`/`Show`/`Focus`
3. **注册框架未改**（`bridge.go` 只消费）

### 前端

1. `DesktopNotificationPanel`：权限状态、请求授权、发送测试通知  
2. 设置页 `/settings` append 面板  
3. 仅 `useDesktopCapability(DesktopCapability.Notification)`，无直接 Wails 引用

## 验收命令（本 worker 已跑）

| 命令 | 结果 |
|---|---|
| `go test ./internal/notify/ ./internal/desktop/ ./internal/config/ -count=1` | 绿 |
| `go vet ./internal/notify/ ./internal/desktop/ ./internal/config/` | 绿 |
| `pnpm typecheck` | 绿 |
| `pnpm lint` | 绿 |
| `pnpm --filter @taskdaemon/web test -- src/lib/desktop` | 15 files / 51 tests 绿 |

## 行为要点

| 场景 | 行为 |
|---|---|
| 纯 server | DesktopSink `Enabled=false`，无错误噪音 |
| Desktop 壳 + 已授权 | Bus 事件 → 系统原生通知 |
| 未授权 show/sink | `DESKTOP_PERMISSION_DENIED`；其他 sink 不受影响 |
| 请求权限 | 仅用户点「请求通知权限」才 `RequestAuthorization` |
| 点击通知 | 激活主窗口（无深链） |
| 浏览器设置页 | 禁用说明，不发 Wails 请求 |

## 手测清单

见同目录 `MANUAL_VERIFY.md`（三平台模板）。本 worker **未**拉起 GUI 壳。

协调者建议：

```bash
pnpm dev:desktop
# 登录 → 设置 → 系统通知 → 请求权限 → 发送测试通知
# 触发一次任务失败事件，确认系统通知标题/正文
```

## 独占路径

- `internal/notify/sink_desktop*.go`
- `internal/desktop/capability_notification*.go`
- `apps/web/src/lib/desktop/DesktopNotificationPanel*`

共享 append-only：`config/types|defaults|load|env`、`app/serve.go`、`desktop/desktop.go`、`routes/router.tsx`、`lib/desktop/index.ts`

## 未做 / 留给协调者

- **不要 merge** 到基线
- 三平台 GUI 手测记录填写
- 路线图 §7.4 状态回写
- 通知深链（Out of Scope）
- commit：本会话未提交；协调者可按功能边界提交

## 契约反馈

- C-2 `Sink` / Bus 隔离足够用
- C-5 `unavailable` 无 `invoke`：权限未授予时 **不** 把 capability 标为 unavailable，否则无法 `requestPermission`；权限走 `status` + show 错误码
