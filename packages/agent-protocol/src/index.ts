/**
 * @taskdaemon/agent-protocol
 *
 * C-3 / C-4 冻结产物：types only，零运行时依赖。
 * 消费方：agent-gateway / agent-web / 未来外部网关类型对齐。
 */

export type {
  AguiBaseEvent,
  AguiCustomEvent,
  AguiEvent,
  AguiEventType,
  AguiMessagesSnapshotEvent,
  AguiReasoningContentEvent,
  AguiReasoningEndEvent,
  AguiReasoningStartEvent,
  AguiRunErrorEvent,
  AguiRunFinishedEvent,
  AguiRunStartedEvent,
  AguiStateSnapshotEvent,
  AguiStepFinishedEvent,
  AguiStepStartedEvent,
  AguiTextMessageContentEvent,
  AguiTextMessageEndEvent,
  AguiTextMessageStartEvent,
  AguiToolCallArgsEvent,
  AguiToolCallEndEvent,
  AguiToolCallResultEvent,
  AguiToolCallStartEvent,
} from "./agui.js";
export {
  type AgentProfile,
  type AttachMode,
  type ColdStartCost,
  type HistoryCapability,
  type MemoryClass,
  type ModelSwitchMode,
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
  PI_SDK_CAPABILITIES,
  type RuntimeCapabilities,
  type SessionPersistenceMode,
  supportsProfile,
  type ToolCallDetail,
} from "./capabilities.js";
export {
  normalizePageQuery,
  type PageQuery,
  type PageResult,
  type ResourceSort,
  type SortDirection,
} from "./common.js";
export {
  AGENT_ERROR_HTTP_STATUS,
  type AgentErrorBody,
  type AgentErrorCode,
  type AgentMessage,
  type CancelRunRequest,
  type CancelRunResponse,
  type CreateRunRequest,
  type CreateThreadRequest,
  type FileChangePayload,
  type ForkThreadRequest,
  type HitlDecision,
  type HitlRequestPayload,
  type HitlRespondRequest,
  type HitlRespondResponse,
  type ListMessagesQuery,
  type ListMessagesResponse,
  type ListRunsQuery,
  type ListRunsResponse,
  type ListThreadsQuery,
  type ListThreadsResponse,
  type MessageContentBlock,
  type MessageRole,
  type Run,
  type RunStatus,
  type SteerRequest,
  type SteerResponse,
  type StreamReconnectQuery,
  type Thread,
  type ThreadCapabilitiesResponse,
  type ThreadStatus,
  type UpdateThreadRequest,
  type UsagePayload,
} from "./protocol.js";
export {
  AGENT_API_PREFIX,
  type AgentRouteKey,
  AgentRoutes,
} from "./routes.js";
export {
  type AgentRuntime,
  hasOptionalRuntimeMethod,
} from "./runtime.js";
export type {
  HistoryPage,
  MessagePage,
  ModelInfo,
  ModelRef,
  PromptInput,
  RuntimeEvent,
  RuntimeState,
  SessionHandle,
  SessionOpts,
} from "./runtime-types.js";
export {
  type AuditEvent,
  DEV_SINGLE_PRINCIPAL,
  type Principal,
  type PrincipalResolver,
  type PrincipalResolverMode,
  TRUST_HEADERS,
  type TrustEventEmitter,
  type TrustHeaderName,
  type UsageEvent,
} from "./trust.js";

import type { AguiEvent as _AguiEventSmoke } from "./agui.js";
import type { RuntimeCapabilities as _CapsSmoke } from "./capabilities.js";
import type { Thread as _ThreadSmoke } from "./protocol.js";
/**
 * 消费方 import 冒烟：保证公共导出可被类型系统解析。
 * 运行 `pnpm --filter @taskdaemon/agent-protocol typecheck` 即验证。
 */
import type { AgentRuntime as _AgentRuntimeSmoke } from "./runtime.js";
import type { Principal as _PrincipalSmoke } from "./trust.js";

export type _PublicExportSmoke = {
  runtime: _AgentRuntimeSmoke;
  event: _AguiEventSmoke;
  caps: _CapsSmoke;
  principal: _PrincipalSmoke;
  thread: _ThreadSmoke;
};
