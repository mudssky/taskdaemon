/**
 * AG-UI 事件流 → 聊天时间线归约（G4 核心 + G5 扩展）。
 * 纯函数，无 I/O；不伪造 runtime 未提供的字段。
 */

import type {
  AgentMessage,
  AguiEvent,
  FileChangePayload,
  HitlRequestPayload,
  MessageRole,
  RunStatus,
  UsagePayload,
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
  /** 推理区默认收起。 */
  thinkingCollapsed?: boolean;
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

/** 运行中 steering 注入（本地时间线标记）。 */
export type TimelineSteer = {
  kind: "steer";
  id: string;
  text: string;
};

/** HITL 决策点。 */
export type TimelineHitl = {
  kind: "hitl";
  id: string;
  request: HitlRequestPayload;
  status: "awaiting" | "approved" | "rejected" | "modified" | "timed_out";
  modifiedText?: string;
};

export type TimelineItem =
  | TimelineMessage
  | TimelineTool
  | TimelineSteer
  | TimelineHitl;

/** 工作区文件变更条目。 */
export type FileChangeItem = FileChangePayload & {
  id: string;
};

/** 单次 run / 会话视图的流状态。 */
export type StreamViewState = {
  runId: string | null;
  /** UI 运行态：idle / running / error / cancelled / awaiting_human。 */
  status: "idle" | "running" | "error" | "cancelled" | "awaiting_human";
  items: TimelineItem[];
  error?: {
    message: string;
    code?: string;
    traceId?: string;
  };
  lastEventId?: string;
  /** 已见事件 id（续传去重）。 */
  seenEventIds: string[];
  /** 连接层状态（与 run status 正交）。 */
  connection:
    | "connected"
    | "connecting"
    | "reconnecting"
    | "disconnected"
    | "cursor_invalid";
  fileChanges: FileChangeItem[];
  usage?: UsagePayload;
  pendingHitlId?: string;
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
    seenEventIds: [],
    connection: "connected",
    fileChanges: [],
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
        thinkingCollapsed: true,
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
  // 续传去重：已见 id 直接跳过（保留 lastEventId 不回退）。
  if (event.id && state.seenEventIds.includes(event.id)) {
    return state;
  }

  const next: StreamViewState = {
    ...state,
    items: state.items.map(cloneItem),
    fileChanges: state.fileChanges.map((f) => ({ ...f })),
    seenEventIds: [...state.seenEventIds],
    error: state.error ? { ...state.error } : undefined,
    usage: state.usage ? { ...state.usage } : undefined,
  };

  if (event.id) {
    next.lastEventId = event.id;
    next.seenEventIds.push(event.id);
    // 环缓：只保留最近 2000 个 id，防内存无限涨。
    if (next.seenEventIds.length > 2000) {
      next.seenEventIds = next.seenEventIds.slice(-1500);
    }
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
      next.pendingHitlId = undefined;
      return next;
    }
    case "RUN_ERROR": {
      next.status = "error";
      next.error = {
        message: event.message,
        code: event.code,
      };
      next.items = finalizeStreaming(next.items);
      next.pendingHitlId = undefined;
      return next;
    }
    case "TEXT_MESSAGE_START": {
      next.items.push({
        kind: "message",
        id: event.messageId,
        role: event.role,
        text: "",
        streaming: true,
        thinkingCollapsed: true,
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
          thinkingCollapsed: true,
        };
        next.items.push(msg);
      } else {
        msg.thinking = msg.thinking ?? "";
        msg.thinkingStreaming = true;
        if (msg.thinkingCollapsed === undefined) {
          msg.thinkingCollapsed = true;
        }
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
    case "CUSTOM": {
      return reduceCustomEvent(next, event.name, event.value, event.id);
    }
    case "STEP_STARTED":
    case "STEP_FINISHED":
    case "STATE_SNAPSHOT":
    case "MESSAGES_SNAPSHOT":
      return next;
    default: {
      return next;
    }
  }
}

/**
 * 标记连接断开（G5 自动续传前的瞬时态）。
 *
 * 参数:
 *   - state: 当前状态。
 *   - mode: disconnected | reconnecting | cursor_invalid。
 *
 * 返回值:
 *   - 更新 connection 的状态。
 */
export function markStreamConnection(
  state: StreamViewState,
  mode: StreamViewState["connection"],
): StreamViewState {
  return {
    ...state,
    connection: mode,
  };
}

/**
 * 兼容 G4 命名：标记断开。
 *
 * 参数:
 *   - state: 当前状态。
 *
 * 返回值:
 *   - connection=disconnected。
 */
export function markStreamDisconnected(
  state: StreamViewState,
): StreamViewState {
  return markStreamConnection(state, "disconnected");
}

/**
 * 用户主动中断后的本地乐观回落。
 *
 * 参数:
 *   - state: 当前状态。
 *   - runStatus: cancel API 返回的状态。
 *
 * 返回值:
 *   - 回落为 cancelled 的状态。
 */
export function applyLocalCancel(
  state: StreamViewState,
  runStatus: RunStatus,
): StreamViewState {
  void runStatus;
  return {
    ...state,
    status: "cancelled",
    pendingHitlId: undefined,
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
 * 切换 thinking 折叠（默认收起）。
 *
 * 参数:
 *   - state: 当前状态。
 *   - messageId: 消息 id。
 *
 * 返回值:
 *   - thinkingCollapsed 翻转后的状态。
 */
export function toggleThinkingCollapsed(
  state: StreamViewState,
  messageId: string,
): StreamViewState {
  return {
    ...state,
    items: state.items.map((item) => {
      if (item.kind === "message" && item.id === messageId) {
        return {
          ...item,
          thinkingCollapsed: !(item.thinkingCollapsed ?? true),
        };
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

/**
 * 追加 steering 时间线条目（不中断 run）。
 *
 * 参数:
 *   - state: 当前状态。
 *   - text: 注入文本。
 *   - id: 条目 id。
 *
 * 返回值:
 *   - 含 steer 条目的状态。
 */
export function appendSteerMessage(
  state: StreamViewState,
  text: string,
  id: string,
): StreamViewState {
  return {
    ...state,
    items: [
      ...state.items,
      {
        kind: "steer",
        id,
        text,
      },
    ],
  };
}

/**
 * 应用 HITL 本地决策（乐观更新）。
 *
 * 参数:
 *   - state: 当前状态。
 *   - requestId: HITL 请求 id。
 *   - decision: approve | reject | modify。
 *   - modifiedText: modify 时的文本。
 *
 * 返回值:
 *   - 更新后的状态。
 */
export function applyHitlDecision(
  state: StreamViewState,
  requestId: string,
  decision: "approve" | "reject" | "modify",
  modifiedText?: string,
): StreamViewState {
  const status =
    decision === "approve"
      ? "approved"
      : decision === "reject"
        ? "rejected"
        : "modified";
  return {
    ...state,
    status: "running",
    pendingHitlId: undefined,
    items: state.items.map((item) => {
      if (item.kind === "hitl" && item.id === requestId) {
        return {
          ...item,
          status,
          modifiedText,
        };
      }
      return item;
    }),
  };
}

/**
 * 标记 HITL 超时。
 *
 * 参数:
 *   - state: 当前状态。
 *   - requestId: HITL id。
 *
 * 返回值:
 *   - timed_out 状态。
 */
export function markHitlTimedOut(
  state: StreamViewState,
  requestId: string,
): StreamViewState {
  if (state.pendingHitlId !== requestId) {
    return state;
  }
  return {
    ...state,
    status: "error",
    pendingHitlId: undefined,
    error: {
      message: "等待人工确认已超时",
      code: "AGENT_HITL_TIMEOUT",
    },
    items: state.items.map((item) => {
      if (item.kind === "hitl" && item.id === requestId) {
        return { ...item, status: "timed_out" as const };
      }
      return item;
    }),
  };
}

function reduceCustomEvent(
  next: StreamViewState,
  name: string,
  value: unknown,
  eventId?: string,
): StreamViewState {
  if (name === "file_change") {
    const payload = value as FileChangePayload;
    if (!payload || typeof payload.path !== "string") {
      return next;
    }
    next.fileChanges.push({
      id: eventId ?? `fc_${next.fileChanges.length}`,
      path: payload.path,
      kind: payload.kind,
      diff: payload.diff,
      outsideSandbox: payload.outsideSandbox,
    });
    return next;
  }

  if (name === "usage") {
    const payload = value as UsagePayload;
    if (payload && typeof payload === "object") {
      next.usage = { ...payload };
    }
    return next;
  }

  if (name === "hitl_request") {
    const payload = value as HitlRequestPayload;
    if (!payload || typeof payload.requestId !== "string") {
      return next;
    }
    next.items.push({
      kind: "hitl",
      id: payload.requestId,
      request: payload,
      status: "awaiting",
    });
    next.pendingHitlId = payload.requestId;
    next.status = "awaiting_human";
    return next;
  }

  // 未知 CUSTOM：忽略，不发明字段。
  return next;
}

function cloneItem(item: TimelineItem): TimelineItem {
  if (item.kind === "message") {
    return { ...item };
  }
  if (item.kind === "tool") {
    return { ...item };
  }
  if (item.kind === "steer") {
    return { ...item };
  }
  return {
    ...item,
    request: { ...item.request },
  };
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
