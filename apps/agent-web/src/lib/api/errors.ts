/**
 * Agent API 错误与通用 envelope 解析。
 */

export class AgentApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;
  readonly traceId: string | null;

  constructor(
    status: number,
    code: string,
    message: string,
    details: unknown,
    traceId: string | null,
  ) {
    super(message);
    this.name = "AgentApiError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.traceId = traceId;
  }
}

type AgentErrorEnvelope = {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
    traceId?: string;
  };
};

/**
 * 从失败 HTTP 响应构造 AgentApiError。
 *
 * 参数:
 *   - response: fetch Response。
 *   - body: 已解析 JSON 或 null。
 *
 * 返回值:
 *   - AgentApiError。
 */
export function errorFromResponse(
  response: Response,
  body: unknown,
): AgentApiError {
  const envelope = body as AgentErrorEnvelope | null;
  const err = envelope?.error;
  return new AgentApiError(
    response.status,
    err?.code ?? "AGENT_INTERNAL_ERROR",
    err?.message ?? (response.statusText || "request failed"),
    err?.details,
    err?.traceId ?? response.headers.get("x-trace-id"),
  );
}
