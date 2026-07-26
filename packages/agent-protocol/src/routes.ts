/**
 * REST 路径常量（Agent Protocol 子集）。
 * 前缀 /v1 由 gateway 挂载；此处为相对资源路径。
 */

export const AGENT_API_PREFIX = "/v1" as const;

export const AgentRoutes = {
  threads: "/threads",
  thread: "/threads/:threadId",
  threadMessages: "/threads/:threadId/messages",
  threadCapabilities: "/threads/:threadId/capabilities",
  threadRuns: "/threads/:threadId/runs",
  threadRun: "/threads/:threadId/runs/:runId",
  threadRunCancel: "/threads/:threadId/runs/:runId/cancel",
  threadRunStream: "/threads/:threadId/runs/:runId/stream",
  /** 创建 run 并立即 SSE 流式返回。 */
  threadRunsStream: "/threads/:threadId/runs/stream",
  /** 运行中 steer（可选，capabilities.steering）。 */
  threadSteer: "/threads/:threadId/steer",
  /** follow-up（可选）。 */
  threadFollowUp: "/threads/:threadId/follow-up",
  /** 会话元数据更新（G5：重命名 / 归档）。 */
  threadUpdate: "/threads/:threadId",
  /** 会话分叉（G5）。 */
  threadFork: "/threads/:threadId/fork",
  /** HITL 决策响应（G5）。 */
  threadHitl: "/threads/:threadId/hitl",
} as const;

export type AgentRouteKey = keyof typeof AgentRoutes;
