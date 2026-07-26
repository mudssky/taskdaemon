# MCP 目录配置生效方式

## 决策：`reloadMode = restart`

MCP server 目录在 **进程启动时** 从环境变量加载（见 `loadConfig` 的 `AGENT_MCP_SERVERS_JSON`），运行中修改配置 **不会** 热生效，需要重启 agent-gateway。

理由：

1. 与 G2 其它 gateway 配置（allowlist、池参数）一致，避免半生效状态
2. server 连接方式（stdio/http）变更涉及子进程生命周期，热插拔超出 G6 范围
3. 策略 allowlist 同样启动期加载，目录与策略同周期

## 环境变量

| 键 | 说明 |
|---|---|
| `AGENT_MCP_SERVERS_JSON` | JSON 数组，元素见 `McpServerConfig` |
| `AGENT_TOOL_ALLOWLIST` | 逗号分隔；支持 `tool`、`server/tool`、`server/*` |

## 降级

单个 server `available: false` 或探测失败时：

- 该 server 的工具从 `GET /v1/mcp/tools` 省略
- **不**导致会话创建/run 失败

## 默认策略

`AGENT_TOOL_ALLOWLIST` 为空 → **拒绝一切具名 tool**（含 MCP）。
