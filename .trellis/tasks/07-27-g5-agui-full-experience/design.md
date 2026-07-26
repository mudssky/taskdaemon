# G5 技术设计：AG-UI 完整体验

## 边界

| 层 | 职责 | 本任务改动 |
|---|---|---|
| `packages/agent-protocol` | C-3 类型 SoT | **append-only** 补会话管理 / HITL / 游标相关 DTO 与路由常量 |
| `docs/agent-contracts` + 路线图 §9 | 契约文档 | 追加变更行与语义 |
| `apps/agent-web` | UI + mock + HTTP client | **主交付** |
| `services/agent-gateway` | 真实现 | **不在本任务独占路径**；HTTP client 先对接 C-3 已有端点（steer / stream reconnect）；新 REST 以 mock 为权威实现，gateway 后续补 |

## 架构

```text
AgentWorkspacePage
  ├─ ThreadList（搜索 / 筛选 / 排序 / 重命名 / 归档 / fork）
  ├─ CapabilityBar + WorkspacePanel（cwd / file_change / 越界说明）
  ├─ Timeline（message / tool / thinking 折叠 / HITL / steering 标记）
  ├─ Composer（send | steer | cancel）
  └─ useRunStream
       ├─ reduceAguiEvent（+ CUSTOM / HITL / file_change / 去重）
       ├─ reconnect with Last-Event-ID / cursor
       └─ AgentApi（http | mock）
```

## 契约扩展（C-3 append-only）

### REST

| 方法 | 路径 | 说明 |
|---|---|---|
| `PATCH` | `/v1/threads/{id}` | 重命名 `title`、归档/恢复 `archived` |
| `POST` | `/v1/threads/{id}/fork` | 从可选 `fromMessageId` 分叉；原会话不变 |
| `POST` | `/v1/threads/{id}/hitl` | 响应 HITL：`approve` / `reject` / `modify` |
| `POST` | `/v1/threads/{id}/steer` | **已有** |
| `GET` | `/v1/threads/{id}/runs/{runId}/stream?cursor=` | **已有** 重连 |

`ListThreadsQuery` 追加：`q?`（标题+消息文本搜索）、`includeArchived?`、`sort` 沿用。

`ThreadStatus` 追加：`archived`。

`Thread.metadata.title` 继续作为标题权威（G4 已用）；PATCH 写入该字段。

### AG-UI CUSTOM

| name | value | 条件 |
|---|---|---|
| `file_change` | `{ path, kind, diff?, outsideSandbox? }` | coding + workspaceBound |
| `usage` | `{ inputTokens?, outputTokens?, totalTokens? }` | 有则展示，无则不造 |
| `hitl_request` | `{ requestId, title, context, options?, timeoutMs? }` | 运行中决策点 |

HITL 响应**不**发明 SSE 类型；走 REST `POST .../hitl`，mock/runtime 继续推事件。

## 核心状态机

### StreamViewState（扩展）

- `connection`: `connected | connecting | reconnecting | disconnected | cursor_invalid`
- `seenEventIds: Set` 语义：归约时按 `event.id` 去重
- `fileChanges: FileChangeItem[]`
- `pendingHitl?: HitlRequest`
- `usage?: UsageSnapshot`
- `items` 增加 `kind: "hitl" | "steer"` 条目

### 断线续传

1. 读流异常且 run 仍 `running` → `reconnecting`，可见提示
2. `GET .../stream?cursor=lastEventId`（或 `Last-Event-ID`）
3. 重放事件经 `reduceAguiEvent`；**同一 id 跳过**
4. `AGENT_STREAM_CURSOR_INVALID` → `cursor_invalid`，提示刷新：`GET messages` + 清空 cursor 只听新流
5. 不可续传（无 runId / 终态）→ `disconnected` + 手动重载

### HITL

```text
idle → (CUSTOM hitl_request) → awaiting_human
awaiting_human → approve/reject/modify → responding → (后续事件) → running/idle
awaiting_human + timeoutMs 到期 → timed_out（UI 明示，不假装仍可提交）
```

### Steering

- `status===running && caps.steering` 显示注入入口
- `POST steer` 不 abort 当前 run
- 本地时间线追加 `kind:"steer"` 气泡（可区分样式）

### 会话管理（纯前端可测逻辑）

- `filterThreads(threads, { q, status, sort })` 纯函数
- `forkThreadLocal(source, fromMessageId)`：新 id + 截断消息副本（mock 实现；不改源）

## 能力分支（继承 G4）

| caps / profile | UI |
|---|---|
| `thinking=false` | 不渲染推理区，不伪造 |
| `steering=false` | 隐藏 steering 入口 |
| `workspaceBinding=false` 或 `!thread.workspaceBound` | 整块工作区 UI 不渲染 |
| 双 adapter mock | pi 全能力 / omp 降级，无崩溃无空白 |

## 测试策略

纯函数单测（不测 CSS/DOM 结构）：

1. 事件去重 / 游标续传归约
2. HITL 状态机
3. steering 时间线条目
4. 会话搜索过滤 / fork 截断
5. file_change + 越界标记
6. capability 分支回归

验收命令：`pnpm typecheck`、`pnpm lint`、`pnpm --filter @taskdaemon/agent-web test`

## 风险

1. **gateway 尚未实现 PATCH/fork/hitl**：HTTP 模式这些按钮会 404；mock 默认开发路径完整。
2. 真 runtime HITL 依赖 adapter 发 `hitl_request`；当前 mock 用关键词触发演示。
3. 断线续传头号风险：必须 id 去重 + cursor 失效路径。
