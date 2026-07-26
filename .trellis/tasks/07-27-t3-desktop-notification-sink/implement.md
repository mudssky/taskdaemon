# T3 Desktop 原生通知 sink — 实施清单

## 顺序

1. **配置** `types.go` / `defaults.go` / `load.go` / `env.go` + `notify_test.go` 断言
2. **DesktopSink** `sink_desktop.go` + `sink_desktop_test.go`（stub sender、级别过滤、无 sender 禁用、Bus 隔离）
3. **装配** `serve.go` 注册 DesktopSink
4. **capability** `capability_notification.go` + 测试（stub backend）
5. **desktop.Run** 注册 Wails notifications service、Bind sender、点击激活窗口
6. **前端** `DesktopNotificationPanel` + settings 挂载 + 导出 + 轻量测试
7. **验收** `go test ./internal/notify/... ./internal/desktop/... ./internal/config/...`、`pnpm typecheck`、`pnpm lint`
8. **HANDOFF** + 三平台手测记录模板

## 验证命令

```bash
cd services/taskdaemon-go && go test ./internal/notify/ ./internal/desktop/ ./internal/config/ -count=1
cd services/taskdaemon-go && go vet ./internal/notify/ ./internal/desktop/ ./internal/config/
pnpm typecheck
pnpm lint
pnpm --filter @taskdaemon/web test -- src/lib/desktop
```

## 回滚

- 配置 `notify.desktop.enabled=false`
- 撤销 `sink_desktop*.go`、`capability_notification*.go`、settings 面板与配置字段
