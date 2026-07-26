# HANDOFF — G6 MCP 目录与双引擎打通

> taskId: `07-27-g6-mcp-and-interop` · Orca dispatch `task_43c9ae8f4b8f`  
> 日期: 2026-07-27  
> 状态: **实现完成（验收命令全绿）**；**不要 merge**（监督约束）

## 交付摘要

两件事均已落地：

1. **MCP 目录**（`services/agent-gateway/src/mcp/**`）
2. **双引擎互调**（Go `agentbridge` + `httpapi/agent_bridge_*` + runner `agent`）

### MCP 目录（gateway）

| 模块 | 路径 | 说明 |
|---|---|---|
| 类型 | `src/mcp/types.ts` | server 配置、工具描述、reloadMode |
| 目录 | `src/mcp/catalog.ts` | 租户过滤、不可用降级（省略工具，不整会话失败） |
| 策略 | `src/mcp/policy.ts` | 复用 G2 `toolAllowlist` 字符串列表；`server/tool`、`server/*`；越界 `MCP_TOOL_NOT_ALLOWED` |
| 脱敏 | `src/mcp/redaction.ts` | audit 入参脱敏 |
| 路由 | `GET /v1/mcp/servers`、`GET /v1/mcp/tools` | 会话可见工具清单 |
| 生效方式 | `src/mcp/CONFIG.md` | **`reloadMode=restart`**（启动加载 `AGENT_MCP_SERVERS_JSON`） |
| 错误码 | `packages/agent-protocol` | append `MCP_*` |

策略拦截在 **orchestrator tool_call_start / createThread tools**，gateway 侧强制，不依赖 runtime。

### 双引擎

| 方向 | 实现 | 认证 |
|---|---|---|
| taskdaemon → agent | `runner.TypeAgent` + `agentbridge.Client` 创建 thread/run 并轮询终态 | 机器头 `x-auth-subject` / `x-tenant-id` / **透传 `x-trace-id`**；**无 cookie** |
| agent → taskdaemon | `POST/GET /api/inbound/agent/tasks/...` | Bearer + SHA-256 `agentBridge.inboundTokenHash`；**无管理员 cookie** |
| 环路 | `X-Agent-Loop-Depth` + `maxLoopDepth`（默认 1） | depth>=max → `MCP_LOOP_DETECTED` |
| 任务白名单 | `agentBridge.allowedTaskIds` | 默认空 = **全部拒绝** |
| 失败通知 | agent runner 失败写 run 历史；既有 C-2 终态事件路径 | gateway 不可用 → run failed + 调度终态通知 |

配置（C-1 风格 append-only）：`config.AgentBridge` / `defaults` / `env` / `taskdaemon.example.yaml`。

Ent：`runner_type` 增加 `agent`（schema + 生成枚举手补 validator）。

## 验证

```bash
pnpm typecheck          # green
pnpm lint               # green
pnpm test:go            # green（含 agentbridge / agent_bridge HTTP）
pnpm vet:go             # green
pnpm --filter @taskdaemon/agent-gateway test   # 62 passed（含 mcp.test.ts）
```

## 配置要点

```bash
# gateway
AGENT_MCP_SERVERS_JSON='[{"id":"fs","enabled":true,"transport":"stub","tools":[{"name":"read_file"}]}]'
AGENT_TOOL_ALLOWLIST='fs/read_file,echo'

# taskdaemon
TASKDAEMON_AGENT_BRIDGE_ENABLED=true
TASKDAEMON_AGENT_BRIDGE_GATEWAY_BASE_URL=http://127.0.0.1:8787
TASKDAEMON_AGENT_BRIDGE_INBOUND_TOKEN_HASH=<sha256>
TASKDAEMON_AGENT_BRIDGE_ALLOWED_TASK_IDS=1,2
```

Agent 任务 runner_config 示例：

```json
{
  "type": "agent",
  "runtimeId": "mock-high",
  "profile": "general",
  "inputText": "summarize overnight runs",
  "timeoutSeconds": 120
}
```

入站音频：继续用既有 `/api/inbound/audio-play-requests` + `audio.inbound` Bearer（`/api/inbound/agent/audio-play-requests` 返回指引 501）。

## 残留 / 协调者

- [ ] **不要 merge**（监督任务）
- [ ] 路线图 §4.1 虚线改实线、§7.5 G6 状态回写
- [ ] 真实 stdio/http MCP 连接探测（当前 stub + available 标志；执行仍归 runtime）
- [ ] agent 触发后对 agent 型任务的 loop depth 注入依赖 TriggerTask context（已接 `WithLoopDepth`）

## 关键文件清单

- `packages/agent-protocol/src/protocol.ts`（MCP_* 错误码）
- `services/agent-gateway/src/mcp/**`
- `services/agent-gateway/src/{app,config,routes,runtime/orchestrator}.ts`
- `services/agent-gateway/tests/{helpers,mcp}.ts`
- `services/taskdaemon-go/internal/agentbridge/**`
- `services/taskdaemon-go/internal/httpapi/agent_bridge_*`
- `services/taskdaemon-go/internal/{config,runner,scheduler,app}/**`（append）
- `services/taskdaemon-go/internal/data/ent/schema/task.go` + enum 补丁
- `services/taskdaemon-go/taskdaemon.example.yaml`
- `.trellis/tasks/07-27-g6-mcp-and-interop/HANDOFF.md`
