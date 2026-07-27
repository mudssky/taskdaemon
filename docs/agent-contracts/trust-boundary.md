# 信任边界与外部网关契约（C-4）

> 类型源：`packages/agent-protocol/src/trust.ts`  
> 读者：未来接入 APISIX / Kong / Java BFF 的同事（无需熟悉本仓库实现）  
> 路线图：§4.3、§6

## 1. 部署拓扑

```text
[Internet]
    → [外部网关：SSO / 配额执行 / 审计存储 / 计费]
         │  注入头（见 §2）
         ▼
    [agent-gateway (Hono)]   ← 假定在外部网关之后，不直接对公网
         │  PrincipalResolver + tool allowlist + workspace 沙箱 + usage/audit 产出
         ▼
    [Runtime Adapter: Pi / OMP]
```

## 2. 注入头

| 头名（规范小写） | 常量 | 语义 | 缺失行为 |
|---|---|---|---|
| `x-auth-subject` | `TRUST_HEADERS.subject` | 调用主体稳定 id | **生产**（gateway-headers 模式）：拒绝 `401 AGENT_UNAUTHORIZED`；**开发**（dev-single-principal）：回落 `dev-user` |
| `x-tenant-id` | `TRUST_HEADERS.tenantId` | 租户分区键 | 生产：拒绝或回落策略由部署配置；开发：回落 `dev` |
| `x-trace-id` | `TRUST_HEADERS.traceId` | 分布式追踪 id | **始终**可缺：gateway 自生成 UUID；已存在则原样透传，**禁止**覆盖 |

说明：

- 头名与路线图 §4.3 / §9.3 一致。  
- agent-gateway **不**验证 JWT/OIDC 签名；只信任已在边界终止 TLS/鉴权的上游。  
- 值不得记入普通 info 日志以外的明文密钥通道；subject/tenant 可进 audit。

## 3. PrincipalResolver

```ts
interface PrincipalResolver {
  resolve(headers: Record<string, string | undefined>): Promise<Principal>
}

type Principal = {
  subject: string
  tenantId: string
  traceId: string
  source: 'gateway-headers' | 'dev-single-principal'
  claims?: Record<string, unknown>
}
```

### 3.1 模式

| 模式 | 配置建议键 | 行为 |
|---|---|---|
| `dev-single-principal` | `PRINCIPAL_RESOLVER=dev`（默认开发） | 固定 subject=`dev-user`，tenant=`dev`，trace 自生成或透传 |
| `gateway-headers` | `PRINCIPAL_RESOLVER=gateway`（生产必选） | 强制读注入头；缺 subject → 401 |

### 3.2 生产切换与启动告警

- 生产部署 **必须** 显式 `gateway-headers`（或等价配置）。  
- 若以默认 dev 模式监听非 loopback，或 `NODE_ENV=production` 仍为 dev resolver：  
  **启动日志必须打显眼告警**（error 级），例如：  
  `SECURITY WARNING: PrincipalResolver is dev-single-principal; not safe for public exposure`  
- 推荐：production + dev resolver → **拒绝启动**（可配置 `ALLOW_DEV_PRINCIPAL=1` 覆盖，仍告警）。

### 3.3 数据分区

所有 thread/run 元数据读写带 `tenantId` 条件；跨租户 id 直接 `404`（不泄露存在性）或 `403`。

## 4. Audit 事件 schema

```ts
type AuditEvent = {
  type: 'agent.audit'
  eventId: string          // uuid
  timestamp: string        // ISO-8601
  traceId: string
  subject: string
  tenantId: string
  threadId?: string
  runId?: string
  workspaceRoot?: string | null
  action:
    | 'thread.create' | 'thread.delete'
    | 'run.create' | 'run.cancel' | 'run.steer' | 'run.follow_up'
    | 'tool.invoke' | 'session.dispose' | 'model.switch'
  resource?: string        // tool name 等
  outcome: 'success' | 'denied' | 'error'
  message?: string
  metadata?: Record<string, unknown>
}
```

- **本仓库**：结构化产出（日志 / webhook / 总线适配器）。  
- **外部**：存储、检索、留存、告警规则。  
- tool.invoke：在策略层允许/拒绝时各打一条；denied 不调用 runtime。

## 5. Usage 事件 schema

```ts
type UsageEvent = {
  type: 'agent.usage'
  eventId: string
  timestamp: string
  traceId: string
  subject: string
  tenantId: string
  threadId: string
  runId: string
  runtimeId: string
  model?: string
  inputTokens?: number
  outputTokens?: number
  totalTokens?: number
  cost?: number
  currency?: string
  metadata?: Record<string, unknown>
}
```

- 来源：runtime 消息上的 usage 字段（G0 确认两 runtime 消息可带 usage）→ 归一后外送。  
- **不做**：配额扣减、账单聚合、多币种换算。  
- 外部网关/计费系统订阅这些事件执行配额。

## 6. 职责分界表

| 能力 | 外部网关 | agent-gateway（本仓库） |
|---|---|---|
| SSO / OIDC / API Key | **负责** | 不实现 |
| 多租户管理 UI | **负责** | 只透传 tenantId 分区 |
| 审计存储与检索 | **负责** | 只产出 AuditEvent |
| 配额执行 / 限流 / 计费 | **负责** | 只产出 UsageEvent |
| APM 后端 | **负责** | 透传 X-Trace-Id |
| tool allowlist | 做不到 | **负责** |
| workspace 沙箱根 | 做不到 | **负责** |
| PrincipalResolver 可插拔 | 注入头 | **负责**解析钩子 |
| agent 语义协议 | 不做 | **负责** |

## 7. 开发期默认

无外部网关时：

1. `PrincipalResolver = dev-single-principal`  
2. audit/usage → 结构化 stdout/文件（G2 定）  
3. 启动打印一次告警  
4. 本地可直连 gateway；**禁止**将该模式暴露到公网

## 8. 给 Java 网关的对接清单

1. 鉴权通过后注入 `X-Auth-Subject`、`X-Tenant-Id`、`X-Trace-Id`。  
2. 将 upstream 的 trace 与本仓 header 对齐，避免双 trace。  
3. 订阅/采集 gateway 吐出的 `agent.audit` / `agent.usage` JSON 事件。  
4. 不要在网关重写 agent body 字段名（契约以本目录与 TS 包为准）。  
5. 健康检查可探 gateway `/health`（G2 实现），不要求带主体头。
