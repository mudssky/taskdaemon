/**
 * 测试用应用装配。
 */

import { createApp } from "../src/app.js";
import type { GatewayConfig } from "../src/config.js";
import { loadConfig } from "../src/config.js";
import { McpCatalog } from "../src/mcp/catalog.js";
import {
  createDefaultMockRuntimes,
  MockAgentRuntime,
  mockCapabilities,
} from "../src/runtime/mock-adapter.js";
import { Orchestrator } from "../src/runtime/orchestrator.js";
import { PoolManager } from "../src/runtime/pool-manager.js";
import { RuntimeRegistry } from "../src/runtime/registry.js";
import { MemoryThreadRunStore } from "../src/store/memory-store.js";
import { StreamHub } from "../src/stream/sse.js";
import { MemoryTrustEventEmitter } from "../src/trust/emitter.js";
import { createPrincipalResolver } from "../src/trust/principal.js";

export type TestContext = {
  app: ReturnType<typeof createApp>;
  orchestrator: Orchestrator;
  emitter: MemoryTrustEventEmitter;
  registry: RuntimeRegistry;
  config: GatewayConfig;
  runtimes: MockAgentRuntime[];
  mcpCatalog: McpCatalog;
};

/**
 * 创建隔离测试上下文。
 *
 * 参数:
 *   - overrides: 配置覆盖。
 *
 * 返回值:
 *   - TestContext。
 */
export function createTestContext(
  overrides: Partial<GatewayConfig> = {},
): TestContext {
  const config: GatewayConfig = {
    ...loadConfig({
      NODE_ENV: "test",
      PRINCIPAL_RESOLVER: "dev",
      AGENT_WORKSPACE_SANDBOX_ROOT: process.cwd(),
      AGENT_TOOL_ALLOWLIST: "echo,read_file",
      POOL_HIGH_MAX_CONCURRENCY: "2",
      POOL_LOW_MAX_CONCURRENCY: "2",
      POOL_HIGH_MIN_IDLE: "1",
      POOL_LOW_MIN_IDLE: "0",
      AGENT_SESSION_IDLE_TTL_MS: "3600000",
      AGENT_IDLE_SCAN_MS: "60000",
    }),
    ...overrides,
  };

  const registry = new RuntimeRegistry();
  const runtimes = createDefaultMockRuntimes();
  for (const runtime of runtimes) {
    registry.register(runtime);
  }

  const pools = new PoolManager();
  for (const desc of registry.list()) {
    const cost = desc.capabilities.coldStartCost;
    const base = { ...config.poolByColdStart[cost] };
    if (desc.capabilities.memoryClass === "heavy") {
      base.maxConcurrency = Math.min(base.maxConcurrency, 3);
    }
    pools.ensure(desc.runtimeId, cost, {
      ...base,
      ...config.runtimePoolOverrides[desc.runtimeId],
    });
  }

  const store = new MemoryThreadRunStore();
  const streams = new StreamHub(100, 60_000);
  const emitter = new MemoryTrustEventEmitter();
  const mcpCatalog = new McpCatalog(config.mcp);
  const orchestrator = new Orchestrator({
    config,
    registry,
    pools,
    store,
    streams,
    emitter,
    mcpCatalog,
  });
  // 测试不启 idle timer，避免干扰

  const app = createApp({
    config,
    orchestrator,
    principalResolver: createPrincipalResolver(config.principalResolverMode),
    emitter,
    mcpCatalog,
  });

  return { app, orchestrator, emitter, registry, config, runtimes, mcpCatalog };
}

/**
 * 发送 JSON 请求。
 *
 * 参数:
 *   - app: Hono app。
 *   - method: HTTP 方法。
 *   - path: 路径。
 *   - body: 可选 JSON body。
 *   - headers: 可选头。
 *
 * 返回值:
 *   - status / json / headers。
 */
export async function jsonRequest(
  app: TestContext["app"],
  method: string,
  path: string,
  body?: unknown,
  headers?: Record<string, string>,
): Promise<{ status: number; json: unknown; headers: Headers }> {
  const res = await app.request(path, {
    method,
    headers: {
      "content-type": "application/json",
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let json: unknown = null;
  if (text) {
    try {
      json = JSON.parse(text);
    } catch {
      json = text;
    }
  }
  return { status: res.status, json, headers: res.headers };
}

export { MockAgentRuntime, mockCapabilities };
