/**
 * AgentRuntime 内部事件与会话类型（adapter → gateway）。
 * 对外再映射为 AG-UI；接口本身禁止 Pi/OMP 专有命名。
 */

import type { AgentProfile } from "./capabilities.js";
import type { AgentMessage, MessageRole } from "./protocol.js";

/** 创建会话选项。 */
export type SessionOpts = {
  /** 工作目录；profile=general 或 workspaceBinding=false 时可省略。 */
  cwd?: string;
  /** 会话画像。 */
  profile: AgentProfile;
  /**
   * 是否绑定 workspace。
   * 缺省：coding → true，general → false。
   */
  workspaceBinding?: boolean;
  /** 工具策略。 */
  tools?: "none" | string[];
  /** 持久化策略。 */
  sessionPersistence?: "ephemeral" | "runtime-file";
  /** 初始模型。 */
  model?: string;
  thinkingLevel?: "off" | "low" | "medium" | "high";
  systemPrompt?: string;
  appendSystemPrompt?: string;
  /** 透传元数据（不得含密钥）。 */
  metadata?: Record<string, unknown>;
};

/** 会话句柄（gateway 持有；不对浏览器暴露 runtime 私有路径细节）。 */
export type SessionHandle = {
  sessionId: string;
  runtimeId: string;
  /** runtime 会话文件路径（仅 gateway 索引，可选）。 */
  sessionFile?: string;
  createdAt: string;
};

/** 用户/系统 prompt 输入。 */
export type PromptInput = {
  text?: string;
  messages?: AgentMessage[];
  metadata?: Record<string, unknown>;
};

/** 模型描述。 */
export type ModelInfo = {
  id: string;
  name?: string;
  provider?: string;
  /** 不得包含 Authorization 等密钥字段。 */
  metadata?: Record<string, unknown>;
};

/** 模型引用（setModel）。 */
export type ModelRef = {
  id: string;
  provider?: string;
};

/** 历史分页。 */
export type HistoryPage = {
  offset?: number;
  limit?: number;
};

/** 历史页结果。 */
export type MessagePage = {
  messages: AgentMessage[];
  offset: number;
  limit: number;
  hasMore: boolean;
};

/**
 * 会话状态快照。
 * 不变量：adapter 返回前必须剥离密钥（headers.Authorization 等）。
 */
export type RuntimeState = {
  sessionId: string;
  isStreaming: boolean;
  model?: ModelInfo;
  messageCount?: number;
  /** 已消毒的扩展字段；禁止 raw provider headers。 */
  extensions?: Record<string, unknown>;
};

/** 归一化 runtime 事件（adapter 产出，gateway 映射 AG-UI）。 */
export type RuntimeEvent =
  | {
      type: "run_started";
      sessionId: string;
      runId?: string;
      timestamp?: string;
    }
  | {
      type: "run_finished";
      sessionId: string;
      runId?: string;
      timestamp?: string;
    }
  | {
      type: "run_error";
      sessionId: string;
      runId?: string;
      message: string;
      code?: string;
      timestamp?: string;
    }
  | {
      type: "step_started";
      sessionId: string;
      stepName: string;
      timestamp?: string;
    }
  | {
      type: "step_finished";
      sessionId: string;
      stepName: string;
      timestamp?: string;
    }
  | {
      type: "message_start";
      sessionId: string;
      messageId: string;
      role: MessageRole;
      timestamp?: string;
    }
  | {
      type: "message_text_delta";
      sessionId: string;
      messageId: string;
      delta: string;
      timestamp?: string;
    }
  | {
      type: "message_thinking_delta";
      sessionId: string;
      messageId: string;
      delta: string;
      timestamp?: string;
    }
  | {
      type: "message_end";
      sessionId: string;
      messageId: string;
      timestamp?: string;
    }
  | {
      type: "tool_call_start";
      sessionId: string;
      toolCallId: string;
      toolName: string;
      parentMessageId?: string;
      timestamp?: string;
    }
  | {
      type: "tool_call_args_delta";
      sessionId: string;
      toolCallId: string;
      delta: string;
      timestamp?: string;
    }
  | {
      type: "tool_call_end";
      sessionId: string;
      toolCallId: string;
      timestamp?: string;
    }
  | {
      type: "tool_call_result";
      sessionId: string;
      toolCallId: string;
      content: string;
      isError?: boolean;
      timestamp?: string;
    }
  | {
      type: "usage";
      sessionId: string;
      inputTokens?: number;
      outputTokens?: number;
      totalTokens?: number;
      model?: string;
      timestamp?: string;
    }
  | {
      type: "file_change";
      sessionId: string;
      path: string;
      kind: "create" | "modify" | "delete";
      /** coding profile 专用；general 下 adapter 不得发出。 */
      diff?: string;
      timestamp?: string;
    }
  | {
      type: "raw_ignored";
      sessionId: string;
      reason: string;
      sourceType?: string;
      timestamp?: string;
    };
