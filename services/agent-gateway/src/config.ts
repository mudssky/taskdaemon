/**
 * Gateway 配置：池化默认值来自 G0 latency 建议。
 */

import type { PrincipalResolverMode } from "@taskdaemon/agent-protocol";

/** 单 adapter 池参数。 */
export type PoolConfig = {
  /** 预热最小空闲数；high 建议 1–2。 */
  minIdle: number;
  /** 最大空闲保留。 */
  maxIdle: number;
  /** 空闲 TTL（毫秒）。 */
  idleTtlMs: number;
  /** 最大并发会话/进程。 */
  maxConcurrency: number;
};

/** 超并发行为：本骨架固定 reject 并返回 503。 */
export type OverLimitPolicy = "reject";

export type GatewayConfig = {
  host: string;
  port: number;
  /** 默认 runtimeId（mock-high）。 */
  defaultRuntimeId: string;
  principalResolverMode: PrincipalResolverMode;
  allowDevPrincipalInProduction: boolean;
  nodeEnv: string;
  /** 空闲 thread 超时回收（毫秒）。 */
  sessionIdleTtlMs: number;
  /** 空闲扫描间隔。 */
  idleScanIntervalMs: number;
  /** 超限策略（文档化：reject）。 */
  overLimitPolicy: OverLimitPolicy;
  /** 按 coldStartCost 的默认池模板。 */
  poolByColdStart: {
    high: PoolConfig;
    low: PoolConfig;
  };
  /** 按 runtimeId 覆盖 maxConcurrency 等。 */
  runtimePoolOverrides: Record<string, Partial<PoolConfig>>;
  /** tool allowlist；空数组 = 禁止一切具名 tool（保守）。 */
  toolAllowlist: string[];
  /** workspace 沙箱根；会话 cwd 必须落在其下。 */
  workspaceSandboxRoot: string;
  /** SSE 环缓条数。 */
  streamBufferSize: number;
  /** run 结束后环缓保留。 */
  streamRetentionMs: number;
};

/**
 * 从环境变量加载配置。
 *
 * 参数:
 *   - env: 环境变量映射，默认 process.env。
 *
 * 返回值:
 *   - GatewayConfig。
 */
export function loadConfig(
  env: Record<string, string | undefined> = process.env,
): GatewayConfig {
  const modeRaw = (env.PRINCIPAL_RESOLVER ?? "dev").toLowerCase();
  const principalResolverMode: PrincipalResolverMode =
    modeRaw === "gateway" || modeRaw === "gateway-headers"
      ? "gateway-headers"
      : "dev-single-principal";

  return {
    host: env.AGENT_GATEWAY_HOST ?? "127.0.0.1",
    port: Number(env.AGENT_GATEWAY_PORT ?? "8787"),
    defaultRuntimeId: env.AGENT_GATEWAY_DEFAULT_RUNTIME ?? "mock-high",
    principalResolverMode,
    allowDevPrincipalInProduction: env.ALLOW_DEV_PRINCIPAL === "1",
    nodeEnv: env.NODE_ENV ?? "development",
    sessionIdleTtlMs: Number(
      env.AGENT_SESSION_IDLE_TTL_MS ?? String(10 * 60_000),
    ),
    idleScanIntervalMs: Number(env.AGENT_IDLE_SCAN_MS ?? "30000"),
    overLimitPolicy: "reject",
    poolByColdStart: {
      // G0：CLI coldStart median ~3s → 预热 + 长空闲
      high: {
        minIdle: Number(env.POOL_HIGH_MIN_IDLE ?? "1"),
        maxIdle: Number(env.POOL_HIGH_MAX_IDLE ?? "2"),
        idleTtlMs: Number(env.POOL_HIGH_IDLE_TTL_MS ?? String(10 * 60_000)),
        maxConcurrency: Number(env.POOL_HIGH_MAX_CONCURRENCY ?? "8"),
      },
      // G0：SDK low → 按需起、短空闲
      low: {
        minIdle: Number(env.POOL_LOW_MIN_IDLE ?? "0"),
        maxIdle: Number(env.POOL_LOW_MAX_IDLE ?? "1"),
        idleTtlMs: Number(env.POOL_LOW_IDLE_TTL_MS ?? String(2 * 60_000)),
        maxConcurrency: Number(env.POOL_LOW_MAX_CONCURRENCY ?? "16"),
      },
    },
    runtimePoolOverrides: {
      // heavy memoryClass 模拟（对齐 OMP 建议更低并发）
      "mock-high-heavy": {
        maxConcurrency: Number(env.POOL_HEAVY_MAX_CONCURRENCY ?? "3"),
      },
    },
    toolAllowlist: (env.AGENT_TOOL_ALLOWLIST ?? "")
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean),
    workspaceSandboxRoot: env.AGENT_WORKSPACE_SANDBOX_ROOT ?? process.cwd(),
    streamBufferSize: Number(env.AGENT_STREAM_BUFFER_SIZE ?? "1000"),
    streamRetentionMs: Number(
      env.AGENT_STREAM_RETENTION_MS ?? String(10 * 60_000),
    ),
  };
}
