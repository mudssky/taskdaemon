/**
 * TrustEventEmitter：默认结构化 stdout；可内存收集供测试。
 */

import type {
  AuditEvent,
  TrustEventEmitter,
  UsageEvent,
} from "@taskdaemon/agent-protocol";

/** 内存收集器（测试用）。 */
export class MemoryTrustEventEmitter implements TrustEventEmitter {
  readonly audits: AuditEvent[] = [];
  readonly usages: UsageEvent[] = [];

  /**
   * 参数:
   *   - event: AuditEvent。
   */
  async emitAudit(event: AuditEvent): Promise<void> {
    this.audits.push(event);
  }

  /**
   * 参数:
   *   - event: UsageEvent。
   */
  async emitUsage(event: UsageEvent): Promise<void> {
    this.usages.push(event);
  }
}

/** 日志外送：失败吞掉，不阻断主路径。 */
export class LoggingTrustEventEmitter implements TrustEventEmitter {
  /**
   * 参数:
   *   - event: AuditEvent。
   */
  async emitAudit(event: AuditEvent): Promise<void> {
    try {
      console.info(JSON.stringify(event));
    } catch {
      // 忽略序列化失败
    }
  }

  /**
   * 参数:
   *   - event: UsageEvent。
   */
  async emitUsage(event: UsageEvent): Promise<void> {
    try {
      console.info(JSON.stringify(event));
    } catch {
      // 忽略
    }
  }
}

/**
 * 安全地发出 audit（永不抛到调用方）。
 *
 * 参数:
 *   - emitter: TrustEventEmitter。
 *   - event: AuditEvent。
 */
export async function safeEmitAudit(
  emitter: TrustEventEmitter,
  event: AuditEvent,
): Promise<void> {
  try {
    await emitter.emitAudit(event);
  } catch (err) {
    console.error("[trust] emitAudit failed", err);
  }
}

/**
 * 安全地发出 usage。
 *
 * 参数:
 *   - emitter: TrustEventEmitter。
 *   - event: UsageEvent。
 */
export async function safeEmitUsage(
  emitter: TrustEventEmitter,
  event: UsageEvent,
): Promise<void> {
  try {
    await emitter.emitUsage(event);
  } catch (err) {
    console.error("[trust] emitUsage failed", err);
  }
}
