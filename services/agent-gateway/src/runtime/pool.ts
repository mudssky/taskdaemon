/**
 * 冷启动池化策略（G0 → G2）。
 * 本模块只产出策略参数，不持有真实进程池（池由 G2 编排）。
 */

import type {
  ColdStartCost,
  MemoryClass,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";

/** 单 runtime 的池参数。 */
export type RuntimePoolPolicy = {
  runtimeId: string;
  coldStartCost: ColdStartCost;
  memoryClass: MemoryClass;
  /** 建议最小空闲热进程数。 */
  minIdle: number;
  /** 最大并发会话。 */
  maxConcurrency: number;
  /** 空闲保留毫秒。 */
  idleTtlMs: number;
  /** 是否建议预热。 */
  warmup: boolean;
};

export type PoolPolicyOptions = {
  /** 机器可用内存粗粒度提示（GiB），影响 maxConcurrency。 */
  hostMemoryGiB?: number;
};

/**
 * 根据 capabilities 推导池策略（G0 §7）。
 *
 * 参数:
 *   - runtimeId: adapter id。
 *   - caps: 能力声明。
 *   - options: 主机资源提示。
 *
 * 返回值:
 *   - RuntimePoolPolicy。
 */
export function derivePoolPolicy(
  runtimeId: string,
  caps: RuntimeCapabilities,
  options: PoolPolicyOptions = {},
): RuntimePoolPolicy {
  const hostMemoryGiB = options.hostMemoryGiB ?? 16;
  const coldStartCost = caps.coldStartCost;
  const memoryClass = caps.memoryClass;

  // CLI high：预热 1–2；SDK low：可按需
  const warmup = coldStartCost === "high";
  const minIdle = coldStartCost === "high" ? 1 : 0;
  // 空闲保留 5–15min；high 取更长
  const idleTtlMs = coldStartCost === "high" ? 10 * 60_000 : 2 * 60_000;

  // 内存：OMP heavy ~490MB，Pi light ~150MB（G0）
  const perSessionGiB = memoryClass === "heavy" ? 0.55 : 0.2;
  // 保留 30% 给系统与其它服务
  const budgetGiB = hostMemoryGiB * 0.7;
  let maxConcurrency = Math.max(1, Math.floor(budgetGiB / perSessionGiB));
  // G0 经验上限
  if (memoryClass === "heavy") {
    maxConcurrency = Math.min(maxConcurrency, 3);
  } else {
    maxConcurrency = Math.min(maxConcurrency, 8);
  }

  return {
    runtimeId,
    coldStartCost,
    memoryClass,
    minIdle,
    maxConcurrency,
    idleTtlMs,
    warmup,
  };
}

/**
 * 批量推导已注册 runtime 的池策略。
 *
 * 参数:
 *   - catalog: id + capabilities 列表。
 *   - options: 主机资源。
 *
 * 返回值:
 *   - RuntimePoolPolicy[]。
 */
export function derivePoolPolicies(
  catalog: Array<{ id: string; capabilities: RuntimeCapabilities }>,
  options: PoolPolicyOptions = {},
): RuntimePoolPolicy[] {
  return catalog.map((entry) =>
    derivePoolPolicy(entry.id, entry.capabilities, options),
  );
}
