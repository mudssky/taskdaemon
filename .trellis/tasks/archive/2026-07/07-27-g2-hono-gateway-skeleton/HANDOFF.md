# G2 HANDOFF — Hono Agent Gateway 骨架

## 摘要

落地 `services/agent-gateway`（`@taskdaemon/agent-gateway`）：Hono + Node、C-3 全端点、mock 多 adapter 编排、PrincipalResolver / audit·usage 钩子、tool allowlist + workspace 沙箱。真实 Pi/OMP adapter **未实现**（归 G3）。

## 规模（G0）

- Gateway = 编排 + 协议 + 策略 + **薄索引**（MemoryStore；不双写 transcript）
- 超并发策略：**reject** → `503 AGENT_RUNTIME_UNAVAILABLE`（见 design.md）
- 池化：`coldStartCost=high` → `minIdle≥1`；`low` → `minIdle=0`

## 路径

| 路径 | 说明 |
|---|---|
| `services/agent-gateway/**` | 服务主体（独占） |
| `package.json` | append typecheck/lint/`test:agent-gateway` + lint-staged |
| `pnpm-lock.yaml` | 新依赖 lock |
| `.trellis/tasks/07-27-g2-hono-gateway-skeleton/{design,implement,HANDOFF}.md` | 任务文档 |

## 模块

- `src/trust/*` — dev / gateway-headers PrincipalResolver；Logging/Memory emitter
- `src/runtime/{registry,pool,orchestrator,mock-adapter}` — 多 adapter 路由与池
- `src/policy/policy.ts` — allowlist + 沙箱
- `src/stream/*` — RuntimeEvent→AG-UI + SSE 环缓
- `src/routes/routes.ts` — C-3 路径 + `GET /v1/runtimes` + `/health`
- Mock runtimeIds：`mock-high` / `mock-low` / `mock-high-heavy`

## 验收

```text
pnpm typecheck          # green（含 gateway）
pnpm lint               # green（info: 无 error）
pnpm --filter @taskdaemon/agent-gateway test   # 22 passed
```

## G3 替换点

```ts
registry.register(realPiRuntime); // AgentRuntime 实现
// routes / orchestrator 无需改
```

## 残留 / 非目标

- 无真实 Pi/OMP 进程、无 SQLite 薄索引、无 MCP 目录（G6）
- 生产须 `PRINCIPAL_RESOLVER=gateway`；dev 模式启动告警已实现
- 同 thread active-run 409 在极速 mock 下窗口极窄；逻辑在 `findActiveRun` + 测试注释

## 分支

`mudssky/g2-hono-gateway-skeleton`（勿 merge；由协调者 archive）
