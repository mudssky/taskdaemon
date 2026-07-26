# HANDOFF · T4 Outbound Notification Sink

**taskId**: `task_ef6ed7c3f271`  
**dispatchId**: `ctx_5f654075fa8d`  
**branch**: `mudssky/t4-outbound-notification-sink`  
**merge**: 未 merge（按简报禁止）

## 做了什么

1. **Webhook sink**（`internal/notify/sink_webhook*.go`）
   - 多目标 + 独立开关、方法/超时/重试可配
   - C-2 事件 JSON payload（二次 `filterDetail`）
   - 自定义 header + HMAC-SHA256 签名（`X-Taskdaemon-Signature: sha256=<hex>`，`VerifyWebhookSignature` 可验）
   - SSRF：协议白名单默认 `https`、默认拒私网、Host 白名单、重定向复检（对齐 T0 音频 URL 模型）
   - 目标级内存投递状态（成功/失败计数、最近错误）

2. **Email sink**（`internal/notify/sink_email*.go`）
   - SMTP host/port/加密/认证/发件人/收件人
   - 可读主题正文（非裸 JSON）
   - `minSeverity` 过滤
   - 注入 `emailSender`，单测不触网

3. **共享重试**（`sink_webhook_retry.go`）：指数退避、次数上限、可注入 sleep

4. **装配**
   - `app.Serve` 注册 webhook + email 到 Bus
   - config append-only：`NotifyConfig.Webhook` / `Email` + defaults/load/env
   - 热更新：`runtime_config.Apply` 标记 `notify.webhook` / `notify.email`
   - example yaml 追加 notify 段

5. **契约文档**
   - C-2 配置键表追加 T4 键
   - `error-handling.md` 追加 `NOTIFY_WEBHOOK_*` / `NOTIFY_EMAIL_*`

6. **顺带**：`bus_test` 隔离用例 race（等 fail 状态落盘再断言）

## 验证

```text
pnpm typecheck  # ok
pnpm lint       # ok
pnpm test:go    # ok（含 go test ./internal/notify/...）
pnpm vet:go     # ok
```

## 残留 / 协调者注意

- **未 commit**（简报未强制；工作树有变更，可按需提交）
- **未改**路线图 §7.2 状态行（共享文档，留给验收方勾 done）
- 前端配置 UI 仍属 W1；本任务仅后端通道
- 目标级 `maxRetries: 0` 语义 = **继承全局默认**（与 timeout 一致）；全局 `defaultMaxRetries: 0` 才表示不重试
- SMTP 密码等敏感字段落盘仍为明文配置文件字段（与现有 inbound tokenHash 模式并存）；未做独立加密信封（C-1 全量 secret 方案未在本任务范围）

## 关键新增/修改文件

- `services/taskdaemon-go/internal/notify/sink_webhook.go`
- `services/taskdaemon-go/internal/notify/sink_webhook_ssrf.go`
- `services/taskdaemon-go/internal/notify/sink_webhook_retry.go`
- `services/taskdaemon-go/internal/notify/sink_webhook_test.go`
- `services/taskdaemon-go/internal/notify/sink_email.go`
- `services/taskdaemon-go/internal/notify/sink_email_test.go`
- `services/taskdaemon-go/internal/notify/bus_test.go`（flake 修复）
- `services/taskdaemon-go/internal/config/types.go|defaults.go|load.go|env.go|notify_test.go`
- `services/taskdaemon-go/internal/app/serve.go`
- `services/taskdaemon-go/internal/httpapi/runtime_config.go`
- `services/taskdaemon-go/taskdaemon.example.yaml`
- `.trellis/spec/backend/notification-event-contract.md`
- `.trellis/spec/backend/error-handling.md`
- `.trellis/tasks/07-27-t4-outbound-notification-sink/design.md`
- `.trellis/tasks/07-27-t4-outbound-notification-sink/research/HANDOFF.md`（本文件）
