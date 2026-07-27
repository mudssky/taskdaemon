/**
 * agent-gateway 入口：加载配置、注册 mock adapter、优雅关闭。
 */

import { serve } from "@hono/node-server";
import { createApp } from "./app.js";
import { loadConfig } from "./config.js";
import { createDefaultMockRuntimes } from "./runtime/mock-adapter.js";
import { Orchestrator } from "./runtime/orchestrator.js";
import { PoolManager } from "./runtime/pool-manager.js";
import { RuntimeRegistry } from "./runtime/registry.js";
import { MemoryThreadRunStore } from "./store/memory-store.js";
import { StreamHub } from "./stream/sse.js";
import { LoggingTrustEventEmitter } from "./trust/emitter.js";
import { createPrincipalResolver } from "./trust/principal.js";

/**
 * 启动服务。
 *
 * 参数: 无。
 * 返回值: Promise<void>。
 */
export async function main(): Promise<void> {
  const config = loadConfig();

  if (
    config.nodeEnv === "production" &&
    config.principalResolverMode === "dev-single-principal"
  ) {
    console.error(
      "SECURITY WARNING: PrincipalResolver is dev-single-principal; not safe for public exposure",
    );
    if (!config.allowDevPrincipalInProduction) {
      console.error(
        "Refusing to start: set PRINCIPAL_RESOLVER=gateway or ALLOW_DEV_PRINCIPAL=1",
      );
      process.exit(1);
    }
  } else if (config.principalResolverMode === "dev-single-principal") {
    console.warn(
      "SECURITY WARNING: PrincipalResolver is dev-single-principal; not safe for public exposure",
    );
  }

  const registry = new RuntimeRegistry();
  for (const runtime of createDefaultMockRuntimes()) {
    registry.register(runtime);
  }

  const pools = new PoolManager();
  // 预注册池配置（high 暴露 minIdle 预热意图）
  for (const desc of registry.list()) {
    const cost = desc.capabilities.coldStartCost;
    const base = { ...config.poolByColdStart[cost] };
    if (desc.capabilities.memoryClass === "heavy") {
      base.maxConcurrency = Math.min(base.maxConcurrency, 3);
    }
    const override = config.runtimePoolOverrides[desc.runtimeId];
    pools.ensure(desc.runtimeId, cost, { ...base, ...override });
  }

  const store = new MemoryThreadRunStore();
  const streams = new StreamHub(
    config.streamBufferSize,
    config.streamRetentionMs,
  );
  const emitter = new LoggingTrustEventEmitter();
  const orchestrator = new Orchestrator({
    config,
    registry,
    pools,
    store,
    streams,
    emitter,
  });
  orchestrator.start();

  const principalResolver = createPrincipalResolver(
    config.principalResolverMode,
  );
  const app = createApp({
    config,
    orchestrator,
    principalResolver,
    emitter,
  });

  const server = serve(
    {
      fetch: app.fetch,
      hostname: config.host,
      port: config.port,
    },
    (info) => {
      console.info(
        `[agent-gateway] listening on http://${info.address}:${info.port}`,
      );
    },
  );

  const shutdown = async (signal: string) => {
    console.info(`[agent-gateway] ${signal} received, shutting down…`);
    await orchestrator.stop();
    server.close(() => {
      console.info("[agent-gateway] closed");
      process.exit(0);
    });
    // 兜底
    setTimeout(() => process.exit(1), 10_000).unref();
  };

  process.on("SIGINT", () => void shutdown("SIGINT"));
  process.on("SIGTERM", () => void shutdown("SIGTERM"));
}

// 仅直接执行时启动
const isDirect =
  typeof process.argv[1] === "string" &&
  (process.argv[1].endsWith("index.ts") ||
    process.argv[1].endsWith("index.js"));

if (isDirect) {
  void main();
}

export { createApp } from "./app.js";
export { loadConfig } from "./config.js";
export {
  createDefaultMockRuntimes,
  MockAgentRuntime,
} from "./runtime/mock-adapter.js";
export { Orchestrator } from "./runtime/orchestrator.js";
