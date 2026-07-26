# T4 Design · Webhook / Email 出站 sink

## 决策

1. **不改 Bus / Event**：仅实现 `notify.Sink` + `UpdateConfig`，在 `app.Serve` 注册。
2. **配置 append-only**：`NotifyConfig.Webhook` / `Email` 追加到 `config/types.go` 尾部。
3. **SSRF 对齐 T0 音频 URL**：协议白名单、默认拒私网、Host 白名单、重定向复检。
4. **共享重试**：`doWithRetry` 指数退避，可注入 sleep，Webhook/邮件共用。
5. **脱敏**：签名密钥、SMTP 密码、自定义 header 永不进日志/错误串；payload 二次 `filterDetail`。
6. **签名**：HMAC-SHA256 body → `X-Taskdaemon-Signature: sha256=<hex>`。

## 文件

| 路径 | 职责 |
|---|---|
| `sink_webhook.go` | Webhook sink + 签名 + 目标状态 |
| `sink_webhook_ssrf.go` | URL 校验 |
| `sink_webhook_retry.go` | 共享重试 |
| `sink_email.go` | 邮件 sink + SMTP + 可读渲染 |
| `*_test.go` | httptest / 假 SMTP / 隔离测试 |

## 验收命令

```bash
cd services/taskdaemon-go && go test ./internal/notify/... ./internal/config/ -count=1
pnpm test:go && pnpm vet:go
pnpm typecheck && pnpm lint
```
