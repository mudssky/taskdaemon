# AG-UI 事件映射表（C-3）

> 类型源：`packages/agent-protocol/src/agui.ts`、`runtime-types.ts`  
> 上游参考：https://docs.ag-ui.com/concepts/events（2026-07-27）  
> G0 样本：`.trellis/tasks/07-27-g0-agent-runtime-spike/research/samples/{pi,omp}/`

## 1. 对外发出的 AG-UI 事件

| AG-UI type | 必选 | 说明 |
|---|---|---|
| `RUN_STARTED` | 是 | 每个 run 边界 |
| `RUN_FINISHED` | 是* | 成功结束；*与 RUN_ERROR 互斥 |
| `RUN_ERROR` | 是* | 失败结束 |
| `STEP_STARTED` / `STEP_FINISHED` | 否 | 映射 turn/step；无则省略 |
| `TEXT_MESSAGE_START` / `CONTENT` / `END` | 是（有助手文本时） | 流式正文 |
| `TOOL_CALL_START` / `ARGS` / `END` / `RESULT` | 视 capabilities | toolCallDetail |
| `REASONING_START` / `CONTENT` / `END` | 视 capabilities.thinking | thinking 块；**不用**已弃用的 THINKING_* |
| `STATE_SNAPSHOT` | 否 | 重连或显式快照 |
| `MESSAGES_SNAPSHOT` | 否 | 重连补历史 |
| `CUSTOM` | 条件 | `file_change`、`usage` 等 |

## 2. RuntimeEvent → AG-UI 转换规则

| RuntimeEvent | AG-UI | 规则 |
|---|---|---|
| `run_started` | `RUN_STARTED` | 填 threadId/runId |
| `run_finished` | `RUN_FINISHED` | outcome 默认 success |
| `run_error` | `RUN_ERROR` | message/code 透传（code 映射 AGENT_* 若可） |
| `step_started` / `step_finished` | `STEP_*` | stepName 原样 |
| `message_start` | `TEXT_MESSAGE_START` | role 映射 |
| `message_text_delta` | `TEXT_MESSAGE_CONTENT` | delta 追加 |
| `message_end` | `TEXT_MESSAGE_END` | |
| `message_thinking_delta` | `REASONING_*` | 首包 START，中间 CONTENT，message_end 或 run 结束时 END |
| `tool_call_start` | `TOOL_CALL_START` | toolCallName ← toolName |
| `tool_call_args_delta` | `TOOL_CALL_ARGS` | 当 toolCallDetail=`full` |
| `tool_call_end` | `TOOL_CALL_END` | |
| `tool_call_result` | `TOOL_CALL_RESULT` | |
| `usage` | `CUSTOM` name=`usage` | 同时触发 trust UsageEvent 外送 |
| `file_change` | `CUSTOM` name=`file_change` | **仅 coding + workspaceBound** |
| `raw_ignored` | （不发出） | 内部日志 |

### 2.1 toolCallDetail 降级

| 值 | 行为 |
|---|---|
| `full` | START + ARGS + END + RESULT |
| `name-only` | START（仅名）+ END + RESULT；**不发 ARGS** |
| `none` | 不发任何 TOOL_* |

### 2.2 thinking 降级

| capabilities.thinking | 行为 |
|---|---|
| `true` | 发 REASONING_* |
| `false` | 丢弃 thinking 块，不模拟 |

## 3. Pi / OMP 不提供或差异处置

依据 G0 能力矩阵与样本：

| 事件/能力 | Pi | OMP | 处置 |
|---|---|---|---|
| agent/turn/message 骨架 | 有 | 有（超集） | 归一到 RuntimeEvent |
| tool_execution_* | full | full | → TOOL_* |
| thinking 块 | 有（message content） | 有 | → REASONING_* |
| `extension_ui_request` | 有（headless 仍出现） | 可能 | **丢弃**（无 host UI bridge）；记 raw_ignored |
| `available_commands_update` | 少 | 常见 | **不映射 AG-UI**；可触发 capabilities 刷新（后置） |
| `ready` | 少 | 有 | 不映射；仅 adapter 内部就绪 |
| OMP task/hub/subagent 拓扑事件 | 无 | 有 | **MVP 不支持**（不进接口、不发 AG-UI） |
| 密钥进 state | 相对干净 | 可能 Authorization | **强制 sanitize**，永不进 STATE_SNAPSHOT |
| get_entries 历史 | 可用 | 本次超时不可靠 | OMP history=`messages` only |
| 错误形态 | 事件/响应错误 | 更易进程 exit | 统一 `RUN_ERROR` + run failed；进程退出由编排换进程 |

**原则**：不提供 → **不发送**（非发空占位）；不模拟假 thinking/假 tool。

## 4. 一次 run 的事件序列

### 4.1 成功（有文本 + 工具）

```text
RUN_STARTED
  [STEP_STARTED turn_N]
  TEXT_MESSAGE_START
    TEXT_MESSAGE_CONTENT*
    [REASONING_START / CONTENT* / END]   # 若 thinking
  TEXT_MESSAGE_END
  TOOL_CALL_START
    TOOL_CALL_ARGS*                      # 若 full
  TOOL_CALL_END
  TOOL_CALL_RESULT
  TEXT_MESSAGE_START ... TEXT_MESSAGE_END
  [STEP_FINISHED turn_N]
  CUSTOM usage
RUN_FINISHED
```

### 4.2 失败

```text
RUN_STARTED
  ... partial events ...
RUN_ERROR
```

`RUN_FINISHED` 与 `RUN_ERROR` **互斥**；之后不得再发该 run 的内容事件。

### 4.3 取消

```text
RUN_STARTED
  ...
RUN_FINISHED  outcome: { type: "interrupt", reason: "cancelled" }
```

或 `RUN_ERROR` code=`AGENT_RUN_CANCELLED`（G2 二选一，**推荐** FINISHED+interrupt 以对齐 AG-UI interrupt 模型）。

## 5. 断线重连补发语义

| 项 | 约定 |
|---|---|
| 游标 | 每个 SSE 事件必须有 `id`（单调字符串，建议 `{runId}:{seq}`） |
| 客户端 | 重连时带 `Last-Event-ID` 或 query `cursor` |
| 服务端 | 从游标 **之后** 重放缓冲事件；不重放已确认事件 |
| 缓冲范围 | **当前 active run** 的事件环缓（建议 ≥ 最近 1000 条或 5MB）；run 结束后短时保留（建议 5–15min）供刷新 |
| 游标失效 | `400 AGENT_STREAM_CURSOR_INVALID`；客户端应改拉 `GET messages` + `MESSAGES_SNAPSHOT` 后只听新事件 |
| 历史权威 | transcript 在 runtime session 文件；重连不够时用 REST messages 补齐，**不**保证无限 SSE 全量回放 |
| onDisconnect | `continue`（默认）：run 继续，可重连；`cancel`：断开即 abort |

**不做**：跨 run 的全局 event log 订阅、Agent Protocol 全量 stream namespace 重放。

## 6. SSE 帧格式

```text
id: {cursor}
event: agui
data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"...","delta":"Hi"}

```

- `event` 固定 `agui`（或省略，靠 data.type 区分）。  
- `data` 为单个 `AguiEvent` JSON。  
- 心跳：注释行 `:ping` 每 15s（实现建议，非类型约束）。
