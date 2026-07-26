/**
 * 会话编排：thread↔runtime session 路由、池化、并发、空闲回收、审计钩子。
 */

import { randomUUID } from "node:crypto";
import {
  type AgentProfile,
  type AuditEvent,
  type CreateRunRequest,
  type CreateThreadRequest,
  hasOptionalRuntimeMethod,
  normalizePageQuery,
  type Principal,
  type PromptInput,
  type Run,
  supportsProfile,
  type Thread,
  type ThreadCapabilitiesResponse,
  type TrustEventEmitter,
} from "@taskdaemon/agent-protocol";
import type { GatewayConfig, PoolConfig } from "../config.js";
import { AgentHttpError } from "../errors.js";
import { assertToolAllowed, resolveWorkspaceRoot } from "../policy/policy.js";
import type {
  MemoryThreadRunStore,
  RunRecord,
  ThreadRecord,
} from "../store/memory-store.js";
import { mapRuntimeEvent } from "../stream/agui-mapper.js";
import type { StreamHub } from "../stream/sse.js";
import { safeEmitAudit, safeEmitUsage } from "../trust/emitter.js";
import type { AdapterPoolStats, PoolManager } from "./pool.js";
import type { RuntimeRegistry } from "./registry.js";

export type OrchestratorDeps = {
  config: GatewayConfig;
  registry: RuntimeRegistry;
  pools: PoolManager;
  store: MemoryThreadRunStore;
  streams: StreamHub;
  emitter: TrustEventEmitter;
};

/** 核心编排服务。 */
export class Orchestrator {
  private readonly config: GatewayConfig;
  private readonly registry: RuntimeRegistry;
  private readonly pools: PoolManager;
  private readonly store: MemoryThreadRunStore;
  private readonly streams: StreamHub;
  private readonly emitter: TrustEventEmitter;
  private idleTimer: ReturnType<typeof setInterval> | undefined;
  private closed = false;

  /**
   * 参数:
   *   - deps: OrchestratorDeps。
   */
  constructor(deps: OrchestratorDeps) {
    this.config = deps.config;
    this.registry = deps.registry;
    this.pools = deps.pools;
    this.store = deps.store;
    this.streams = deps.streams;
    this.emitter = deps.emitter;
  }

  /** 启动空闲扫描。 */
  start(): void {
    this.idleTimer = setInterval(() => {
      void this.reapIdleSessions();
    }, this.config.idleScanIntervalMs);
    if (typeof this.idleTimer === "object" && "unref" in this.idleTimer) {
      this.idleTimer.unref();
    }
  }

  /** 优雅关闭：dispose 全部会话。 */
  async stop(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    clearInterval(this.idleTimer);
    this.idleTimer = undefined;
    for (const thread of this.store.allThreads()) {
      try {
        await this.deleteThreadInternal(thread);
      } catch {
        // 关闭路径尽力而为
      }
    }
    this.streams.closeAll();
  }

  /**
   * 创建 thread + runtime session。
   */
  async createThread(
    principal: Principal,
    body: CreateThreadRequest,
  ): Promise<Thread> {
    const profile: AgentProfile = body.profile ?? "coding";
    const runtimeId = body.runtimeId ?? this.config.defaultRuntimeId;
    const runtime = this.registry.get(runtimeId);
    const caps = runtime.capabilities();

    if (!supportsProfile(caps, profile)) {
      throw new AgentHttpError(
        "AGENT_CAPABILITY_UNSUPPORTED",
        `profile ${profile} not supported by ${runtimeId}`,
      );
    }

    const workspaceBinding = body.workspaceBinding ?? profile === "coding";

    let workspaceRoot: string | null = null;
    try {
      workspaceRoot = resolveWorkspaceRoot(
        body.workspaceRoot,
        this.config.workspaceSandboxRoot,
        workspaceBinding,
      );
    } catch (err) {
      await this.audit(principal, {
        action: "thread.create",
        outcome: "denied",
        message: err instanceof Error ? err.message : "sandbox deny",
        workspaceRoot: body.workspaceRoot,
      });
      throw err;
    }

    // tools 策略：会话级 + gateway allowlist 在 tool 调用时再检
    if (body.tools && body.tools !== "none" && Array.isArray(body.tools)) {
      for (const tool of body.tools) {
        try {
          assertToolAllowed(tool, this.config.toolAllowlist, body.tools);
        } catch (err) {
          await this.audit(principal, {
            action: "thread.create",
            outcome: "denied",
            resource: tool,
            message: err instanceof Error ? err.message : "tool deny",
          });
          throw err;
        }
      }
    }

    const pool = this.ensurePool(
      runtimeId,
      caps.coldStartCost,
      caps.memoryClass,
    );
    const threadId = body.threadId ?? randomUUID();
    if (this.store.getThread(principal.tenantId, threadId)) {
      throw new AgentHttpError(
        "AGENT_VALIDATION_FAILED",
        `thread already exists: ${threadId}`,
      );
    }

    pool.acquire(threadId); // 先占名额；sessionId 稍后替换 key
    let sessionId: string | undefined;
    try {
      const handle = await runtime.createSession({
        cwd: workspaceRoot ?? undefined,
        profile,
        workspaceBinding,
        tools: body.tools,
        model: body.model,
        thinkingLevel: body.thinkingLevel,
        systemPrompt: body.systemPrompt,
        appendSystemPrompt: body.appendSystemPrompt,
        metadata: body.metadata,
        sessionPersistence: "runtime-file",
      });
      sessionId = handle.sessionId;
      // 用真实 sessionId 重挂池槽
      pool.release(threadId);
      pool.acquire(sessionId);

      const now = new Date().toISOString();
      const record: ThreadRecord = {
        threadId,
        status: "idle",
        runtimeId,
        profile,
        workspaceBound: workspaceBinding,
        workspaceRoot,
        model: body.model,
        metadata: body.metadata,
        createdAt: now,
        updatedAt: now,
        tenantId: principal.tenantId,
        subject: principal.subject,
        sessionId,
        tools: body.tools,
        lastActivityAt: Date.now(),
      };
      this.store.putThread(record);
      await this.audit(principal, {
        action: "thread.create",
        outcome: "success",
        threadId,
        workspaceRoot,
      });
      return toThread(record);
    } catch (err) {
      pool.release(threadId);
      if (sessionId) pool.release(sessionId);
      if (sessionId) {
        try {
          await runtime.disposeSession(sessionId);
        } catch {
          // ignore
        }
      }
      await this.audit(principal, {
        action: "thread.create",
        outcome: "error",
        message: err instanceof Error ? err.message : "create failed",
      });
      if (err instanceof AgentHttpError) throw err;
      throw new AgentHttpError(
        "AGENT_RUNTIME_UNAVAILABLE",
        err instanceof Error ? err.message : "createSession failed",
      );
    }
  }

  /**
   * 获取 thread（跨租户 404）。
   */
  getThread(principal: Principal, threadId: string): Thread {
    const record = this.requireThread(principal, threadId);
    return toThread(record);
  }

  listThreads(
    principal: Principal,
    query: {
      offset?: number;
      limit?: number;
      status?: Thread["status"];
      runtimeId?: string;
      profile?: AgentProfile;
      sortField?: "created_at" | "updated_at";
      sortDir?: "asc" | "desc";
    },
  ): {
    items: Thread[];
    offset: number;
    limit: number;
    hasMore: boolean;
    total: number;
  } {
    const { offset, limit } = normalizePageQuery(query);
    let items = this.store.listThreads(principal.tenantId, {
      status: query.status,
      runtimeId: query.runtimeId,
      profile: query.profile,
    });
    const field = query.sortField ?? "updated_at";
    const dir = query.sortDir ?? "desc";
    items = items.sort((a, b) => {
      const av = field === "created_at" ? a.createdAt : a.updatedAt;
      const bv = field === "created_at" ? b.createdAt : b.updatedAt;
      return dir === "asc" ? av.localeCompare(bv) : bv.localeCompare(av);
    });
    const total = items.length;
    const page = items.slice(offset, offset + limit).map(toThread);
    return {
      items: page,
      offset,
      limit,
      hasMore: offset + limit < total,
      total,
    };
  }

  async deleteThread(principal: Principal, threadId: string): Promise<void> {
    const record = this.requireThread(principal, threadId);
    await this.deleteThreadInternal(record, principal);
  }

  async getMessages(
    principal: Principal,
    threadId: string,
    page?: { offset?: number; limit?: number },
  ) {
    const record = this.requireThread(principal, threadId);
    const runtime = this.registry.get(record.runtimeId);
    const { offset, limit } = normalizePageQuery(page);
    try {
      const result = await runtime.getMessages(record.sessionId, {
        offset,
        limit,
      });
      this.touch(record);
      return {
        items: result.messages,
        offset: result.offset,
        limit: result.limit,
        hasMore: result.hasMore,
      };
    } catch (err) {
      await this.handleRuntimeFailure(record, err);
      throw new AgentHttpError(
        "AGENT_RUNTIME_UNAVAILABLE",
        err instanceof Error ? err.message : "getMessages failed",
      );
    }
  }

  getCapabilities(
    principal: Principal,
    threadId: string,
  ): ThreadCapabilitiesResponse {
    const record = this.requireThread(principal, threadId);
    const runtime = this.registry.get(record.runtimeId);
    // 只透传
    return {
      threadId: record.threadId,
      runtimeId: record.runtimeId,
      capabilities: runtime.capabilities(),
    };
  }

  listRuntimes() {
    return this.registry.list();
  }

  poolStats(): AdapterPoolStats[] {
    return this.pools.allStats();
  }

  /**
   * 创建 background run（可随后挂 stream）。
   */
  async createRun(
    principal: Principal,
    threadId: string,
    body: CreateRunRequest,
    options?: { stream?: boolean },
  ): Promise<Run> {
    const record = this.requireThread(principal, threadId);
    if (this.store.findActiveRun(principal.tenantId, threadId)) {
      throw new AgentHttpError(
        "AGENT_RUN_CONFLICT",
        "thread already has an active run",
      );
    }

    const runtime = this.registry.get(record.runtimeId);
    const caps = runtime.capabilities();
    if (body.model && caps.modelSwitch === "none") {
      throw new AgentHttpError(
        "AGENT_CAPABILITY_UNSUPPORTED",
        "model switch not supported",
      );
    }

    const runId = randomUUID();
    const now = new Date().toISOString();
    const run: RunRecord = {
      runId,
      threadId,
      status: "pending",
      createdAt: now,
      updatedAt: now,
      onDisconnect: body.onDisconnect ?? "continue",
      seq: 0,
      metadata: body.metadata,
    };
    this.store.putRunForTenant(principal.tenantId, run);
    this.store.updateThread(principal.tenantId, threadId, {
      status: "busy",
      updatedAt: now,
      lastActivityAt: Date.now(),
    });

    await this.audit(principal, {
      action: "run.create",
      outcome: "success",
      threadId,
      runId,
      workspaceRoot: record.workspaceRoot,
    });

    // 异步执行 prompt
    void this.executeRun(
      principal,
      record,
      run,
      body,
      options?.stream === true,
    );

    return toRun(run);
  }

  getRun(principal: Principal, threadId: string, runId: string): Run {
    this.requireThread(principal, threadId);
    const run = this.store.getRun(principal.tenantId, threadId, runId);
    if (!run) {
      throw new AgentHttpError("AGENT_RUN_NOT_FOUND", "run not found");
    }
    return toRun(run);
  }

  listRuns(
    principal: Principal,
    threadId: string,
    query?: { offset?: number; limit?: number },
  ) {
    this.requireThread(principal, threadId);
    const { offset, limit } = normalizePageQuery(query);
    const all = this.store
      .listRuns(principal.tenantId, threadId)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
    const total = all.length;
    return {
      items: all.slice(offset, offset + limit).map(toRun),
      offset,
      limit,
      hasMore: offset + limit < total,
      total,
    };
  }

  async cancelRun(
    principal: Principal,
    threadId: string,
    runId: string,
  ): Promise<Run> {
    const record = this.requireThread(principal, threadId);
    const run = this.store.getRun(principal.tenantId, threadId, runId);
    if (!run) {
      throw new AgentHttpError("AGENT_RUN_NOT_FOUND", "run not found");
    }
    const runtime = this.registry.get(record.runtimeId);
    await runtime.abort(record.sessionId);
    const now = new Date().toISOString();
    const updated =
      this.store.updateRun(principal.tenantId, threadId, runId, {
        status: "cancelled",
        updatedAt: now,
        endedAt: now,
      }) ?? run;
    this.store.updateThread(principal.tenantId, threadId, {
      status: "idle",
      updatedAt: now,
      lastActivityAt: Date.now(),
    });
    await this.audit(principal, {
      action: "run.cancel",
      outcome: "success",
      threadId,
      runId,
    });
    return toRun(updated);
  }

  async steer(
    principal: Principal,
    threadId: string,
    input: PromptInput,
  ): Promise<void> {
    const record = this.requireThread(principal, threadId);
    const runtime = this.registry.get(record.runtimeId);
    const caps = runtime.capabilities();
    if (!caps.steering || !hasOptionalRuntimeMethod(runtime, "steer")) {
      throw new AgentHttpError(
        "AGENT_CAPABILITY_UNSUPPORTED",
        "steering not supported",
      );
    }
    await runtime.steer?.(record.sessionId, input);
    await this.audit(principal, {
      action: "run.steer",
      outcome: "success",
      threadId,
    });
  }

  async followUp(
    principal: Principal,
    threadId: string,
    input: PromptInput,
  ): Promise<void> {
    const record = this.requireThread(principal, threadId);
    const runtime = this.registry.get(record.runtimeId);
    const caps = runtime.capabilities();
    if (!caps.followUp || !hasOptionalRuntimeMethod(runtime, "followUp")) {
      throw new AgentHttpError(
        "AGENT_CAPABILITY_UNSUPPORTED",
        "followUp not supported",
      );
    }
    await runtime.followUp?.(record.sessionId, input);
    await this.audit(principal, {
      action: "run.follow_up",
      outcome: "success",
      threadId,
    });
  }

  /**
   * 获取 stream buffer；供路由挂 SSE。
   */
  getStreamBuffer(runId: string) {
    return this.streams.getOrCreate(runId);
  }

  getStreamBufferIfExists(runId: string) {
    return this.streams.get(runId);
  }

  /**
   * 客户端断开时按 onDisconnect 处理。
   */
  async onClientDisconnect(
    principal: Principal,
    threadId: string,
    runId: string,
  ): Promise<void> {
    const run = this.store.getRun(principal.tenantId, threadId, runId);
    if (!run) return;
    if (
      run.onDisconnect === "cancel" &&
      (run.status === "running" || run.status === "pending")
    ) {
      await this.cancelRun(principal, threadId, runId);
    }
  }

  // —— internal ——

  private async executeRun(
    principal: Principal,
    thread: ThreadRecord,
    run: RunRecord,
    body: CreateRunRequest,
    _stream: boolean,
  ): Promise<void> {
    const runtime = this.registry.get(thread.runtimeId);
    const caps = runtime.capabilities();
    const buffer = this.streams.getOrCreate(run.runId);
    const mapperState = { reasoningOpen: new Set<string>() };
    let seq = run.seq;

    const nextSeq = () => {
      seq += 1;
      return seq;
    };

    const startedAt = new Date().toISOString();
    this.store.updateRun(principal.tenantId, thread.threadId, run.runId, {
      status: "running",
      startedAt,
      updatedAt: startedAt,
    });

    const input: PromptInput = {
      text: body.input?.text,
      messages: body.input?.messages,
      metadata: body.input?.metadata,
    };

    try {
      if (body.model && hasOptionalRuntimeMethod(runtime, "setModel")) {
        await runtime.setModel?.(thread.sessionId, { id: body.model });
      }

      for await (const event of runtime.prompt(thread.sessionId, input)) {
        // 策略：tool 调用时检查 allowlist
        if (event.type === "tool_call_start") {
          try {
            assertToolAllowed(
              event.toolName,
              this.config.toolAllowlist,
              thread.tools,
            );
            await this.audit(principal, {
              action: "tool.invoke",
              outcome: "success",
              threadId: thread.threadId,
              runId: run.runId,
              resource: event.toolName,
              workspaceRoot: thread.workspaceRoot,
            });
          } catch (err) {
            await this.audit(principal, {
              action: "tool.invoke",
              outcome: "denied",
              threadId: thread.threadId,
              runId: run.runId,
              resource: event.toolName,
              message: err instanceof Error ? err.message : "denied",
            });
            // 策略拒绝：中止并错误结束
            await runtime.abort(thread.sessionId);
            const agui = mapRuntimeEvent(
              {
                type: "run_error",
                sessionId: thread.sessionId,
                message: err instanceof Error ? err.message : "tool denied",
                code: "AGENT_FORBIDDEN",
              },
              {
                threadId: thread.threadId,
                runId: run.runId,
                capabilities: caps,
                nextSeq,
              },
              mapperState,
            );
            for (const e of agui) buffer.push(e);
            throw err;
          }
        }

        if (event.type === "usage") {
          await safeEmitUsage(this.emitter, {
            type: "agent.usage",
            eventId: randomUUID(),
            timestamp: new Date().toISOString(),
            traceId: principal.traceId,
            subject: principal.subject,
            tenantId: principal.tenantId,
            threadId: thread.threadId,
            runId: run.runId,
            runtimeId: thread.runtimeId,
            model: event.model,
            inputTokens: event.inputTokens,
            outputTokens: event.outputTokens,
            totalTokens: event.totalTokens,
          });
        }

        const aguiEvents = mapRuntimeEvent(
          event,
          {
            threadId: thread.threadId,
            runId: run.runId,
            capabilities: caps,
            nextSeq,
          },
          mapperState,
        );
        for (const e of aguiEvents) buffer.push(e);

        // abort 后 mock 会发 run_finished
        const current = this.store.getRun(
          principal.tenantId,
          thread.threadId,
          run.runId,
        );
        if (current?.status === "cancelled") {
          // 补 interrupt 结局
          buffer.push({
            type: "RUN_FINISHED",
            threadId: thread.threadId,
            runId: run.runId,
            outcome: { type: "interrupt", reason: "cancelled" },
            id: `${run.runId}:${nextSeq()}`,
            timestamp: Date.now(),
          });
          break;
        }
      }

      const ended = new Date().toISOString();
      const current = this.store.getRun(
        principal.tenantId,
        thread.threadId,
        run.runId,
      );
      if (current && current.status !== "cancelled") {
        this.store.updateRun(principal.tenantId, thread.threadId, run.runId, {
          status: "success",
          updatedAt: ended,
          endedAt: ended,
          seq,
        });
      } else if (current) {
        this.store.updateRun(principal.tenantId, thread.threadId, run.runId, {
          seq,
        });
      }
      this.store.updateThread(principal.tenantId, thread.threadId, {
        status: "idle",
        updatedAt: ended,
        lastActivityAt: Date.now(),
      });
      this.pools.get(thread.runtimeId)?.touch(thread.sessionId);
    } catch (err) {
      const ended = new Date().toISOString();
      const message = err instanceof Error ? err.message : "run failed";
      this.store.updateRun(principal.tenantId, thread.threadId, run.runId, {
        status: "error",
        updatedAt: ended,
        endedAt: ended,
        error: { code: "AGENT_INTERNAL_ERROR", message },
        seq,
      });
      this.store.updateThread(principal.tenantId, thread.threadId, {
        status: "error",
        updatedAt: ended,
        lastActivityAt: Date.now(),
      });
      buffer.push({
        type: "RUN_ERROR",
        threadId: thread.threadId,
        runId: run.runId,
        message,
        code: "AGENT_INTERNAL_ERROR",
        id: `${run.runId}:${nextSeq()}`,
        timestamp: Date.now(),
      });
    } finally {
      this.streams.scheduleRetention(run.runId);
    }
  }

  private ensurePool(
    runtimeId: string,
    coldStartCost: "high" | "low",
    memoryClass: "light" | "heavy",
  ) {
    const base: PoolConfig = {
      ...this.config.poolByColdStart[coldStartCost],
    };
    if (memoryClass === "heavy") {
      base.maxConcurrency = Math.min(base.maxConcurrency, 3);
    }
    const override = this.config.runtimePoolOverrides[runtimeId];
    const merged = { ...base, ...override };
    return this.pools.ensure(runtimeId, coldStartCost, merged);
  }

  private requireThread(principal: Principal, threadId: string): ThreadRecord {
    const record = this.store.getThread(principal.tenantId, threadId);
    if (!record) {
      throw new AgentHttpError("AGENT_THREAD_NOT_FOUND", "thread not found");
    }
    return record;
  }

  private touch(record: ThreadRecord): void {
    this.store.updateThread(record.tenantId, record.threadId, {
      lastActivityAt: Date.now(),
      updatedAt: new Date().toISOString(),
    });
    this.pools.get(record.runtimeId)?.touch(record.sessionId);
  }

  private async deleteThreadInternal(
    record: ThreadRecord,
    principal?: Principal,
  ): Promise<void> {
    const runtime = this.registry.get(record.runtimeId);
    try {
      await runtime.disposeSession(record.sessionId);
    } catch {
      // 僵尸清理仍删索引
    }
    this.pools.get(record.runtimeId)?.release(record.sessionId);
    this.store.deleteThread(record.tenantId, record.threadId);
    if (principal) {
      await this.audit(principal, {
        action: "thread.delete",
        outcome: "success",
        threadId: record.threadId,
        workspaceRoot: record.workspaceRoot,
      });
      await this.audit(principal, {
        action: "session.dispose",
        outcome: "success",
        threadId: record.threadId,
      });
    }
  }

  private async handleRuntimeFailure(
    record: ThreadRecord,
    _err: unknown,
  ): Promise<void> {
    // 运行时异常：释放池槽并标 thread error
    this.pools.get(record.runtimeId)?.release(record.sessionId);
    this.store.updateThread(record.tenantId, record.threadId, {
      status: "error",
      updatedAt: new Date().toISOString(),
    });
  }

  private async reapIdleSessions(): Promise<void> {
    const now = Date.now();
    for (const thread of this.allThreadsUnsafe()) {
      if (thread.status === "busy") continue;
      if (now - thread.lastActivityAt > this.config.sessionIdleTtlMs) {
        try {
          await this.deleteThreadInternal(thread);
        } catch {
          // continue
        }
      }
    }
  }

  private allThreadsUnsafe(): ThreadRecord[] {
    return this.store.allThreads();
  }

  private async audit(
    principal: Principal,
    partial: Omit<
      AuditEvent,
      "type" | "eventId" | "timestamp" | "traceId" | "subject" | "tenantId"
    >,
  ): Promise<void> {
    await safeEmitAudit(this.emitter, {
      type: "agent.audit",
      eventId: randomUUID(),
      timestamp: new Date().toISOString(),
      traceId: principal.traceId,
      subject: principal.subject,
      tenantId: principal.tenantId,
      ...partial,
    });
  }
}

function toThread(record: ThreadRecord): Thread {
  return {
    threadId: record.threadId,
    status: record.status,
    runtimeId: record.runtimeId,
    profile: record.profile,
    workspaceBound: record.workspaceBound,
    workspaceRoot: record.workspaceRoot,
    model: record.model,
    metadata: record.metadata,
    createdAt: record.createdAt,
    updatedAt: record.updatedAt,
    tenantId: record.tenantId,
    subject: record.subject,
  };
}

function toRun(run: RunRecord): Run {
  return {
    runId: run.runId,
    threadId: run.threadId,
    status: run.status,
    createdAt: run.createdAt,
    updatedAt: run.updatedAt,
    startedAt: run.startedAt,
    endedAt: run.endedAt,
    error: run.error,
    metadata: run.metadata,
  };
}
