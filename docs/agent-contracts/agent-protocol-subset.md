# Agent Protocol REST 子集（C-3）

> 类型源：`packages/agent-protocol/src/protocol.ts`、`routes.ts`  
> 上游参考：Agent Protocol OpenAPI **0.1.6**  
> API 前缀：`/v1`（`AGENT_API_PREFIX`）

## 1. 实现的资源与操作

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/v1/threads` | 创建 thread + 底层 runtime session |
| `GET` | `/v1/threads` | 列表（分页/过滤） |
| `GET` | `/v1/threads/{threadId}` | 获取 thread 元数据 |
| `DELETE` | `/v1/threads/{threadId}` | 删除 thread 并 dispose runtime session |
| `GET` | `/v1/threads/{threadId}/messages` | 消息历史（透传 `getMessages`） |
| `GET` | `/v1/threads/{threadId}/capabilities` | **能力协商透出** |
| `POST` | `/v1/threads/{threadId}/runs` | 创建 background run |
| `GET` | `/v1/threads/{threadId}/runs` | 列出 runs |
| `GET` | `/v1/threads/{threadId}/runs/{runId}` | 获取 run 状态 |
| `POST` | `/v1/threads/{threadId}/runs/stream` | 创建 run 并 **SSE 推送 AG-UI 事件** |
| `GET` | `/v1/threads/{threadId}/runs/{runId}/stream` | 重连已有 run 的事件流 |
| `POST` | `/v1/threads/{threadId}/runs/{runId}/cancel` | abort 当前生成 |
| `POST` | `/v1/threads/{threadId}/steer` | 可选：运行中转向 |
| `POST` | `/v1/threads/{threadId}/follow-up` | 可选：follow-up |

路径常量见 `AgentRoutes`。

## 2. 请求 / 响应形态

### 2.1 分页（统一）

查询：`offset`（默认 0）、`limit`（默认 50，上限 200）。  
响应：

```json
{
  "items": [],
  "offset": 0,
  "limit": 50,
  "hasMore": false,
  "total": 0
}
```

排序（threads/runs）：`sort.field=created_at|updated_at`，`sort.direction=asc|desc`。默认 `updated_at desc`。

### 2.2 `POST /v1/threads`

**Request**（`CreateThreadRequest`）：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| threadId | string | 否 | 客户端指定；缺省 UUID |
| runtimeId | string | 否 | 默认 gateway 配置（通常 `pi`） |
| profile | `coding` \| `general` | 否 | 默认 `coding` |
| workspaceRoot | string \| null | 否 | general 可省略 |
| workspaceBinding | boolean | 否 | 强制开关；缺省随 profile |
| model | string | 否 | |
| thinkingLevel | enum | 否 | off/low/medium/high |
| tools | `"none"` \| string[] | 否 | 透传 SessionOpts |
| systemPrompt / appendSystemPrompt | string | 否 | OMP general 裁剪关键 |
| metadata | object | 否 | 不得含密钥 |

**Response 200**：`Thread`  
**错误**：`422 AGENT_VALIDATION_FAILED`、`503 AGENT_RUNTIME_UNAVAILABLE`

### 2.3 `GET /v1/threads`

Query：分页 + `status` + `runtimeId` + `profile` + sort。  
租户隔离：仅返回当前 `X-Tenant-Id` 下的 threads。

### 2.4 `GET /v1/threads/{threadId}/messages`

透传 `AgentRuntime.getMessages`；**不做**独立消息库双写（G0 持久化结论）。

### 2.5 `GET /v1/threads/{threadId}/capabilities`

**Response**（`ThreadCapabilitiesResponse`）：

```json
{
  "threadId": "...",
  "runtimeId": "pi",
  "capabilities": { "...": "RuntimeCapabilities" }
}
```

前端**只**通过此端点（或 thread 创建响应的附带字段，G2 可选）发现能力，禁止按 `runtimeId` 硬编码。

### 2.6 `POST /v1/threads/{threadId}/runs`

**Request**（`CreateRunRequest`）：`input.text` / `input.messages`、`model?`、`onDisconnect`（`cancel`|`continue`，默认 `continue`）。  
**Response 200**：`Run`（status 通常 `pending`→快速转 `running`）。  
并发：同一 thread **同时仅一个 active run**（409 `AGENT_RUN_CONFLICT`）。

### 2.7 `POST /v1/threads/{threadId}/runs/stream`

与创建 run 相同 body；响应 `Content-Type: text/event-stream`，事件为 AG-UI JSON（见 [agui-event-mapping.md](./agui-event-mapping.md)）。  
响应头建议带 `Content-Location` 指向 `/v1/threads/{threadId}/runs/{runId}`。

### 2.8 `GET .../runs/{runId}/stream`

重连：支持 `cursor` query 或 `Last-Event-ID` 头。语义见事件映射文档。

### 2.9 `POST .../cancel`

Body 可选 `{ "action": "interrupt" }`。映射 `AgentRuntime.abort`。  
G0：abort 真停后会话可继续 → run 变 `cancelled`/`interrupted`，thread 回到 `idle`。

### 2.10 steer / follow-up

仅当 capabilities 对应字段为 true。否则 `400 AGENT_CAPABILITY_UNSUPPORTED`。  
Body：`PromptInput` 形状（`{ "input": { "text": "..." } }`）。

## 3. 错误形态

统一 envelope：

```json
{
  "error": {
    "code": "AGENT_THREAD_NOT_FOUND",
    "message": "thread not found",
    "details": {},
    "traceId": "..."
  }
}
```

| code | HTTP |
|---|---|
| AGENT_VALIDATION_FAILED | 422 |
| AGENT_UNAUTHORIZED | 401 |
| AGENT_FORBIDDEN | 403 |
| AGENT_THREAD_NOT_FOUND | 404 |
| AGENT_RUN_NOT_FOUND | 404 |
| AGENT_RUN_CONFLICT | 409 |
| AGENT_RUN_CANCELLED | 409 |
| AGENT_RUNTIME_UNAVAILABLE | 503 |
| AGENT_CAPABILITY_UNSUPPORTED | 400 |
| AGENT_STREAM_CURSOR_INVALID | 400 |
| AGENT_INTERNAL_ERROR | 500 |

## 4. 明确不实现（及原因）

| 上游能力 | 原因 |
|---|---|
| Store 全量（`/store/*`） | 长期记忆非 MVP；runtime 会话文件已覆盖对话恢复 |
| Agents 内省（`/agents/*`） | 用 `runtimeId` + capabilities 替代；无多 agent 注册表需求 |
| 无 thread 的 ephemeral `/runs/stream` | 企业场景以 thread 为中心；减少状态机分叉 |
| `/threads/{id}/history` 状态修订树 | 薄索引模型不维护 checkpoint 树 |
| `/threads/{id}/copy` | YAGNI |
| 全量 Agent Streaming Protocol（WS commands 原语、namespace 嵌套） | 过载；SSE + AG-UI 足够 G4/G5 |
| 完整 thread values PATCH | 状态权威在 runtime；gateway 只持元数据 |

## 5. 与 runtime 透传关系

| HTTP 操作 | Runtime 方法 |
|---|---|
| POST threads | `createSession` |
| DELETE threads | `disposeSession` |
| GET messages | `getMessages` |
| POST runs / stream | `prompt` → 事件映射 |
| cancel | `abort` |
| steer | `steer?` |
| follow-up | `followUp?` |
| capabilities | `capabilities()` |

**禁止**在协议层重新建模 agent loop、token 生成或 tool 执行引擎。
