# Pi 能力矩阵（实测）

> 模型：`xh/grok-4.5`（thinking 默认 off，thinking 实验用 high）  
> 版本：`@earendil-works/pi-coding-agent` / CLI `pi` **0.82.0**  
> 快照日期：2026-07-27  
> 验证脚本：`research/scripts/capability-probe.mjs`、`rpc-driver.mjs`  
> 原始事件流：`research/samples/pi/*.jsonl`

## 基础信息

| 项 | 实测 |
|---|---|
| 全称 | Pi coding agent |
| 仓库 | https://github.com/earendil-works/pi （历史 monorepo 亦称 pi-mono / badlogic） |
| 许可证 | MIT |
| 维护活跃度 | 2026-07-26 仍有 push；npm 版本 0.82.0；GitHub ~78k stars（组织仓库） |
| 非交互/RPC | **有**：`pi --mode rpc`（JSONL/`\n` 分帧）；`pi -p` print 模式 |
| SDK | **有**：`createAgentSession` / `AgentSession` / `RpcClient`（`@earendil-works/pi-coding-agent`） |

## 能力表

| 能力 | 结论 | 验证方式 | 观察 |
|---|---|---|---|
| agent loop | **提供** | RPC `prompt` + 事件流 | loop 完全在 runtime：`agent_start`→`turn_*`→`message_*`→`agent_end`/`agent_settled`；gateway 无需介入 loop |
| MCP 工具接入 | **部分提供** | CLI/配置面 + 事件 | 本机默认可走扩展/MCP；本次以 `--tools bash` 测内置工具。运行时增删需扩展/配置层，非 RPC 单命令热插（未测到专用 RPC 帧） |
| 会话持久化 | **提供（runtime）** | 起→发→杀→重启→恢复 | 状态在 `--session-dir` 下 `*.jsonl`；杀进程后 `--session <file>` 恢复；`get_last_assistant_text` 返回 `BLUEBERRY`；`messageCount` 恢复为 2 |
| 多会话并发 | **一会话一进程（CLI）** | 两进程并行 RPC | 两进程各有独立 `sessionId`；未发现单 CLI 进程多会话。SDK `createAgentSession` 可在同进程多实例（SDK 创建成功，见 latency 文档） |
| 模型切换 | **提供（会话级）** | `set_model` / `get_available_models` | RPC 有 `set_model`；实测切到同模型成功；历史 `messageCount` 保留。粒度是会话级，非单请求独立模型字段 |
| steering | **提供** | 协议/类型 + abort 邻域 | RPC 命令 `steer` / `follow_up`；`set_steering_mode`；文档与类型完整。本次 abort 路径验证会话可继续 |
| abort | **提供（真停可继续）** | 长 prompt 中 `abort` | `isStreaming`：abort 前 `true` → 后 `false`；随后 `AFTER_ABORT` 回合成功（~3.7s） |
| 消息历史 | **提供** | `get_messages` / `get_entries` | `get_messages` 返回 `{messages: [...]}`；`get_entries` 含 `model_change` 等；`messageCount` 与轮次一致 |
| 工具调用可见性 | **full** | `--tools bash` + echo | 事件：`tool_execution_start` / `tool_execution_update` / `tool_execution_end`；样本中可还原 tool 名与执行过程（56 条相关事件） |
| thinking / 中间态 | **提供** | `--thinking high` | `message` content 含 `type:"thinking"`；`thinking_events` 命中 614 条相关 JSON；事件类型仍走 `message_update` 等 |
| 非 coding 可用性 | **提供** | 空 tmp cwd + `--no-tools --no-context-files --no-skills --no-extensions` | 可启动并完成「法国首都」类问答；不强制业务 workspace |
| 错误与重试 | **提供** | 错误模型名 | 坏模型启动不一定立刻崩；事件流出现 `success:false` 的 response / error 形态；stderr 有诊断 |
| 接入方式 | **CLI + SDK** | 实测 | CLI RPC 可用；SDK `createAgentSession` import+创建成功（首创 ~2.5s，后续 ~0.26–0.65s） |

## RuntimeCapabilities（Pi 建议填值）

```ts
{
  profile: ['coding', 'general'],
  attachMode: 'cli-spawn', // 另可选 sdk-inprocess
  coldStartCost: 'high',   // CLI 冷启动 median ~3.1s
  steering: true,
  thinking: true,
  toolCallDetail: 'full',
  modelSwitch: 'session',
  workspaceBinding: true,  // 默认 coding；可用 flag 降级
  sessionPersistence: 'runtime'
}
```

## 事件类型样本（basic_prompt）

`extension_ui_request`, `response`, `agent_start`, `turn_start`, `message_start`, `message_end`, `message_update`, `turn_end`, `agent_end`, `agent_settled`

## 备注

- 帧协议：严格 JSONL，仅 `\n` 分帧（与官方 rpc.md 一致；半包/粘包实测通过）。
- `extension_ui_request` 在 headless 下仍会出现，gateway 需忽略或实现 host UI bridge。
