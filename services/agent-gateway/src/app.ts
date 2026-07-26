/**
 * 组装 Hono app：健康检查、信任中间件、协议路由。
 */

import type {
  PrincipalResolver,
  TrustEventEmitter,
} from "@taskdaemon/agent-protocol";
import { Hono } from "hono";
import type { GatewayConfig } from "./config.js";
import { McpCatalog } from "./mcp/catalog.js";
import { type AppEnv, mountRoutes } from "./routes/routes.js";
import type { Orchestrator } from "./runtime/orchestrator.js";
import { principalMiddleware } from "./trust/middleware.js";

export type CreateAppOptions = {
  config: GatewayConfig;
  orchestrator: Orchestrator;
  principalResolver: PrincipalResolver;
  emitter: TrustEventEmitter;
  /** 可选注入；缺省由 config.mcp 构造。 */
  mcpCatalog?: McpCatalog;
};

/**
 * 创建 Hono 应用。
 *
 * 参数:
 *   - options: CreateAppOptions。
 *
 * 返回值:
 *   - Hono app。
 */
export function createApp(options: CreateAppOptions): Hono<AppEnv> {
  const app = new Hono<AppEnv>();
  const { config, orchestrator, principalResolver } = options;
  const mcpCatalog = options.mcpCatalog ?? new McpCatalog(config.mcp);

  app.get("/health", (c) => {
    return c.json({
      ok: true,
      runtime: "node",
      principalResolver: config.principalResolverMode,
      overLimitPolicy: config.overLimitPolicy,
      adapters: orchestrator.poolStats(),
      runtimes: orchestrator.listRuntimes().map((r) => r.runtimeId),
      mcpReloadMode: mcpCatalog.reloadMode,
      mcpServerCount: mcpCatalog.listServers().length,
    });
  });

  // API 需要 principal
  app.use("/v1/*", principalMiddleware(principalResolver));
  app.use("/v1/*", async (c, next) => {
    c.set("orchestrator", orchestrator);
    c.set("mcpCatalog", mcpCatalog);
    await next();
  });

  mountRoutes(
    app,
    () => orchestrator,
    () => mcpCatalog,
  );

  app.onError((err, c) => {
    console.error("[agent-gateway] unhandled", err);
    return c.json(
      {
        error: {
          code: "AGENT_INTERNAL_ERROR",
          message: err.message || "internal error",
        },
      },
      500,
    );
  });

  return app;
}
