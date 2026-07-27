/**
 * Agent Protocol 子集 — Thread / Message / Run DTO 与错误形态。
 *
 * 参考：Agent Protocol OpenAPI 0.1.6（2026-07-27 快照）。
 * 本仓库只实现 threads / runs / stream / cancel / messages；不做 Store / Agents / 全量 stream 原语。
 */

import type { AgentProfile, RuntimeCapabilities } from "./capabilities.js";
import type { PageQuery, PageResult, ResourceSort } from "./common.js";

/** Thread 生命周期状态（gateway 视角，非 runtime 私有状态）。 */
export type ThreadStatus =
  | "idle"
  | "busy"
  | "interrupted"
  | "error"
  | "archived"
  | "deleted";

/** Run 生命周期状态。 */
export type RunStatus =
  | "pending"
  | "running"
  | "success"
  | "error"
  | "cancelled"
  | "interrupted";

/** 消息角色（对齐 LLM 通用子集，非 Pi/OMP 专有命名）。 */
export type MessageRole =
  | "system"
  | "user"
  | "assistant"
  | "tool"
  | "developer";

/** 消息内容块。 */
export type MessageContentBlock =
  | { type: "text"; text: string }
  | { type: "thinking"; text: string }
  | { type: "tool_use"; toolCallId: string; name: string; args?: unknown }
  | {
      type: "tool_result";
      toolCallId: string;
      content: string;
      isError?: boolean;
    };

/** 会话内一条消息。 */
export type AgentMessage = {
  id: string;
  role: MessageRole;
  content: MessageContentBlock[];
  createdAt: string;
  metadata?: Record<string, unknown>;
};

/** Thread 元数据（gateway 薄索引；transcript 权威在 runtime session 文件）。 */
export type Thread = {
  threadId: string;
  status: ThreadStatus;
  /** 绑定的 runtime id，如 `pi` / `omp`（中立字符串，非枚举硬编码）。 */
  runtimeId: string;
  /** 会话 profile。 */
  profile: AgentProfile;
  /** 是否绑定 workspace；false 表示 general/无文件系统会话。 */
  workspaceBound: boolean;
  /** workspace 根路径；workspaceBound=false 时必须省略或 null。 */
  workspaceRoot?: string | null;
  /** 当前模型 id（若可知）。 */
  model?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
  /** 租户分区键（来自 X-Tenant-Id）。 */
  tenantId: string;
  /** 创建主体（来自 X-Auth-Subject）。 */
  subject: string;
};

/** 创建 Thread 请求体。 */
export type CreateThreadRequest = {
  /** 可选客户端指定 id；缺省由 gateway 生成 UUID。 */
  threadId?: string;
  /** runtime 选择；缺省 gateway 默认（通常 pi）。 */
  runtimeId?: string;
  /** coding | general；缺省 coding。 */
  profile?: AgentProfile;
  /**
   * workspace 根。profile=general 时可省略；
   * 省略且 profile=coding 时由 gateway 分配沙箱根。
   */
  workspaceRoot?: string | null;
  /** 强制关闭 workspace 绑定（即使 profile=coding）。 */
  workspaceBinding?: boolean;
  model?: string;
  thinkingLevel?: "off" | "low" | "medium" | "high";
  /** 工具策略：none | 允许名列表；透传 SessionOpts，不做二次封装。 */
  tools?: "none" | string[];
  systemPrompt?: string;
  appendSystemPrompt?: string;
  metadata?: Record<string, unknown>;
};

/** Thread 列表过滤。 */
export type ListThreadsQuery = PageQuery & {
  status?: ThreadStatus;
  runtimeId?: string;
  profile?: AgentProfile;
  sort?: ResourceSort;
  /** 标题与消息内容搜索（G5；gateway 可降级为仅标题）。 */
  q?: string;
  /** 为 true 时包含 archived；默认 false。 */
  includeArchived?: boolean;
};

/**
 * 更新 Thread 元数据（G5 会话管理）。
 * title 写入 metadata.title；archived=true 时 status→archived。
 */
export type UpdateThreadRequest = {
  title?: string;
  /** true=归档，false=从归档恢复为 idle。 */
  archived?: boolean;
  metadata?: Record<string, unknown>;
};

/** 从某条消息分叉新会话（G5）。 */
export type ForkThreadRequest = {
  /** 分叉点消息 id；缺省复制全部历史。 */
  fromMessageId?: string;
  /** 新会话标题；缺省自动生成。 */
  title?: string;
};

/** 运行中注入（与 CreateRunRequest.input 同形子集）。 */
export type SteerRequest = {
  input: {
    text?: string;
    messages?: AgentMessage[];
    metadata?: Record<string, unknown>;
  };
};

export type SteerResponse = {
  ok: true;
  threadId: string;
};

/** HITL 决策（G5；对应 CUSTOM hitl_request）。 */
export type HitlDecision = "approve" | "reject" | "modify";

export type HitlRespondRequest = {
  requestId: string;
  decision: HitlDecision;
  /** decision=modify 时的改写输入。 */
  modifiedInput?: {
    text?: string;
    metadata?: Record<string, unknown>;
  };
};

export type HitlRespondResponse = {
  ok: true;
  requestId: string;
  decision: HitlDecision;
};

/** CUSTOM hitl_request 的 value 形状。 */
export type HitlRequestPayload = {
  requestId: string;
  title: string;
  context: string;
  options?: Array<{ id: string; label: string }>;
  /** 可选超时毫秒；缺省无超时。 */
  timeoutMs?: number;
};

/** CUSTOM file_change 的 value 形状。 */
export type FileChangePayload = {
  path: string;
  kind: "create" | "modify" | "delete";
  /** 可选 unified diff 文本；只读预览。 */
  diff?: string;
  /** 越出沙箱根时为 true。 */
  outsideSandbox?: boolean;
};

/** CUSTOM usage 的 value 形状。 */
export type UsagePayload = {
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
};

export type ListThreadsResponse = PageResult<Thread>;

/** 创建 Run 请求体（background 或 stream 共用核心字段）。 */
export type CreateRunRequest = {
  /** 用户输入；与 messages 二选一或并用（gateway 归一为 PromptInput）。 */
  input?: {
    text?: string;
    messages?: AgentMessage[];
    metadata?: Record<string, unknown>;
  };
  /** 可选：本 run 覆盖模型（仅当 capabilities.modelSwitch 支持）。 */
  model?: string;
  metadata?: Record<string, unknown>;
  /**
   * 断线行为提示（Agent Protocol on_disconnect 子集）。
   * cancel = 断开即 abort；continue = 后台继续（默认 continue）。
   */
  onDisconnect?: "cancel" | "continue";
};

/** Run 资源。 */
export type Run = {
  runId: string;
  threadId: string;
  status: RunStatus;
  createdAt: string;
  updatedAt: string;
  startedAt?: string;
  endedAt?: string;
  error?: {
    code: string;
    message: string;
  };
  metadata?: Record<string, unknown>;
};

export type ListRunsQuery = PageQuery & {
  status?: RunStatus;
  sort?: ResourceSort;
};

export type ListRunsResponse = PageResult<Run>;

export type ListMessagesQuery = PageQuery;

export type ListMessagesResponse = PageResult<AgentMessage>;

/**
 * 取消 Run 请求。
 * action 固定为 interrupt（真停可续，对齐 G0 abort 语义）。
 */
export type CancelRunRequest = {
  /** 预留；当前仅支持 interrupt。 */
  action?: "interrupt";
};

export type CancelRunResponse = {
  runId: string;
  status: RunStatus;
};

/**
 * 流式重连查询。
 * Last-Event-ID / cursor 语义见 docs/agent-contracts/agui-event-mapping.md。
 */
export type StreamReconnectQuery = {
  /** 上次收到的事件游标（gateway 签发的单调序列或 SSE id）。 */
  cursor?: string;
  /** 兼容 SSE Last-Event-ID。 */
  lastEventId?: string;
};

/** Thread 上透出的 capabilities 响应。 */
export type ThreadCapabilitiesResponse = {
  threadId: string;
  runtimeId: string;
  capabilities: RuntimeCapabilities;
};

/**
 * 稳定错误 envelope（前缀 AGENT_*，见路线图 §9.1）。
 */
export type AgentErrorBody = {
  error: {
    code: AgentErrorCode;
    message: string;
    details?: Record<string, unknown>;
    traceId?: string;
  };
};

/** G1 冻结的错误码子集；G2/G6 可 append-only 扩展。 */
export type AgentErrorCode =
  | "AGENT_VALIDATION_FAILED"
  | "AGENT_UNAUTHORIZED"
  | "AGENT_FORBIDDEN"
  | "AGENT_THREAD_NOT_FOUND"
  | "AGENT_RUN_NOT_FOUND"
  | "AGENT_RUN_CONFLICT"
  | "AGENT_RUN_CANCELLED"
  | "AGENT_RUNTIME_UNAVAILABLE"
  | "AGENT_CAPABILITY_UNSUPPORTED"
  | "AGENT_STREAM_CURSOR_INVALID"
  | "AGENT_INTERNAL_ERROR"
  // G6 MCP / 双引擎
  | "MCP_TOOL_NOT_ALLOWED"
  | "MCP_SERVER_UNAVAILABLE"
  | "MCP_VALIDATION_FAILED"
  | "MCP_LOOP_DETECTED"
  | "MCP_TASK_NOT_ALLOWED";

/** HTTP 状态与错误码建议映射（实现侧参考，非运行时代码）。 */
export const AGENT_ERROR_HTTP_STATUS: Record<AgentErrorCode, number> = {
  AGENT_VALIDATION_FAILED: 422,
  AGENT_UNAUTHORIZED: 401,
  AGENT_FORBIDDEN: 403,
  AGENT_THREAD_NOT_FOUND: 404,
  AGENT_RUN_NOT_FOUND: 404,
  AGENT_RUN_CONFLICT: 409,
  AGENT_RUN_CANCELLED: 409,
  AGENT_RUNTIME_UNAVAILABLE: 503,
  AGENT_CAPABILITY_UNSUPPORTED: 400,
  AGENT_STREAM_CURSOR_INVALID: 400,
  AGENT_INTERNAL_ERROR: 500,
  MCP_TOOL_NOT_ALLOWED: 403,
  MCP_SERVER_UNAVAILABLE: 503,
  MCP_VALIDATION_FAILED: 422,
  MCP_LOOP_DETECTED: 409,
  MCP_TASK_NOT_ALLOWED: 403,
};
