# Agent 对外契约（C-3 / C-4）

> **状态**：已冻结（G1）  
> **日期**：2026-07-27  
> **类型包**：`@taskdaemon/agent-protocol`（`packages/agent-protocol/`）  
> **消费方**：G2 agent-gateway、G3 adapters、G4/G5 agent-web、未来外部 Java 网关  
> **前提**：G0 实测结论（`.trellis/tasks/07-27-g0-agent-runtime-spike/research/`）

## 文档索引

| 文档 | 内容 |
|---|---|
| [agent-protocol-subset.md](./agent-protocol-subset.md) | REST 子集：threads / runs / stream / cancel / messages |
| [agui-event-mapping.md](./agui-event-mapping.md) | AG-UI 事件映射、run 序列、断线重连 |
| [agent-runtime.md](./agent-runtime.md) | `AgentRuntime` 接口、类型、中立性论证 |
| [runtime-capabilities.md](./runtime-capabilities.md) | 能力协商字段、Pi/OMP 实例、与 C-5 同构 |
| [non-coding-profile.md](./non-coding-profile.md) | general profile / workspaceBinding=false |
| [trust-boundary.md](./trust-boundary.md) | C-4：注入头、PrincipalResolver、audit/usage |

## 版本锚点

| 规范 | 参考版本 | 快照日期 | 本仓库实现子集 |
|---|---|---|---|
| AG-UI | docs.ag-ui.com `concepts/events`（无独立 semver；事件名 SCREAMING_SNAKE） | 2026-07-27 | Lifecycle + Text + Tool + Reasoning + 有限 State/Custom；见 agui-event-mapping.md |
| Agent Protocol | OpenAPI **0.1.6**（https://langchain-ai.github.io/agent-protocol/openapi.json） | 2026-07-27 | threads / runs / stream / cancel / messages；**无** Store、Agents 内省、全量 stream 原语 |
| MCP | 协议修订 **2025-03-26**（tools 面） | 2026-07-27 | 不在 gateway 重做 MCP transport；经 runtime 透传 tools，G6 接目录/allowlist |
| Pi RPC / SDK | `@earendil-works/pi-coding-agent` **0.82.0** | 2026-07-27 | Adapter 内部（G0 已锚） |
| OMP | CLI `omp` **17.1.3** | 2026-07-27 | Adapter 内部（G0 已锚） |

## 设计原则

1. **透传优先**：runtime 已提供的 agent loop / LLM / tool 执行 / steer / abort **不二次封装**。
2. **子集克制**：能少一个端点就少一个。
3. **中立抽象**：`AgentRuntime` 无 Pi/OMP 专有命名；差异进 `RuntimeCapabilities`。
4. **能力协商**：前端不得硬编码 adapter 能力（与 C-5 Desktop capability 同构）。
5. **信任外置**：SSO/租户/审计存储/配额执行在外部网关；本仓只消费注入头并产出事件。

## 快速入口（类型）

```ts
import type {
  AgentRuntime,
  RuntimeCapabilities,
  Thread,
  Run,
  AguiEvent,
  Principal,
} from "@taskdaemon/agent-protocol";
import {
  PI_CLI_CAPABILITIES,
  OMP_CLI_CAPABILITIES,
  AgentRoutes,
  TRUST_HEADERS,
} from "@taskdaemon/agent-protocol";
```
