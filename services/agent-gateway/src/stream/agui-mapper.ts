/**
 * RuntimeEvent → AguiEvent 映射（C-3 表；按 capabilities 降级）。
 */

import type {
  AguiEvent,
  RuntimeCapabilities,
  RuntimeEvent,
} from "@taskdaemon/agent-protocol";

export type MapperContext = {
  threadId: string;
  runId: string;
  capabilities: RuntimeCapabilities;
  /** 单调序号，由调用方维护。 */
  nextSeq: () => number;
};

/**
 * 将单条 RuntimeEvent 转为 0..n 条 AG-UI 事件。
 *
 * 参数:
 *   - event: RuntimeEvent。
 *   - ctx: 映射上下文。
 *   - state: 跨事件状态（thinking 开闭）。
 *
 * 返回值:
 *   - AguiEvent[]。
 */
export function mapRuntimeEvent(
  event: RuntimeEvent,
  ctx: MapperContext,
  state: { reasoningOpen: Set<string> },
): AguiEvent[] {
  const ts = Date.now();
  const id = () => `${ctx.runId}:${ctx.nextSeq()}`;
  const { threadId, runId } = ctx;
  const caps = ctx.capabilities;

  switch (event.type) {
    case "run_started":
      return [
        {
          type: "RUN_STARTED",
          threadId,
          runId,
          timestamp: ts,
          id: id(),
        },
      ];
    case "run_finished":
      return [
        {
          type: "RUN_FINISHED",
          threadId,
          runId,
          outcome: { type: "success" },
          timestamp: ts,
          id: id(),
        },
      ];
    case "run_error":
      return [
        {
          type: "RUN_ERROR",
          threadId,
          runId,
          message: event.message,
          code: event.code,
          timestamp: ts,
          id: id(),
        },
      ];
    case "step_started":
      return [
        {
          type: "STEP_STARTED",
          stepName: event.stepName,
          timestamp: ts,
          id: id(),
        },
      ];
    case "step_finished":
      return [
        {
          type: "STEP_FINISHED",
          stepName: event.stepName,
          timestamp: ts,
          id: id(),
        },
      ];
    case "message_start":
      return [
        {
          type: "TEXT_MESSAGE_START",
          messageId: event.messageId,
          role: event.role,
          timestamp: ts,
          id: id(),
        },
      ];
    case "message_text_delta":
      return [
        {
          type: "TEXT_MESSAGE_CONTENT",
          messageId: event.messageId,
          delta: event.delta,
          timestamp: ts,
          id: id(),
        },
      ];
    case "message_thinking_delta": {
      if (!caps.thinking) return [];
      const out: AguiEvent[] = [];
      if (!state.reasoningOpen.has(event.messageId)) {
        state.reasoningOpen.add(event.messageId);
        out.push({
          type: "REASONING_START",
          messageId: event.messageId,
          timestamp: ts,
          id: id(),
        });
      }
      out.push({
        type: "REASONING_CONTENT",
        messageId: event.messageId,
        delta: event.delta,
        timestamp: ts,
        id: id(),
      });
      return out;
    }
    case "message_end": {
      const out: AguiEvent[] = [];
      if (state.reasoningOpen.has(event.messageId)) {
        state.reasoningOpen.delete(event.messageId);
        out.push({
          type: "REASONING_END",
          messageId: event.messageId,
          timestamp: ts,
          id: id(),
        });
      }
      out.push({
        type: "TEXT_MESSAGE_END",
        messageId: event.messageId,
        timestamp: ts,
        id: id(),
      });
      return out;
    }
    case "tool_call_start": {
      if (caps.toolCallDetail === "none") return [];
      return [
        {
          type: "TOOL_CALL_START",
          toolCallId: event.toolCallId,
          toolCallName: event.toolName,
          parentMessageId: event.parentMessageId,
          timestamp: ts,
          id: id(),
        },
      ];
    }
    case "tool_call_args_delta": {
      if (caps.toolCallDetail !== "full") return [];
      return [
        {
          type: "TOOL_CALL_ARGS",
          toolCallId: event.toolCallId,
          delta: event.delta,
          timestamp: ts,
          id: id(),
        },
      ];
    }
    case "tool_call_end": {
      if (caps.toolCallDetail === "none") return [];
      return [
        {
          type: "TOOL_CALL_END",
          toolCallId: event.toolCallId,
          timestamp: ts,
          id: id(),
        },
      ];
    }
    case "tool_call_result": {
      if (caps.toolCallDetail === "none") return [];
      return [
        {
          type: "TOOL_CALL_RESULT",
          toolCallId: event.toolCallId,
          content: event.content,
          timestamp: ts,
          id: id(),
        },
      ];
    }
    case "usage":
      return [
        {
          type: "CUSTOM",
          name: "usage",
          value: {
            inputTokens: event.inputTokens,
            outputTokens: event.outputTokens,
            totalTokens: event.totalTokens,
            model: event.model,
          },
          timestamp: ts,
          id: id(),
        },
      ];
    case "file_change":
      return [
        {
          type: "CUSTOM",
          name: "file_change",
          value: {
            path: event.path,
            kind: event.kind,
            diff: event.diff,
          },
          timestamp: ts,
          id: id(),
        },
      ];
    case "raw_ignored":
      return [];
    default:
      return [];
  }
}

/**
 * 格式化 SSE 帧。
 *
 * 参数:
 *   - event: AguiEvent。
 *
 * 返回值:
 *   - SSE 文本块。
 */
export function formatSseFrame(event: AguiEvent): string {
  const eventId = event.id ?? "";
  const lines = [
    eventId ? `id: ${eventId}` : null,
    "event: agui",
    `data: ${JSON.stringify(event)}`,
    "",
    "",
  ].filter((line) => line !== null);
  return lines.join("\n");
}
