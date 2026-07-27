/**
 * 按 coldStartCost 差异化的会话槽池。
 *
 * high：维护 minIdle 预热语义（计数）与更长 idleTtl。
 * low：按需起、快速回收。
 * 超 maxConcurrency：reject（由 orchestrator 抛 AGENT_RUNTIME_UNAVAILABLE）。
 */

import type { ColdStartCost } from "@taskdaemon/agent-protocol";
import type { PoolConfig } from "../config.js";
import { AgentHttpError } from "../errors.js";

export type PoolSlot = {
  sessionId: string;
  runtimeId: string;
  /** 绑定到 thread 后为 busy；预热未绑定为 idle。 */
  state: "idle" | "busy";
  createdAt: number;
  lastUsedAt: number;
};

export type AdapterPoolStats = {
  runtimeId: string;
  coldStartCost: ColdStartCost;
  activeSessions: number;
  processCount: number;
  warmPoolIdle: number;
  warmPoolBusy: number;
  maxConcurrency: number;
  minIdle: number;
};

/** 单 runtime 的池状态。 */
export class AdapterPool {
  readonly runtimeId: string;
  readonly coldStartCost: ColdStartCost;
  readonly config: PoolConfig;
  private readonly slots = new Map<string, PoolSlot>();

  /**
   * 参数:
   *   - runtimeId: adapter id。
   *   - coldStartCost: 冷启动成本。
   *   - config: 池配置。
   */
  constructor(
    runtimeId: string,
    coldStartCost: ColdStartCost,
    config: PoolConfig,
  ) {
    this.runtimeId = runtimeId;
    this.coldStartCost = coldStartCost;
    this.config = config;
  }

  /**
   * 尝试占用一个并发名额并登记 busy 槽。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *
   * 异常:
   *   - 超限 → AGENT_RUNTIME_UNAVAILABLE。
   */
  acquire(sessionId: string): void {
    if (this.busyCount() >= this.config.maxConcurrency) {
      throw new AgentHttpError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `concurrency limit reached for runtime ${this.runtimeId}`,
        {
          details: {
            runtimeId: this.runtimeId,
            maxConcurrency: this.config.maxConcurrency,
            policy: "reject",
          },
        },
      );
    }
    const now = Date.now();
    this.slots.set(sessionId, {
      sessionId,
      runtimeId: this.runtimeId,
      state: "busy",
      createdAt: now,
      lastUsedAt: now,
    });
  }

  /**
   * 标记最近使用。
   */
  touch(sessionId: string): void {
    const slot = this.slots.get(sessionId);
    if (slot) {
      slot.lastUsedAt = Date.now();
    }
  }

  /**
   * 释放槽（会话 dispose）。
   */
  release(sessionId: string): void {
    this.slots.delete(sessionId);
  }

  /**
   * 当前 busy 数。
   */
  busyCount(): number {
    let n = 0;
    for (const slot of this.slots.values()) {
      if (slot.state === "busy") n += 1;
    }
    return n;
  }

  /**
   * 当前登记槽数（≈ 进程数代理）。
   */
  processCount(): number {
    return this.slots.size;
  }

  /**
   * 预热目标：high 需要 minIdle；此处返回「是否仍需预热名额」。
   * 真实预热进程由 G3 实现；骨架用配置暴露意图并用 stats 证明分支。
   */
  warmPoolTarget(): number {
    return this.coldStartCost === "high" ? this.config.minIdle : 0;
  }

  /**
   * 统计快照。
   */
  stats(): AdapterPoolStats {
    let idle = 0;
    let busy = 0;
    for (const slot of this.slots.values()) {
      if (slot.state === "idle") idle += 1;
      else busy += 1;
    }
    // high 的 warm 目标计入观测（即便尚未 spawn 真实预热进程）
    const warmTarget = this.warmPoolTarget();
    return {
      runtimeId: this.runtimeId,
      coldStartCost: this.coldStartCost,
      activeSessions: busy,
      processCount: this.slots.size,
      warmPoolIdle: idle,
      warmPoolBusy: busy,
      maxConcurrency: this.config.maxConcurrency,
      minIdle: warmTarget,
    };
  }

  /**
   * 池策略是否为 high 预热型。
   */
  usesWarmPool(): boolean {
    return this.coldStartCost === "high" && this.config.minIdle > 0;
  }
}

/** 多 adapter 池管理。 */
export class PoolManager {
  private readonly pools = new Map<string, AdapterPool>();

  /**
   * 确保 runtime 有池。
   *
   * 参数:
   *   - runtimeId: id。
   *   - coldStartCost: 成本。
   *   - config: 池配置。
   *
   * 返回值:
   *   - AdapterPool。
   */
  ensure(
    runtimeId: string,
    coldStartCost: ColdStartCost,
    config: PoolConfig,
  ): AdapterPool {
    let pool = this.pools.get(runtimeId);
    if (!pool) {
      pool = new AdapterPool(runtimeId, coldStartCost, config);
      this.pools.set(runtimeId, pool);
    }
    return pool;
  }

  /**
   * 获取池。
   */
  get(runtimeId: string): AdapterPool | undefined {
    return this.pools.get(runtimeId);
  }

  /**
   * 全部统计。
   */
  allStats(): AdapterPoolStats[] {
    return [...this.pools.values()].map((p) => p.stats());
  }
}
