/**
 * Thread / Run 薄索引（内存）。transcript 权威在 runtime。
 */

import type {
  Run,
  RunStatus,
  Thread,
  ThreadStatus,
} from "@taskdaemon/agent-protocol";

export type ThreadRecord = Thread & {
  sessionId: string;
  tools?: "none" | string[];
  lastActivityAt: number;
};

export type RunRecord = Run & {
  onDisconnect: "cancel" | "continue";
  seq: number;
};

/** 存储接口：便于后续换 SQLite。 */
export interface ThreadRunStore {
  putThread(thread: ThreadRecord): void;
  getThread(tenantId: string, threadId: string): ThreadRecord | undefined;
  deleteThread(tenantId: string, threadId: string): void;
  listThreads(
    tenantId: string,
    filter?: {
      status?: ThreadStatus;
      runtimeId?: string;
      profile?: string;
    },
  ): ThreadRecord[];
  putRun(run: RunRecord): void;
  getRun(
    tenantId: string,
    threadId: string,
    runId: string,
  ): RunRecord | undefined;
  listRuns(tenantId: string, threadId: string): RunRecord[];
  updateThread(
    tenantId: string,
    threadId: string,
    patch: Partial<ThreadRecord>,
  ): ThreadRecord | undefined;
  updateRun(
    tenantId: string,
    threadId: string,
    runId: string,
    patch: Partial<RunRecord>,
  ): RunRecord | undefined;
  findActiveRun(tenantId: string, threadId: string): RunRecord | undefined;
  /** 全部 thread（空闲回收用）。 */
  allThreads(): ThreadRecord[];
}

/** 进程内实现。 */
export class MemoryThreadRunStore implements ThreadRunStore {
  private readonly threads = new Map<string, ThreadRecord>();
  private readonly runs = new Map<string, RunRecord>();

  private threadKey(tenantId: string, threadId: string): string {
    return `${tenantId}::${threadId}`;
  }

  private runKey(tenantId: string, threadId: string, runId: string): string {
    return `${tenantId}::${threadId}::${runId}`;
  }

  putThread(thread: ThreadRecord): void {
    this.threads.set(this.threadKey(thread.tenantId, thread.threadId), thread);
  }

  getThread(tenantId: string, threadId: string): ThreadRecord | undefined {
    return this.threads.get(this.threadKey(tenantId, threadId));
  }

  deleteThread(tenantId: string, threadId: string): void {
    this.threads.delete(this.threadKey(tenantId, threadId));
    for (const key of [...this.runs.keys()]) {
      if (key.startsWith(`${tenantId}::${threadId}::`)) {
        this.runs.delete(key);
      }
    }
  }

  listThreads(
    tenantId: string,
    filter?: {
      status?: ThreadStatus;
      runtimeId?: string;
      profile?: string;
    },
  ): ThreadRecord[] {
    const items: ThreadRecord[] = [];
    for (const thread of this.threads.values()) {
      if (thread.tenantId !== tenantId) continue;
      if (filter?.status && thread.status !== filter.status) continue;
      if (filter?.runtimeId && thread.runtimeId !== filter.runtimeId) continue;
      if (filter?.profile && thread.profile !== filter.profile) continue;
      items.push(thread);
    }
    return items;
  }

  putRun(run: RunRecord): void {
    // tenant 存在 thread 上；调用方保证一致性，key 用 thread 查找
    // 这里要求 run 已关联；tenant 通过 list 时过滤
    const tenant = this.findTenantForThread(run.threadId);
    if (!tenant) {
      throw new Error(`thread missing for run: ${run.threadId}`);
    }
    this.runs.set(this.runKey(tenant, run.threadId, run.runId), run);
  }

  /** 带 tenant 写入 run。 */
  putRunForTenant(tenantId: string, run: RunRecord): void {
    this.runs.set(this.runKey(tenantId, run.threadId, run.runId), run);
  }

  getRun(
    tenantId: string,
    threadId: string,
    runId: string,
  ): RunRecord | undefined {
    return this.runs.get(this.runKey(tenantId, threadId, runId));
  }

  listRuns(tenantId: string, threadId: string): RunRecord[] {
    const prefix = `${tenantId}::${threadId}::`;
    const items: RunRecord[] = [];
    for (const [key, run] of this.runs) {
      if (key.startsWith(prefix)) items.push(run);
    }
    return items;
  }

  updateThread(
    tenantId: string,
    threadId: string,
    patch: Partial<ThreadRecord>,
  ): ThreadRecord | undefined {
    const key = this.threadKey(tenantId, threadId);
    const current = this.threads.get(key);
    if (!current) return undefined;
    const next = { ...current, ...patch };
    this.threads.set(key, next);
    return next;
  }

  updateRun(
    tenantId: string,
    threadId: string,
    runId: string,
    patch: Partial<RunRecord>,
  ): RunRecord | undefined {
    const key = this.runKey(tenantId, threadId, runId);
    const current = this.runs.get(key);
    if (!current) return undefined;
    const next = { ...current, ...patch };
    this.runs.set(key, next);
    return next;
  }

  findActiveRun(tenantId: string, threadId: string): RunRecord | undefined {
    const active: RunStatus[] = ["pending", "running"];
    return this.listRuns(tenantId, threadId).find((r) =>
      active.includes(r.status),
    );
  }

  private findTenantForThread(threadId: string): string | undefined {
    for (const thread of this.threads.values()) {
      if (thread.threadId === threadId) return thread.tenantId;
    }
    return undefined;
  }

  allThreads(): ThreadRecord[] {
    return [...this.threads.values()];
  }
}
