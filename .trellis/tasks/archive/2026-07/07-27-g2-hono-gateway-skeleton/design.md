# G2 Agent Gateway 技术设计

> 依据：G0 `gateway-scope-decision.md` / `runtime-attach-and-latency.md` / `runtime-toolchain.md` / `persistence-decision` 结论；C-3/C-4 冻结类型包 `@taskdaemon/agent-protocol`。

## 1. 规模结论（G0 回写）

Gateway = **编排 + 协议 + 策略 + 薄索引**，不是第二 agent 运行时。

| 职责 | 本任务 |
|---|---|
| 会话↔进程编排 | **做**（mock runtime 证明；真实 Pi/OMP 归 G3） |
| 非交互 HTTP 入口 | **做**（C-3 全端点） |
| thread/run 持久化 | **薄索引**（内存实现；可换存储；**不**双写 transcript） |
| 协议转换 | **做**（RuntimeEvent → AG-UI SSE） |
| 多 adapter 抽象 | **消费** `AgentRuntime` + capabilities 透出；mock 两种 `coldStartCost` |
| 策略 | **做** tool allowlist + workspace 沙箱根 |
| usage/audit | **做** 只产出，不聚合/不计费 |

**明确不做**：真实 Pi/OMP adapter、密钥管理、SSO、审计存储、配额、agent loop。

## 2. 工具链

| 项 | 选择 |
|---|---|
| 路径 | `services/agent-gateway/**`（路线图独占） |
| Runtime | Node 22+（Hono + `@hono/node-server`） |
| 包名 | `@taskdaemon/agent-gateway` |
| Lint | Biome（对齐 agent-protocol） |
| Test | vitest |
| 依赖契约 | `@taskdaemon/agent-protocol` workspace 协议 |

根 `package.json` 仅 **append** typecheck/lint/test filter。

## 3. 模块边界

```text
services/agent-gateway/src/
  index.ts                 # 启动 / 优雅关闭
  app.ts                   # createApp(deps)
  config.ts                # 环境与池化默认
  errors.ts                # AGENT_* envelope
  trust/                   # PrincipalResolver + TrustEventEmitter + middleware
  policy/                  # tool allowlist + workspace sandbox
  runtime/
    registry.ts            # runtimeId → AgentRuntime
    pool.ts                # coldStartCost 差异化池
    orchestrator.ts        # thread↔session 路由、并发、空闲回收、僵尸清理
    mock-adapter.ts        # high/low coldStart mock（G3 可替换）
  store/
    memory-store.ts        # thread/run 薄索引（tenant 分区）
  stream/
    agui-mapper.ts         # RuntimeEvent → AguiEvent（按 capabilities 降级）
    sse.ts                 # SSE 帧、环缓、重连
  routes/
    health.ts
    runtimes.ts            # GET /v1/runtimes（能力列表；C-3 未列的运维扩展）
    threads.ts
    runs.ts
  observability.ts         # 按 adapter 的池/会话统计
```

**依赖方向**：routes → orchestrator/store/policy/trust；orchestrator → registry/pool；**禁止** routes 直接碰 mock 实现细节。

## 4. 信任边界

- 默认 `PrincipalResolverMode = dev-single-principal`（`DEV_SINGLE_PRINCIPAL` + 透传/自生成 traceId）。
- `gateway-headers`：缺 `x-auth-subject` → `401 AGENT_UNAUTHORIZED`。
- `NODE_ENV=production` 且仍为 dev resolver：启动 **error 告警**；无 `ALLOW_DEV_PRINCIPAL=1` 时 **拒绝启动**。
- `TrustEventEmitter` 默认结构化日志（stdout JSON 行）；失败不阻断主请求。

## 5. 编排与池化

配置（默认来自 G0 延迟建议）：

| 键 | high（CLI 模拟） | low（SDK 模拟） |
|---|---|---|
| minIdle | 1 | 0 |
| maxIdle | 2 | 1 |
| idleTtlMs | 10 min | 2 min |
| maxConcurrency | 8（light）/ 3（heavy） | 16 |
| overLimit | **reject** → `503 AGENT_RUNTIME_UNAVAILABLE`（文档化；不做排队） |

- 创建 thread：选 `runtimeId`（请求 > 配置默认 `mock-high`）→ `createSession` → 薄索引写入。
- 同一 thread **同时仅一个 active run** → `409 AGENT_RUN_CONFLICT`。
- dispose / idle timeout / runtime 异常：清理索引与池槽，不留僵尸（mock 用 `disposed`/`crashed` 标志证明）。
- 观测：`GET /health` 附带按 adapter 分组的 activeSessions / processes / warmPool。

## 6. 协议与流

- 路径挂载 `AGENT_API_PREFIX` + `AgentRoutes`。
- 错误统一 `AgentErrorBody` + `AGENT_ERROR_HTTP_STATUS`。
- SSE：`event: agui`，`id: {runId}:{seq}`，data = `AguiEvent` JSON。
- cancel → `runtime.abort`；推荐 `RUN_FINISHED outcome.interrupt`。
- 客户端断开：`onDisconnect=cancel` 则 abort；默认 `continue` 保留环缓供重连。
- capabilities：**只透传** `runtime.capabilities()`，不改写。

## 7. 策略

- **tool allowlist**：配置默认 `[]`（保守=禁止全部具名 tool 调用时 deny；`tools: "none"` 会话跳过）。越界 audit `denied` + `403 AGENT_FORBIDDEN`（策略拒绝，不调 runtime）。
- **workspace 沙箱**：配置 `workspaceSandboxRoot`；解析后路径必须落在根内；`workspaceBinding=false` **跳过**路径校验，不误拒。

## 8. 持久化决策（记录）

G0：runtime 会话文件为 transcript 权威 → gateway **仅薄索引**。  
本骨架：**进程内 MemoryStore**（重启丢失可接受；接口 `ThreadRunStore` 便于 G 后续换 SQLite）。  
**不做**完整消息双写。

## 9. 测试策略

- 不启真实 Pi/OMP；只用 mock adapter（high + low）。
- 覆盖：契约成功/失败路径、池化分支、并发拒绝、idle 回收、异常清理、allowlist、路径穿越、capabilities 透传、cancel/abort、SSE 断开清理。

## 10. 替换点（G3）

`RuntimeRegistry.register(runtime: AgentRuntime)` — 注册真实 adapter 后无需改 routes/orchestrator。
