/**
 * Pi/OMP 原生事件 → 中立 RuntimeEvent。
 * 禁止伪造 runtime 未提供的数据；未知事件 → raw_ignored。
 */

import type { MessageRole, RuntimeEvent } from "@taskdaemon/agent-protocol";
import type { NativeMessage } from "./rpc-client.js";

export type EventMapperContext = {
  sessionId: string;
  /** general / workspaceBinding=false 时禁止 file_change。 */
  allowFileChange: boolean;
  thinkingEnabled: boolean;
};

export type EventMapperState = {
  /** assistant messageId 跟踪（原生常无稳定 id）。 */
  openAssistantMessageId?: string;
  openUserMessageId?: string;
  turnIndex: number;
  messageSeq: number;
  runStarted: boolean;
  runEnded: boolean;
};

/**
 * 创建事件映射器状态。
 *
 * 参数: 无。
 * 返回值: 初始状态。
 */
export function createEventMapperState(): EventMapperState {
  return {
    turnIndex: 0,
    messageSeq: 0,
    runStarted: false,
    runEnded: false,
  };
}

/**
 * 将单条原生消息映射为 0..n 条 RuntimeEvent。
 *
 * 参数:
 *   - msg: 原生 stdout 消息。
 *   - ctx: 会话上下文。
 *   - state: 可变映射状态。
 *
 * 返回值:
 *   - RuntimeEvent 数组。
 */
export function mapNativeToRuntimeEvents(
  msg: NativeMessage,
  ctx: EventMapperContext,
  state: EventMapperState,
): RuntimeEvent[] {
  const type = typeof msg.type === "string" ? msg.type : "";
  const ts =
    typeof msg.timestamp === "string" || typeof msg.timestamp === "number"
      ? String(msg.timestamp)
      : undefined;
  const base = { sessionId: ctx.sessionId, ...(ts ? { timestamp: ts } : {}) };

  // RPC response / ready / UI 噪声 / settled 收尾
  if (type === "response") return [];
  if (type === "agent_settled") return [];
  if (
    type === "ready" ||
    type === "extension_ui_request" ||
    type === "available_commands_update"
  ) {
    return [
      {
        type: "raw_ignored",
        ...base,
        reason: "not mapped to RuntimeEvent",
        sourceType: type,
      },
    ];
  }

  if (type === "agent_start") {
    state.runStarted = true;
    state.runEnded = false;
    return [{ type: "run_started", ...base }];
  }

  if (type === "agent_end") {
    state.runEnded = true;
    state.runStarted = false;
    const out: RuntimeEvent[] = [];
    if (state.openAssistantMessageId) {
      out.push({
        type: "message_end",
        sessionId: ctx.sessionId,
        messageId: state.openAssistantMessageId,
      });
      state.openAssistantMessageId = undefined;
    }
    out.push({ type: "run_finished", ...base });
    return out;
  }

  if (type === "turn_start") {
    state.turnIndex += 1;
    return [
      {
        type: "step_started",
        sessionId: ctx.sessionId,
        stepName: `turn_${state.turnIndex}`,
      },
    ];
  }

  if (type === "turn_end") {
    return [
      {
        type: "step_finished",
        sessionId: ctx.sessionId,
        stepName: `turn_${state.turnIndex || 1}`,
      },
    ];
  }

  if (type === "message_start") {
    return mapMessageStart(msg, ctx, state);
  }

  if (type === "message_end") {
    return mapMessageEnd(msg, ctx, state);
  }

  if (type === "message_update") {
    return mapMessageUpdate(msg, ctx, state);
  }

  if (type === "tool_execution_start") {
    const toolCallId = String(msg.toolCallId ?? "");
    const toolName = String(msg.toolName ?? "unknown");
    const out: RuntimeEvent[] = [
      {
        type: "tool_call_start",
        sessionId: ctx.sessionId,
        toolCallId,
        toolName,
      },
    ];
    if (msg.args !== undefined) {
      out.push({
        type: "tool_call_args_delta",
        sessionId: ctx.sessionId,
        toolCallId,
        delta: JSON.stringify(msg.args),
      });
    }
    return out;
  }

  if (type === "tool_execution_update") {
    // 流式 tool 结果：若有 partial args 增量可扩展；当前样本多为 partialResult
    return [];
  }

  if (type === "tool_execution_end") {
    const toolCallId = String(msg.toolCallId ?? "");
    const content = extractToolResultText(msg.result);
    return [
      {
        type: "tool_call_end",
        sessionId: ctx.sessionId,
        toolCallId,
      },
      {
        type: "tool_call_result",
        sessionId: ctx.sessionId,
        toolCallId,
        content,
        isError: Boolean(msg.isError),
      },
    ];
  }

  if (type === "error" || type === "agent_error") {
    return [
      {
        type: "run_error",
        sessionId: ctx.sessionId,
        message: String(msg.message ?? msg.error ?? "runtime error"),
        code: typeof msg.code === "string" ? msg.code : "AGENT_INTERNAL_ERROR",
      },
    ];
  }

  // OMP task/hub/subagent 等拓扑：MVP 丢弃
  if (
    type.startsWith("task_") ||
    type.startsWith("hub_") ||
    type.includes("subagent")
  ) {
    return [
      {
        type: "raw_ignored",
        ...base,
        reason: "mvp excludes topology events",
        sourceType: type,
      },
    ];
  }

  return [
    {
      type: "raw_ignored",
      ...base,
      reason: "unknown native event",
      sourceType: type || "missing_type",
    },
  ];
}

/**
 * message_start 映射。
 */
function mapMessageStart(
  msg: NativeMessage,
  ctx: EventMapperContext,
  state: EventMapperState,
): RuntimeEvent[] {
  const message = (msg.message ?? {}) as Record<string, unknown>;
  const roleRaw = typeof message.role === "string" ? message.role : "assistant";
  // custom 等角色不进对外协议，忽略
  if (roleRaw === "custom") {
    return [
      {
        type: "raw_ignored",
        sessionId: ctx.sessionId,
        reason: "custom message role",
        sourceType: "message_start",
      },
    ];
  }
  const role = normalizeRole(roleRaw);
  state.messageSeq += 1;
  const messageId = `msg-${state.messageSeq}`;
  if (role === "assistant") state.openAssistantMessageId = messageId;
  if (role === "user") state.openUserMessageId = messageId;

  // user 消息通常完整出现在 start，补 text
  const out: RuntimeEvent[] = [
    {
      type: "message_start",
      sessionId: ctx.sessionId,
      messageId,
      role,
    },
  ];
  if (role === "user") {
    const text = extractMessageText(message);
    if (text) {
      out.push({
        type: "message_text_delta",
        sessionId: ctx.sessionId,
        messageId,
        delta: text,
      });
    }
  }
  return out;
}

/**
 * message_end 映射。
 */
function mapMessageEnd(
  msg: NativeMessage,
  ctx: EventMapperContext,
  state: EventMapperState,
): RuntimeEvent[] {
  const message = (msg.message ?? {}) as Record<string, unknown>;
  const roleRaw = typeof message.role === "string" ? message.role : "";
  if (roleRaw === "custom") {
    return [
      {
        type: "raw_ignored",
        sessionId: ctx.sessionId,
        reason: "custom message role",
        sourceType: "message_end",
      },
    ];
  }
  let messageId: string | undefined;
  if (roleRaw === "assistant" || roleRaw === "") {
    messageId = state.openAssistantMessageId;
    state.openAssistantMessageId = undefined;
  } else if (roleRaw === "user") {
    messageId = state.openUserMessageId;
    state.openUserMessageId = undefined;
  }
  if (!messageId) {
    state.messageSeq += 1;
    messageId = `msg-${state.messageSeq}`;
  }

  const out: RuntimeEvent[] = [];
  // 若 assistant end 带 usage，透传
  const usage = (message.usage ?? undefined) as
    | Record<string, unknown>
    | undefined;
  if (usage && typeof usage === "object") {
    out.push({
      type: "usage",
      sessionId: ctx.sessionId,
      inputTokens: num(usage.input),
      outputTokens: num(usage.output),
      totalTokens: num(usage.totalTokens),
      model: typeof message.model === "string" ? message.model : undefined,
    });
  }
  out.push({
    type: "message_end",
    sessionId: ctx.sessionId,
    messageId,
  });
  return out;
}

/**
 * message_update / assistantMessageEvent 映射。
 */
function mapMessageUpdate(
  msg: NativeMessage,
  ctx: EventMapperContext,
  state: EventMapperState,
): RuntimeEvent[] {
  const ame = (msg.assistantMessageEvent ?? msg) as Record<string, unknown>;
  const evtType = typeof ame.type === "string" ? ame.type : "";
  if (!state.openAssistantMessageId) {
    state.messageSeq += 1;
    state.openAssistantMessageId = `msg-${state.messageSeq}`;
  }
  const messageId = state.openAssistantMessageId;

  if (evtType === "text_delta" || evtType === "text_start") {
    const delta =
      typeof ame.delta === "string"
        ? ame.delta
        : typeof ame.content === "string"
          ? ame.content
          : "";
    if (!delta && evtType === "text_start") return [];
    if (!delta) return [];
    return [
      {
        type: "message_text_delta",
        sessionId: ctx.sessionId,
        messageId,
        delta,
      },
    ];
  }

  if (
    evtType === "thinking_delta" ||
    evtType === "thinking_start" ||
    evtType === "thinking_end"
  ) {
    if (!ctx.thinkingEnabled) return [];
    const delta =
      typeof ame.delta === "string"
        ? ame.delta
        : typeof ame.content === "string" && evtType !== "thinking_end"
          ? ame.content
          : "";
    if (!delta) return [];
    return [
      {
        type: "message_thinking_delta",
        sessionId: ctx.sessionId,
        messageId,
        delta,
      },
    ];
  }

  if (evtType === "text_end") return [];

  return [
    {
      type: "raw_ignored",
      sessionId: ctx.sessionId,
      reason: "unhandled message_update",
      sourceType: evtType || "message_update",
    },
  ];
}

/**
 * 角色归一。
 */
function normalizeRole(role: string): MessageRole {
  switch (role) {
    case "system":
    case "user":
    case "assistant":
    case "tool":
    case "developer":
      return role;
    default:
      return "assistant";
  }
}

/**
 * 抽取消息文本。
 */
function extractMessageText(message: Record<string, unknown>): string {
  const content = message.content;
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";
  const parts: string[] = [];
  for (const block of content) {
    if (!block || typeof block !== "object") continue;
    const b = block as Record<string, unknown>;
    if (b.type === "text" && typeof b.text === "string") parts.push(b.text);
  }
  return parts.join("");
}

/**
 * 抽取 tool result 文本。
 */
function extractToolResultText(result: unknown): string {
  if (result == null) return "";
  if (typeof result === "string") return result;
  if (typeof result !== "object") return String(result);
  const r = result as Record<string, unknown>;
  if (typeof r.content === "string") return r.content;
  if (Array.isArray(r.content)) {
    const parts: string[] = [];
    for (const block of r.content) {
      if (!block || typeof block !== "object") continue;
      const b = block as Record<string, unknown>;
      if (typeof b.text === "string") parts.push(b.text);
    }
    return parts.join("");
  }
  try {
    return JSON.stringify(result);
  } catch {
    return "";
  }
}

function num(v: unknown): number | undefined {
  return typeof v === "number" && Number.isFinite(v) ? v : undefined;
}
