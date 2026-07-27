/**
 * Hono 中间件：解析 Principal 并挂到 context。
 */

import type { Principal, PrincipalResolver } from "@taskdaemon/agent-protocol";
import type { Context, MiddlewareHandler, Next } from "hono";
import { errorBody, toAgentHttpError } from "../errors.js";
import { normalizeHeaders } from "./principal.js";

export type PrincipalVariables = {
  principal: Principal;
};

/**
 * 创建身份解析中间件。
 *
 * 参数:
 *   - resolver: PrincipalResolver。
 *
 * 返回值:
 *   - Hono MiddlewareHandler。
 */
export function principalMiddleware(
  resolver: PrincipalResolver,
): MiddlewareHandler<{ Variables: PrincipalVariables }> {
  return async (
    c: Context<{ Variables: PrincipalVariables }>,
    next: Next,
  ): Promise<Response | undefined> => {
    try {
      const headers = normalizeHeaders(c.req.raw.headers);
      const principal = await resolver.resolve(headers);
      c.set("principal", principal);
      c.header("x-trace-id", principal.traceId);
      await next();
    } catch (err) {
      const agentErr = toAgentHttpError(err);
      return c.json(
        errorBody(
          agentErr.code,
          agentErr.message,
          agentErr.traceId,
          agentErr.details,
        ),
        agentErr.status as 401,
      );
    }
  };
}
