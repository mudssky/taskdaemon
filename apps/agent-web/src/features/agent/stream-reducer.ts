/**
 * AG-UI 事件流 → 聊天时间线归约（G4 核心逻辑）。
 * 纯函数，无 I/O；不伪造 runtime 未提供的字段。
 */

import type {
  AgentMessage,
  AguiEvent,
  MessageRole,
  RunStatus,
} from "@taskdaemon/agent-protocol";

/** 时间线上的消息气泡。 */
export type TimelineMessage = {
  kind: "message";
  id: string;
  role: MessageRole;
  text: string;
  /** 推理文本；仅 capabilities.thinking 且事件提供时出现。 */
  thinking?: string;
  streaming: boolean;
  thinkingStreaming?: boolean;
};

/** 时间线上的工具调用条目。 */
export type TimelineTool = {
  kind: "tool";
  id: string;
  name: string;
  /** 入参 JSON/文本；name-only 或未提供时为 undefined。 */
  args?: string;
  result?: string;
  status: "running" | "success" | "error";
  /** 长结果默认折叠。 */
  collapsed: boolean;
};

export type TimelineItem = TimelineMessage | TimelineTool;

/** 单次 run / 会话视图的流状态。 */
export type StreamViewState = {
  runId: string | null;
  /** UI 运行态：idle / running / error / cancelled。 */
  status: "idle" | "running" | "error" | "cancelled";
  items: TimelineItem[];
  error?: {
    message: string;
    code?: string;
    traceId?: string;
  };
  lastEventId?: string;
  /** 连接层状态（与 run status 正交）。 */
  connection: "connected" | "disconnected" | "connecting";
};

/**
 * 创建空流状态。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - 初始 StreamViewState。
 */
export function createEmptyStreamState(): StreamViewState {
  return {
    runId: null,
    status: "idle",
    items: [],
    connection: "connected",
  };
}

/**
 * 将历史 AgentMessage 列表水合为时间线（非流式）。
 *
 * 参数:
 *   - messages: REST 消息历史。
 *   - options.includeThinking: 是否保留 thinking 块（由 capabilities 决定）。
 *   - options.toolCallDetail: 工具展示粒度。
 *
 * 返回值:
 *   - 时间线条目列表。
 */
export function hydrateTimelineFromMessages(
  messages: AgentMessage[],
  options: {
    includeThinking: boolean;
    toolCallDetail: "full" | "name-only" | "none";
  },
): TimelineItem[] {
  const items: TimelineItem[] = [];

  for (const message of messages) {
    let text = "";
    let thinking: string | undefined;

    for (const block of message.content) {
      if (block.type === "text") {
        text += block.text;
        continue;
      }
      if (block.type === "thinking" && options.includeThinking) {
        thinking = (thinking ?? "") + block.text;
        continue;
      }
      if (block.type === "tool_use") {
        if (options.toolCallDetail === "none") {
          continue;
        }
        items.push({
          kind: "tool",
          id: block.toolCallId,
          name: block.name,
          args:
            options.toolCallDetail === "full" && block.args !== undefined
              ? formatUnknown(block.args)
              : undefined,
          status: "success",
          collapsed: true,
        });
        continue;
      }
      if (block.type === "tool_result") {
        if (options.toolCallDetail === "none") {
          continue;
        }
        const existing = items.find(
          (item): item is TimelineTool =>
            item.kind === "tool" && item.id === block.toolCallId,
        );
        if (existing) {
          existing.result = block.content;
          existing.status = block.isError ? "error" : "success";
          existing.collapsed = shouldCollapseResult(block.content);
        } else {
          items.push({
            kind: "tool",
            id: block.toolCallId,
            name: "(tool)",
            result: block.content,
            status: block.isError ? "error" : "success",
            collapsed: shouldCollapseResult(block.content),
          });
        }
      }
    }

    if (text.length > 0 || thinking !== undefined || message.role === "user") {
      items.push({
        kind: "message",
        id: message.id,
        role: message.role,
        text,
        thinking,
        streaming: false,
      });
    }
  }

  return items;
}

/**
 * 用一条 AG-UI 事件归约流状态。
 *
 * 参数:
 *   - state: 当前状态。
 *   - event: AG-UI 事件。
 *   - options.toolCallDetail: 工具展示粒度（name-only 忽略 ARGS）。
 *   - options.includeThinking: false 时丢弃 REASONING_*。
 *
 * 返回值:
 *   - 新状态（不可变）。
 */
export function reduceAguiEvent(
  state: StreamViewState,
  event: AguiEvent,
  options: {
    toolCallDetail: "full" | "name-only" | "none";
    includeThinking: boolean;
  } = { toolCallDetail: "full", includeThinking: true },
): StreamViewState {
  const next: StreamViewState = {
    ...state,
    items: state.items.map(cloneItem),
    error: state.error ? { ...state.error } : undefined,
  };

  if (event.id) {
    next.lastEventId = event.id;
  }

  switch (event.type) {
    case "RUN_STARTED": {
      next.runId = event.runId;
      next.status = "running";
      next.connection = "connected";
      next.error = undefined;
      return next;
    }
    case "RUN_FINISHED": {
      next.status = event.outcome?.type === "interrupt" ? "cancelled" : "idle";
      next.items = finalizeStreaming(next.items);
      return next;
    }
    case "RUN_ERROR": {
      next.status = "error";
      next.error = {
        message: event.message,
        code: event.code,
      };
      next.items = finalizeStreaming(next.items);
      return next;
    }
    case "TEXT_MESSAGE_START": {
      next.items.push({
        kind: "message",
        id: event.messageId,
        role: event.role,
        text: "",
        streaming: true,
      });
      return next;
    }
    case "TEXT_MESSAGE_CONTENT": {
      const msg = findMessage(next.items, event.messageId);
      if (msg) {
        msg.text += event.delta;
        msg.streaming = true;
      }
      return next;
    }
    case "TEXT_MESSAGE_END": {
      const msg = findMessage(next.items, event.messageId);
      if (msg) {
        msg.streaming = false;
      }
      return next;
    }
    case "REASONING_START": {
      if (!options.includeThinking) {
        return next;
      }
      let msg = findMessage(next.items, event.messageId);
      if (!msg) {
        msg = {
          kind: "message",
          id: event.messageId,
          role: "assistant",
          text: "",
          thinking: "",
          streaming: true,
          thinkingStreaming: true,
        };
        next.items.push(msg);
      } else {
        msg.thinking = msg.thinking ?? "";
        msg.thinkingStreaming = true;
      }
      return next;
    }
    case "REASONING_CONTENT": {
      if (!options.includeThinking) {
        return next;
      }
      const msg = findMessage(next.items, event.messageId);
      if (msg) {
        msg.thinking = (msg.thinking ?? "") + event.delta;
        msg.thinkingStreaming = true;
      }
      return next;
    }
    case "REASONING_END": {
      if (!options.includeThinking) {
        return next;
      }
      const msg = findMessage(next.items, event.messageId);
      if (msg) {
        msg.thinkingStreaming = false;
      }
      return next;
    }
    case "TOOL_CALL_START": {
      if (options.toolCallDetail === "none") {
        return next;
      }
      next.items.push({
        kind: "tool",
        id: event.toolCallId,
        name: event.toolCallName,
        status: "running",
        collapsed: true,
      });
      return next;
    }
    case "TOOL_CALL_ARGS": {
      if (options.toolCallDetail !== "full") {
        return next;
      }
      const tool = findTool(next.items, event.toolCallId);
      if (tool) {
        tool.args = (tool.args ?? "") + event.delta;
      }
      return next;
    }
    case "TOOL_CALL_END": {
      if (options.toolCallDetail === "none") {
        return next;
      }
      // END 本身不改 status；等 RESULT 或保持 running 直到 result。
      return next;
    }
    case "TOOL_CALL_RESULT": {
      if (options.toolCallDetail === "none") {
        return next;
      }
      const tool = findTool(next.items, event.toolCallId);
      if (tool) {
        tool.result = event.content;
        tool.status = looksLikeToolError(event.content) ? "error" : "success";
        tool.collapsed = shouldCollapseResult(event.content);
      }
      return next;
    }
    case "STEP_STARTED":
    case "STEP_FINISHED":
    case "STATE_SNAPSHOT":
    case "MESSAGES_SNAPSHOT":
    case "CUSTOM":
      return next;
    default: {
      // 穷尽检查：未知类型原样返回。
      return next;
    }
  }
}

/**
 * 标记连接断开（不做自动续传；G5 才做）。
 *
 * 参数:
 *   - state: 当前状态。
 *
 * 返回值:
 *   - connection=disconnected 的状态。
 */
export function markStreamDisconnected(
  state: StreamViewState,
): StreamViewState {
  return {
    ...state,
    connection: "disconnected",
  };
}

/**
 * 用户主动中断后的本地乐观回落。
 *
 * 参数:
 *   - state: 当前状态。
 *   - runStatus: cancel API 返回的状态。
 *
 * 返回值:
 *   - 回落为 cancelled/idle 的状态。
 */
export function applyLocalCancel(
  state: StreamViewState,
  runStatus: RunStatus,
): StreamViewState {
  const cancelled =
    runStatus === "cancelled" ||
    runStatus === "interrupted" ||
    runStatus === "success"
      ? "cancelled"
      : "cancelled";
  return {
    ...state,
    status: cancelled,
    items: finalizeStreaming(state.items.map(cloneItem)),
  };
}

/**
 * 切换工具结果折叠。
 *
 * 参数:
 *   - state: 当前状态。
 *   - toolCallId: 工具调用 id。
 *
 * 返回值:
 *   - 折叠态翻转后的状态。
 */
export function toggleToolCollapsed(
  state: StreamViewState,
  toolCallId: string,
): StreamViewState {
  return {
    ...state,
    items: state.items.map((item) => {
      if (item.kind === "tool" && item.id === toolCallId) {
        return { ...item, collapsed: !item.collapsed };
      }
      return item;
    }),
  };
}

/**
 * 追加本地用户消息（发送前乐观展示）。
 *
 * 参数:
 *   - state: 当前状态。
 *   - text: 用户输入。
 *   - id: 消息 id。
 *
 * 返回值:
 *   - 含用户气泡的状态。
 */
export function appendLocalUserMessage(
  state: StreamViewState,
  text: string,
  id: string,
): StreamViewState {
  return {
    ...state,
    items: [
      ...state.items,
      {
        kind: "message",
        id,
        role: "user",
        text,
        streaming: false,
      },
    ],
  };
}

function cloneItem(item: TimelineItem): TimelineItem {
  if (item.kind === "message") {
    return { ...item };
  }
  return { ...item };
}

function findMessage(
  items: TimelineItem[],
  messageId: string,
): TimelineMessage | undefined {
  return items.find(
    (item): item is TimelineMessage =>
      item.kind === "message" && item.id === messageId,
  );
}

function findTool(
  items: TimelineItem[],
  toolCallId: string,
): TimelineTool | undefined {
  return items.find(
    (item): item is TimelineTool =>
      item.kind === "tool" && item.id === toolCallId,
  );
}

function finalizeStreaming(items: TimelineItem[]): TimelineItem[] {
  return items.map((item) => {
    if (item.kind === "message") {
      return {
        ...item,
        streaming: false,
        thinkingStreaming: false,
      };
    }
    if (item.kind === "tool" && item.status === "running") {
      // 中断时工具可能无 RESULT；保持 running 会误导，标为 error 但不伪造 result。
      return { ...item, status: "error" as const };
    }
    return item;
  });
}

function formatUnknown(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function shouldCollapseResult(content: string): boolean {
  return content.length > 240 || content.split("\n").length > 6;
}

function looksLikeToolError(content: string): boolean {
  const head = content.slice(0, 80).toLowerCase();
  return (
    head.startsWith("error:") ||
    head.includes('"iserror":true') ||
    head.includes("is_error")
  );
}
