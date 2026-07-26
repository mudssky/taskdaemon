/**
 * MCP 目录 HTTP 路由。
 */

import type { Principal } from "@taskdaemon/agent-protocol";
import type { Context, Hono } from "hono";
import { errorBody, toAgentHttpError } from "../errors.js";
import type { McpCatalog } from "./catalog.js";

export type McpRouteEnv = {
  Variables: {
    principal: Principal;
    mcpCatalog: McpCatalog;
  };
};

/**
 * 挂载 MCP 查询路由到 v1 子应用。
 *
 * 参数:
 *   - v1: Hono v1 路由。
 *   - getCatalog: 取目录。
 */
export function mountMcpRoutes(
  v1: Hono<McpRouteEnv>,
  getCatalog: () => McpCatalog,
): void {
  v1.get("/mcp/servers", (c) => {
    try {
      const principal = c.get("principal");
      const catalog = getCatalog();
      const servers = catalog.serversForTenant(principal.tenantId).map((s) => ({
        id: s.id,
        enabled: s.enabled,
        transport: s.transport,
        available: s.available !== false,
        toolCount: (s.tools ?? []).length,
        reloadMode: catalog.reloadMode,
      }));
      return c.json({
        reloadMode: catalog.reloadMode,
        items: servers,
      });
    } catch (err) {
      return respondMcpError(c, err);
    }
  });

  v1.get("/mcp/tools", (c) => {
    try {
      const principal = c.get("principal");
      const catalog = getCatalog();
      const tools = catalog.listToolsForTenant(principal.tenantId);
      return c.json({
        reloadMode: catalog.reloadMode,
        items: tools,
      });
    } catch (err) {
      return respondMcpError(c, err);
    }
  });
}

/**
 * MCP 路由错误响应。
 *
 * 参数:
 *   - c: Hono context。
 *   - err: 错误。
 *
 * 返回值:
 *   - Response。
 */
function respondMcpError(c: Context, err: unknown): Response {
  const principal = c.get("principal") as Principal | undefined;
  const httpErr = toAgentHttpError(err, principal?.traceId);
  return c.json(
    errorBody(httpErr.code, httpErr.message, httpErr.traceId, httpErr.details),
    httpErr.status as 400,
  );
}
