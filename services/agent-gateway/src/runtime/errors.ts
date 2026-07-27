/**
 * Adapter 层结构化错误（AGENT_*）。
 * gateway 路由层可直接映射 HTTP 状态。
 */

import type { AgentErrorCode } from "@taskdaemon/agent-protocol";

/** 可选能力方法名。 */
export type OptionalRuntimeMethod =
  | "steer"
  | "followUp"
  | "setModel"
  | "listModels";

/**
 * Runtime adapter 抛出的错误。
 *
 * 参数语义:
 *   - code: AGENT_* 错误码。
 *   - message: 人类可读摘要（不得含密钥）。
 *   - details: 可选结构化细节。
 *   - cause: 原始错误。
 */
export class AgentRuntimeError extends Error {
  readonly code: AgentErrorCode;
  readonly details?: Record<string, unknown>;

  /**
   * 创建 AgentRuntimeError。
   *
   * 参数:
   *   - code: 错误码。
   *   - message: 摘要。
   *   - options: details / cause。
   *
   * 返回值:
   *   - 无（构造函数）。
   */
  constructor(
    code: AgentErrorCode,
    message: string,
    options?: {
      details?: Record<string, unknown>;
      cause?: unknown;
    },
  ) {
    super(
      message,
      options?.cause !== undefined ? { cause: options.cause } : undefined,
    );
    this.name = "AgentRuntimeError";
    this.code = code;
    if (options?.details) this.details = options.details;
  }
}

/**
 * 构造「能力未声明」错误。
 *
 * 参数:
 *   - runtimeId: adapter id。
 *   - method: 被调用的可选方法。
 *
 * 返回值:
 *   - AgentRuntimeError(AGENT_CAPABILITY_UNSUPPORTED)。
 */
export function capabilityUnsupportedError(
  runtimeId: string,
  method: OptionalRuntimeMethod,
): AgentRuntimeError {
  return new AgentRuntimeError(
    "AGENT_CAPABILITY_UNSUPPORTED",
    `runtime ${runtimeId} does not support ${method}`,
    { details: { runtimeId, method } },
  );
}

/**
 * 会话不存在。
 *
 * 参数:
 *   - sessionId: 会话 id。
 *
 * 返回值:
 *   - AgentRuntimeError。
 */
export function sessionNotFoundError(sessionId: string): AgentRuntimeError {
  return new AgentRuntimeError(
    "AGENT_THREAD_NOT_FOUND",
    `session not found: ${sessionId}`,
    { details: { sessionId } },
  );
}

/**
 * 将未知错误包装为 AGENT_INTERNAL_ERROR。
 *
 * 参数:
 *   - err: 原始错误。
 *   - fallbackMessage: 无 message 时的回退文案。
 *
 * 返回值:
 *   - AgentRuntimeError。
 */
export function wrapInternalError(
  err: unknown,
  fallbackMessage = "runtime internal error",
): AgentRuntimeError {
  if (err instanceof AgentRuntimeError) return err;
  const message =
    err instanceof Error && err.message ? err.message : fallbackMessage;
  return new AgentRuntimeError("AGENT_INTERNAL_ERROR", message, {
    cause: err,
  });
}
