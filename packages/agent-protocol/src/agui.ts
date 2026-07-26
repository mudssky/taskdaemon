/**
 * AG-UI 事件子集类型（对外 SSE 载荷）。
 *
 * 参考：https://docs.ag-ui.com/concepts/events （2026-07-27 快照）。
 * 仅冻结本仓库 MVP 发出的事件；完整 AG-UI 枚举不照搬。
 */

/** AG-UI 事件 type 字符串（SCREAMING_SNAKE 对齐上游）。 */
export type AguiEventType =
  | "RUN_STARTED"
  | "RUN_FINISHED"
  | "RUN_ERROR"
  | "STEP_STARTED"
  | "STEP_FINISHED"
  | "TEXT_MESSAGE_START"
  | "TEXT_MESSAGE_CONTENT"
  | "TEXT_MESSAGE_END"
  | "TOOL_CALL_START"
  | "TOOL_CALL_ARGS"
  | "TOOL_CALL_END"
  | "TOOL_CALL_RESULT"
  | "REASONING_START"
  | "REASONING_CONTENT"
  | "REASONING_END"
  | "STATE_SNAPSHOT"
  | "MESSAGES_SNAPSHOT"
  | "CUSTOM";

/** 所有 AG-UI 事件的公共字段。 */
export type AguiBaseEvent = {
  type: AguiEventType;
  /** Unix ms；可选。 */
  timestamp?: number;
  /**
   * SSE id / 游标。gateway 必须为可重连事件填充，供 Last-Event-ID 使用。
   */
  id?: string;
  /** 原始 runtime 事件（调试用，生产可剥离）。 */
  rawEvent?: unknown;
};

export type AguiRunStartedEvent = AguiBaseEvent & {
  type: "RUN_STARTED";
  threadId: string;
  runId: string;
};

export type AguiRunFinishedEvent = AguiBaseEvent & {
  type: "RUN_FINISHED";
  threadId: string;
  runId: string;
  outcome?: { type: "success" } | { type: "interrupt"; reason?: string };
};

export type AguiRunErrorEvent = AguiBaseEvent & {
  type: "RUN_ERROR";
  threadId: string;
  runId: string;
  message: string;
  code?: string;
};

export type AguiStepStartedEvent = AguiBaseEvent & {
  type: "STEP_STARTED";
  stepName: string;
};

export type AguiStepFinishedEvent = AguiBaseEvent & {
  type: "STEP_FINISHED";
  stepName: string;
};

export type AguiTextMessageStartEvent = AguiBaseEvent & {
  type: "TEXT_MESSAGE_START";
  messageId: string;
  role: "assistant" | "user" | "system" | "tool" | "developer";
};

export type AguiTextMessageContentEvent = AguiBaseEvent & {
  type: "TEXT_MESSAGE_CONTENT";
  messageId: string;
  delta: string;
};

export type AguiTextMessageEndEvent = AguiBaseEvent & {
  type: "TEXT_MESSAGE_END";
  messageId: string;
};

export type AguiToolCallStartEvent = AguiBaseEvent & {
  type: "TOOL_CALL_START";
  toolCallId: string;
  toolCallName: string;
  parentMessageId?: string;
};

export type AguiToolCallArgsEvent = AguiBaseEvent & {
  type: "TOOL_CALL_ARGS";
  toolCallId: string;
  delta: string;
};

export type AguiToolCallEndEvent = AguiBaseEvent & {
  type: "TOOL_CALL_END";
  toolCallId: string;
};

export type AguiToolCallResultEvent = AguiBaseEvent & {
  type: "TOOL_CALL_RESULT";
  toolCallId: string;
  content: string;
  messageId?: string;
};

/** thinking → REASONING_*（AG-UI 已弃用 THINKING_*）。 */
export type AguiReasoningStartEvent = AguiBaseEvent & {
  type: "REASONING_START";
  messageId: string;
};

export type AguiReasoningContentEvent = AguiBaseEvent & {
  type: "REASONING_CONTENT";
  messageId: string;
  delta: string;
};

export type AguiReasoningEndEvent = AguiBaseEvent & {
  type: "REASONING_END";
  messageId: string;
};

export type AguiStateSnapshotEvent = AguiBaseEvent & {
  type: "STATE_SNAPSHOT";
  snapshot: Record<string, unknown>;
};

export type AguiMessagesSnapshotEvent = AguiBaseEvent & {
  type: "MESSAGES_SNAPSHOT";
  messages: unknown[];
};

/**
 * 扩展事件（coding 文件变更、HITL、用量等）。
 * name 使用稳定字符串；前端按 capabilities / profile 决定是否渲染。
 * G5 稳定名：file_change | usage | hitl_request。
 */
export type AguiCustomEvent = AguiBaseEvent & {
  type: "CUSTOM";
  name: "file_change" | "usage" | "hitl_request" | string;
  value: unknown;
};

export type AguiEvent =
  | AguiRunStartedEvent
  | AguiRunFinishedEvent
  | AguiRunErrorEvent
  | AguiStepStartedEvent
  | AguiStepFinishedEvent
  | AguiTextMessageStartEvent
  | AguiTextMessageContentEvent
  | AguiTextMessageEndEvent
  | AguiToolCallStartEvent
  | AguiToolCallArgsEvent
  | AguiToolCallEndEvent
  | AguiToolCallResultEvent
  | AguiReasoningStartEvent
  | AguiReasoningContentEvent
  | AguiReasoningEndEvent
  | AguiStateSnapshotEvent
  | AguiMessagesSnapshotEvent
  | AguiCustomEvent;
