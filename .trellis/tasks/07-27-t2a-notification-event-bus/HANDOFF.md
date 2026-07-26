# T2a HANDOFF — 通知事件总线与站内 sink

## 状态

- **C-2 已冻结**
- 分支：`mudssky/t2a-notification-event-bus`（**未 merge 到 dev**）
- **W2 / T4 / D2 可以开工**（仅依赖 C-2 契约，不依赖本分支是否已合入）

## 决策

| 决策 | 结论 |
|---|---|
| A 脱敏 | Detail 白名单在 `internal/notify` 内实现；不 import `httpapi` |
| B Publisher | scheduler 直接使用 `notify.Event`，经 `Publisher` 接口注入；nil = noop |

## 交付物

### 代码

- `services/taskdaemon-go/internal/notify/**` — Event / Bus / Sink / StoreSink / builders
- `services/taskdaemon-go/internal/data/ent/schema/notification.go` + `pnpm generate:go` 生成物
- append-only：`config/{types,defaults,env,load}.go`、`httpapi/router.go`、`app/{serve,reload,app}.go`、`scheduler/*`
- 新建：`httpapi/notification_{dto,routes}.go`（+ test）

### 契约文档（C-2）

- `.trellis/spec/backend/notification-event-contract.md`（主文档）
- `.trellis/spec/backend/api-contracts.md` — 通知 API 节
- `.trellis/spec/backend/error-handling.md` — `NOTIFY_*` 表
- `.trellis/spec/backend/index.md` — 索引条目
- `docs/roadmap-post-mvp-and-agent.md` — §7.2 T2a done、§9 C-2 产物路径、§16 v5

## 下游开工指引

| 任务 | 只读契约 | 独占可写 |
|---|---|---|
| W2 通知中心 UI | C-2 API 形状 + 错误码 | `apps/web` feature 目录 |
| T4 Webhook/邮件 sink | `Sink` 接口 + Event 字段 | 新 sink 实现 + config append |
| D2 Desktop sink | 同上 | Desktop sink + D1 capability |

**禁止**：在下游发明第二套事件字段名；契约不够用时回到 T2a 改 C-2。

## 验证

```bash
# 本任务相关（全绿）
cd services/taskdaemon-go && go test ./internal/notify/... ./internal/httpapi/... ./internal/scheduler/... ./internal/app/...
pnpm vet:go

# 全量（config 包有 2 个既有 flaky：macOS /var vs /private/var 路径比较，与本任务无关）
pnpm typecheck && pnpm lint && pnpm test:go && pnpm vet:go
```

## 残留

1. **未 merge**：协调者验收后合入 `dev`；T6 若有 Ent 冲突需重跑 `pnpm generate:go`。
2. **config 既有失败**：`TestProjectPathFindsFirstProjectConfig` / `TestProjectLocalPathFindsFirstLocalConfig`（symlink 路径），非 T2a 引入。
3. 调度器 lifecycle 启停事件已发布；高级订阅/静默时段仍属 Out of Scope。

## 给协调者

- PRD AC：事件模型、Sink 注册、失败隔离、站内持久化/已读、API、脱敏、C-2 文档均已覆盖。
- 建议：优先合入本分支以关闭 Ent 冲突窗口（§8.3），再放行 T6 schema 改动。
