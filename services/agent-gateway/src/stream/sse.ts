/**
 * Run 事件环缓与 SSE 订阅。
 */

import type { AguiEvent } from "@taskdaemon/agent-protocol";
import { formatSseFrame } from "./agui-mapper.js";

export type StreamListener = {
  onEvent: (frame: string) => void;
  onClose: () => void;
};

/** 单 run 的事件缓冲。 */
export class RunEventBuffer {
  readonly runId: string;
  private readonly maxSize: number;
  private readonly events: AguiEvent[] = [];
  private readonly listeners = new Set<StreamListener>();
  private closed = false;

  /**
   * 参数:
   *   - runId: run id。
   *   - maxSize: 环缓上限。
   */
  constructor(runId: string, maxSize: number) {
    this.runId = runId;
    this.maxSize = maxSize;
  }

  /**
   * 追加事件并广播。
   *
   * 参数:
   *   - event: AguiEvent。
   */
  push(event: AguiEvent): void {
    this.events.push(event);
    if (this.events.length > this.maxSize) {
      this.events.splice(0, this.events.length - this.maxSize);
    }
    const frame = formatSseFrame(event);
    for (const listener of this.listeners) {
      listener.onEvent(frame);
    }
  }

  /**
   * 从游标之后重放。
   *
   * 参数:
   *   - cursor: 上次事件 id；空则从头。
   *
   * 返回值:
   *   - 待发送帧；cursor 无效时 null。
   */
  replayFrom(cursor: string | undefined): string[] | null {
    if (!cursor) {
      return this.events.map(formatSseFrame);
    }
    const idx = this.events.findIndex((e) => e.id === cursor);
    if (idx === -1) {
      return null;
    }
    return this.events.slice(idx + 1).map(formatSseFrame);
  }

  /**
   * 订阅后续事件。
   *
   * 参数:
   *   - listener: StreamListener。
   *
   * 返回值:
   *   - 取消订阅函数。
   */
  subscribe(listener: StreamListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
      listener.onClose();
    };
  }

  /**
   * 监听者数量（测泄漏）。
   */
  listenerCount(): number {
    return this.listeners.size;
  }

  /**
   * 关闭并通知订阅者。
   */
  close(): void {
    if (this.closed) return;
    this.closed = true;
    for (const listener of this.listeners) {
      listener.onClose();
    }
    this.listeners.clear();
  }
}

/** 多 run 缓冲管理。 */
export class StreamHub {
  private readonly buffers = new Map<string, RunEventBuffer>();
  private readonly maxSize: number;
  private readonly retentionMs: number;
  private readonly retentionTimers = new Map<
    string,
    ReturnType<typeof setTimeout>
  >();

  /**
   * 参数:
   *   - maxSize: 每 run 环缓。
   *   - retentionMs: 结束后保留。
   */
  constructor(maxSize: number, retentionMs: number) {
    this.maxSize = maxSize;
    this.retentionMs = retentionMs;
  }

  /**
   * 获取或创建缓冲。
   */
  getOrCreate(runId: string): RunEventBuffer {
    let buf = this.buffers.get(runId);
    if (!buf) {
      buf = new RunEventBuffer(runId, this.maxSize);
      this.buffers.set(runId, buf);
    }
    return buf;
  }

  get(runId: string): RunEventBuffer | undefined {
    return this.buffers.get(runId);
  }

  /**
   * run 结束后安排淘汰。
   */
  scheduleRetention(runId: string): void {
    const existing = this.retentionTimers.get(runId);
    if (existing) clearTimeout(existing);
    const timer = setTimeout(() => {
      const buf = this.buffers.get(runId);
      buf?.close();
      this.buffers.delete(runId);
      this.retentionTimers.delete(runId);
    }, this.retentionMs);
    // 不阻塞进程退出
    if (typeof timer === "object" && "unref" in timer) {
      timer.unref();
    }
    this.retentionTimers.set(runId, timer);
  }

  /**
   * 关闭全部。
   */
  closeAll(): void {
    for (const timer of this.retentionTimers.values()) {
      clearTimeout(timer);
    }
    this.retentionTimers.clear();
    for (const buf of this.buffers.values()) {
      buf.close();
    }
    this.buffers.clear();
  }
}
