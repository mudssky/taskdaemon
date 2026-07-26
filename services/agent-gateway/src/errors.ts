/**
 * AGENT_* 错误与 HTTP envelope。
 */

import {
  AGENT_ERROR_HTTP_STATUS,
  type AgentErrorBody,
  type AgentErrorCode,
} from "@taskdaemon/agent-protocol";

/** 可抛出的业务错误。 */
export class AgentHttpError extends Error {
  readonly code: AgentErrorCode;
  readonly status: number;
  readonly details?: Record<string, unknown>;
  readonly traceId?: string;

  /**
   * 参数:
   *   - code: AGENT_* 错误码。
   *   - message: 人类可读说明。
   *   - options: details / traceId / cause。
   */
  constructor(
    code: AgentErrorCode,
    message: string,
    options?: {
      details?: Record<string, unknown>;
      traceId?: string;
      cause?: unknown;
    },
  ) {
    super(message, options?.cause ? { cause: options.cause } : undefined);
    this.name = "AgentHttpError";
    this.code = code;
    this.status = AGENT_ERROR_HTTP_STATUS[code];
    this.details = options?.details;
    this.traceId = options?.traceId;
  }
}

/**
 * 构造统一错误 envelope。
 *
 * 参数:
 *   - code: 错误码。
 *   - message: 说明。
 *   - traceId: 可选追踪 id。
 *   - details: 可选细节。
 *
 * 返回值:
 *   - AgentErrorBody。
 */
export function errorBody(
  code: AgentErrorCode,
  message: string,
  traceId?: string,
  details?: Record<string, unknown>,
): AgentErrorBody {
  return {
    error: {
      code,
      message,
      ...(details ? { details } : {}),
      ...(traceId ? { traceId } : {}),
    },
  };
}

/**
 * 从未知错误归一为 AgentHttpError。
 *
 * 参数:
 *   - err: 捕获值。
 *   - traceId: 请求 trace。
 *
 * 返回值:
 *   - AgentHttpError。
 */
export function toAgentHttpError(
  err: unknown,
  traceId?: string,
): AgentHttpError {
  if (err instanceof AgentHttpError) {
    if (traceId && !err.traceId) {
      return new AgentHttpError(err.code, err.message, {
        details: err.details,
        traceId,
        cause: err,
      });
    }
    return err;
  }
  const message = err instanceof Error ? err.message : "internal error";
  return new AgentHttpError("AGENT_INTERNAL_ERROR", message, { traceId });
}
