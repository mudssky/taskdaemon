/**
 * 进程内 mock gateway：支撑 G4/G5 开发与单测。
 * 双 runtime 模板（pi 全能力 / omp 降级）验证能力驱动分支。
 */

import {
  type AgentMessage,
  type AguiEvent,
  type CancelRunResponse,
  type CreateRunRequest,
  type CreateThreadRequest,
  type ForkThreadRequest,
  type HitlRespondRequest,
  type HitlRespondResponse,
  type ListMessagesQuery,
  type ListThreadsQuery,
  type ListThreadsResponse,
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
  type RuntimeCapabilities,
  type SteerRequest,
  type SteerResponse,
  type StreamReconnectQuery,
  type Thread,
  type ThreadCapabilitiesResponse,
  type UpdateThreadRequest,
} from "@taskdaemon/agent-protocol";
import {
  buildForkedThread,
  filterAndSortThreads,
  sliceMessagesForFork,
} from "../../features/agent/session-filters";
import type {
  AgentApi,
  RuntimeCatalogItem,
  StreamHandle,
  StreamHandlers,
} from "./client";
import { AgentApiError } from "./errors";

type StoredThread = {
  thread: Thread;
  messages: AgentMessage[];
  capabilities: RuntimeCapabilities;
  activeRunId: string | null;
  abortControllers: Map<string, AbortController>;
  /** run 事件环缓，供断线续传。 */
  runBuffers: Map<string, AguiEvent[]>;
  pendingHitl?: {
    requestId: string;
    resolve: (decision: HitlRespondRequest) => void;
  };
};

const RUNTIMES: RuntimeCatalogItem[] = [
  {
    runtimeId: "pi",
    label: "pi (full)",
    capabilities: {
      ...PI_CLI_CAPABILITIES,
      thinking: true,
      steering: true,
      toolCallDetail: "full",
      workspaceBinding: true,
    },
  },
  {
    runtimeId: "omp",
    label: "omp (degraded demo)",
    capabilities: {
      ...OMP_CLI_CAPABILITIES,
      thinking: false,
      steering: false,
      toolCallDetail: "name-only",
      workspaceBinding: false,
      modelSwitch: "none",
    },
  },
];

/**
 * 创建内存 mock AgentApi。
 *
 * 参数:
 *   - options.delayMs: 流式 chunk 间隔，默认 20。
 *
 * 返回值:
 *   - AgentApi。
 */
export function createMockAgentApi(options?: { delayMs?: number }): AgentApi {
  const delayMs = options?.delayMs ?? 20;
  const store = new Map<string, StoredThread>();
  seedDemoThreads(store);

  return {
    async listRuntimes() {
      return RUNTIMES.map((item) => ({ ...item }));
    },

    async listThreads(query?: ListThreadsQuery) {
      const offset = query?.offset ?? 0;
      const limit = Math.min(200, Math.max(1, query?.limit ?? 50));
      const prepared = [...store.values()].map((s) => ({
        thread: s.thread,
        searchableText: s.messages
          .map((m) =>
            m.content
              .map((b) =>
                b.type === "text" || b.type === "thinking" ? b.text : "",
              )
              .join(" "),
          )
          .join("\n"),
      }));
      const filtered = filterAndSortThreads(prepared, {
        q: query?.q,
        status: query?.status,
        runtimeId: query?.runtimeId,
        profile: query?.profile,
        includeArchived: query?.includeArchived,
        sort: query?.sort,
      });
      const page = filtered.slice(offset, offset + limit);
      const result: ListThreadsResponse = {
        items: page,
        offset,
        limit,
        hasMore: offset + limit < filtered.length,
        total: filtered.length,
      };
      return result;
    },

    async createThread(body: CreateThreadRequest) {
      const runtimeId = body.runtimeId ?? "pi";
      const catalog = RUNTIMES.find((r) => r.runtimeId === runtimeId);
      if (!catalog) {
        throw new AgentApiError(
          503,
          "AGENT_RUNTIME_UNAVAILABLE",
          `runtime not found: ${runtimeId}`,
          { runtimeId },
          "mock-trace",
        );
      }

      const profile = body.profile ?? "coding";
      const workspaceBinding =
        body.workspaceBinding ??
        (profile === "general" ? false : catalog.capabilities.workspaceBinding);

      const now = new Date().toISOString();
      const threadId = body.threadId ?? cryptoRandomId("thr");
      const thread: Thread = {
        threadId,
        status: "idle",
        runtimeId,
        profile,
        workspaceBound: workspaceBinding,
        workspaceRoot: workspaceBinding
          ? (body.workspaceRoot ?? "/tmp/mock-workspace")
          : null,
        model: body.model ?? "mock-model",
        metadata: {
          title: body.metadata?.title ?? `会话 ${threadId.slice(-6)}`,
          ...body.metadata,
        },
        createdAt: now,
        updatedAt: now,
        tenantId: "dev",
        subject: "dev-user",
      };

      const capabilities: RuntimeCapabilities = {
        ...catalog.capabilities,
        workspaceBinding,
      };

      store.set(threadId, {
        thread,
        messages: [],
        capabilities,
        activeRunId: null,
        abortControllers: new Map(),
        runBuffers: new Map(),
      });
      return thread;
    },

    async getThread(threadId: string) {
      return requireThread(store, threadId).thread;
    },

    async updateThread(threadId: string, body: UpdateThreadRequest) {
      const entry = requireThread(store, threadId);
      if (body.title !== undefined) {
        entry.thread.metadata = {
          ...entry.thread.metadata,
          title: body.title,
        };
      }
      if (body.metadata) {
        entry.thread.metadata = {
          ...entry.thread.metadata,
          ...body.metadata,
        };
      }
      if (body.archived === true) {
        entry.thread.status = "archived";
      } else if (
        body.archived === false &&
        entry.thread.status === "archived"
      ) {
        entry.thread.status = "idle";
      }
      entry.thread.updatedAt = new Date().toISOString();
      return entry.thread;
    },

    async forkThread(threadId: string, body: ForkThreadRequest = {}) {
      const entry = requireThread(store, threadId);
      const now = new Date().toISOString();
      const newId = cryptoRandomId("thr");
      const forked = buildForkedThread(entry.thread, newId, body.title, now);
      const messages = sliceMessagesForFork(entry.messages, body.fromMessageId);
      store.set(newId, {
        thread: forked,
        messages,
        capabilities: { ...entry.capabilities },
        activeRunId: null,
        abortControllers: new Map(),
        runBuffers: new Map(),
      });
      // 源会话不变
      return forked;
    },

    async deleteThread(threadId: string) {
      if (!store.has(threadId)) {
        throw notFound(threadId);
      }
      store.delete(threadId);
    },

    async listMessages(threadId: string, query?: ListMessagesQuery) {
      const entry = requireThread(store, threadId);
      const offset = query?.offset ?? 0;
      const limit = Math.min(200, Math.max(1, query?.limit ?? 50));
      const items = entry.messages.slice(offset, offset + limit);
      return {
        items,
        offset,
        limit,
        hasMore: offset + limit < entry.messages.length,
        total: entry.messages.length,
      };
    },

    async getCapabilities(
      threadId: string,
    ): Promise<ThreadCapabilitiesResponse> {
      const entry = requireThread(store, threadId);
      return {
        threadId,
        runtimeId: entry.thread.runtimeId,
        capabilities: entry.capabilities,
      };
    },

    streamRun(threadId, body, handlers) {
      return startMockStream(store, threadId, body, handlers, delayMs);
    },

    reconnectRunStream(threadId, runId, query, handlers) {
      return replayMockStream(store, threadId, runId, query, handlers, delayMs);
    },

    async cancelRun(threadId, runId): Promise<CancelRunResponse> {
      const entry = requireThread(store, threadId);
      const controller = entry.abortControllers.get(runId);
      controller?.abort();
      entry.activeRunId = null;
      entry.thread.status = "idle";
      entry.thread.updatedAt = new Date().toISOString();
      entry.pendingHitl = undefined;
      return { runId, status: "cancelled" };
    },

    async steer(threadId: string, body: SteerRequest): Promise<SteerResponse> {
      const entry = requireThread(store, threadId);
      if (!entry.capabilities.steering) {
        throw new AgentApiError(
          400,
          "AGENT_CAPABILITY_UNSUPPORTED",
          "steering not supported",
          {},
          "mock-trace",
        );
      }
      const text = body.input?.text?.trim() ?? "";
      if (text) {
        entry.messages.push({
          id: cryptoRandomId("msg"),
          role: "user",
          content: [{ type: "text", text: `[steer] ${text}` }],
          createdAt: new Date().toISOString(),
          metadata: { kind: "steer" },
        });
        entry.thread.updatedAt = new Date().toISOString();
      }
      return { ok: true, threadId };
    },

    async respondHitl(
      threadId: string,
      body: HitlRespondRequest,
    ): Promise<HitlRespondResponse> {
      const entry = requireThread(store, threadId);
      if (
        !entry.pendingHitl ||
        entry.pendingHitl.requestId !== body.requestId
      ) {
        throw new AgentApiError(
          422,
          "AGENT_VALIDATION_FAILED",
          "no pending hitl request",
          { requestId: body.requestId },
          "mock-trace",
        );
      }
      entry.pendingHitl.resolve(body);
      entry.pendingHitl = undefined;
      return {
        ok: true,
        requestId: body.requestId,
        decision: body.decision,
      };
    },
  };
}

function seedDemoThreads(store: Map<string, StoredThread>) {
  const now = new Date().toISOString();
  const piCaps: RuntimeCapabilities = {
    ...PI_CLI_CAPABILITIES,
    thinking: true,
    steering: true,
    toolCallDetail: "full",
    workspaceBinding: true,
  };
  const ompCaps: RuntimeCapabilities = {
    ...OMP_CLI_CAPABILITIES,
    thinking: false,
    steering: false,
    toolCallDetail: "name-only",
    workspaceBinding: false,
    modelSwitch: "none",
  };

  const piId = "thr_demo_pi";
  store.set(piId, {
    thread: {
      threadId: piId,
      status: "idle",
      runtimeId: "pi",
      profile: "coding",
      workspaceBound: true,
      workspaceRoot: "/tmp/mock-workspace",
      model: "mock-model",
      metadata: { title: "演示 · pi 全能力" },
      createdAt: now,
      updatedAt: now,
      tenantId: "dev",
      subject: "dev-user",
    },
    messages: [
      {
        id: "msg_pi_user",
        role: "user",
        content: [{ type: "text", text: "你好，演示会话" }],
        createdAt: now,
      },
      {
        id: "msg_pi_asst",
        role: "assistant",
        content: [
          { type: "thinking", text: "先打招呼再说明能力。" },
          {
            type: "text",
            text: "本会话具备 thinking / steering / workspace。输入 hitl / file / tool 触发演示。",
          },
        ],
        createdAt: now,
      },
    ],
    capabilities: piCaps,
    activeRunId: null,
    abortControllers: new Map(),
    runBuffers: new Map(),
  });

  const ompId = "thr_demo_omp";
  store.set(ompId, {
    thread: {
      threadId: ompId,
      status: "idle",
      runtimeId: "omp",
      profile: "general",
      workspaceBound: false,
      workspaceRoot: null,
      model: "mock-model",
      metadata: { title: "演示 · omp 降级" },
      createdAt: now,
      updatedAt: now,
      tenantId: "dev",
      subject: "dev-user",
    },
    messages: [
      {
        id: "msg_omp_user",
        role: "user",
        content: [{ type: "text", text: "general 会话" }],
        createdAt: now,
      },
      {
        id: "msg_omp_asst",
        role: "assistant",
        content: [
          {
            type: "text",
            text: "本会话 tool 仅 name-only，无 thinking / steering / workspace。",
          },
        ],
        createdAt: now,
      },
    ],
    capabilities: ompCaps,
    activeRunId: null,
    abortControllers: new Map(),
    runBuffers: new Map(),
  });
}

function startMockStream(
  store: Map<string, StoredThread>,
  threadId: string,
  body: CreateRunRequest,
  handlers: StreamHandlers,
  delayMs: number,
): StreamHandle {
  const entry = requireThread(store, threadId);
  if (entry.activeRunId) {
    throw new AgentApiError(
      409,
      "AGENT_RUN_CONFLICT",
      "thread already has an active run",
      {},
      "mock-trace",
    );
  }

  const runId = cryptoRandomId("run");
  const controller = new AbortController();
  entry.abortControllers.set(runId, controller);
  entry.activeRunId = runId;
  entry.thread.status = "busy";
  entry.thread.updatedAt = new Date().toISOString();
  entry.runBuffers.set(runId, []);

  const userText = body.input?.text?.trim() ?? "";
  if (userText) {
    entry.messages.push({
      id: cryptoRandomId("msg"),
      role: "user",
      content: [{ type: "text", text: userText }],
      createdAt: new Date().toISOString(),
    });
  }

  const runIdPromise = Promise.resolve(runId);

  void (async () => {
    try {
      let seq = 0;
      const emit = async (event: AguiEvent) => {
        if (controller.signal.aborted) {
          throw new DOMException("aborted", "AbortError");
        }
        const withId: AguiEvent = {
          ...event,
          id: `${runId}:${seq++}`,
          timestamp: Date.now(),
        };
        entry.runBuffers.get(runId)?.push(withId);
        handlers.onEvent(withId);
        if (delayMs > 0) {
          await sleep(delayMs, controller.signal);
        }
      };

      await emit({
        type: "RUN_STARTED",
        threadId,
        runId,
      });

      const messageId = cryptoRandomId("msg");
      const caps = entry.capabilities;
      const lower = userText.toLowerCase();

      if (caps.thinking) {
        await emit({ type: "REASONING_START", messageId });
        await emit({
          type: "REASONING_CONTENT",
          messageId,
          delta: "规划：按能力分支演示 G5 特性。",
        });
        await emit({ type: "REASONING_END", messageId });
      }

      // HITL 演示：关键词 hitl / confirm
      if (lower.includes("hitl") || lower.includes("confirm")) {
        const requestId = cryptoRandomId("hitl");
        await emit({
          type: "CUSTOM",
          name: "hitl_request",
          value: {
            requestId,
            title: "确认执行敏感操作",
            context: `用户请求：${userText}\n是否继续在沙箱内执行？`,
            options: [
              { id: "approve", label: "批准" },
              { id: "reject", label: "拒绝" },
            ],
            timeoutMs: 120_000,
          },
        });

        const decision = await waitForHitl(entry, requestId, controller.signal);
        if (decision.decision === "reject") {
          await emit({
            type: "TEXT_MESSAGE_START",
            messageId,
            role: "assistant",
          });
          await emit({
            type: "TEXT_MESSAGE_CONTENT",
            messageId,
            delta: "已根据您的拒绝中止操作。",
          });
          await emit({ type: "TEXT_MESSAGE_END", messageId });
          entry.messages.push({
            id: messageId,
            role: "assistant",
            content: [{ type: "text", text: "已根据您的拒绝中止操作。" }],
            createdAt: new Date().toISOString(),
          });
          await emit({
            type: "RUN_FINISHED",
            threadId,
            runId,
            outcome: { type: "interrupt", reason: "hitl_rejected" },
          });
          finishRun(entry, runId);
          handlers.onDone();
          return;
        }
      }

      await emit({
        type: "TEXT_MESSAGE_START",
        messageId,
        role: "assistant",
      });

      const reply = buildMockReply(userText, entry.thread.runtimeId);
      for (const chunk of chunkText(reply, 12)) {
        await emit({
          type: "TEXT_MESSAGE_CONTENT",
          messageId,
          delta: chunk,
        });
      }

      if (caps.toolCallDetail !== "none" && lower.includes("tool")) {
        const toolCallId = cryptoRandomId("tool");
        await emit({
          type: "TOOL_CALL_START",
          toolCallId,
          toolCallName: "echo",
          parentMessageId: messageId,
        });
        if (caps.toolCallDetail === "full") {
          await emit({
            type: "TOOL_CALL_ARGS",
            toolCallId,
            delta: JSON.stringify({ text: userText }, null, 2),
          });
        }
        await emit({ type: "TOOL_CALL_END", toolCallId });
        await emit({
          type: "TOOL_CALL_RESULT",
          toolCallId,
          content: `echo ok: ${userText}`,
          messageId,
        });
      }

      // 工作区 file_change（仅 workspaceBound）
      if (
        entry.thread.workspaceBound &&
        caps.workspaceBinding &&
        (lower.includes("file") ||
          lower.includes("edit") ||
          lower.includes("diff"))
      ) {
        await emit({
          type: "CUSTOM",
          name: "file_change",
          value: {
            path: "src/hello.ts",
            kind: "modify",
            diff: "--- a/src/hello.ts\n+++ b/src/hello.ts\n@@ -1,3 +1,4 @@\n export function hello() {\n-  return 'hi'\n+  return 'hello world'\n }\n",
          },
        });
        await emit({
          type: "CUSTOM",
          name: "file_change",
          value: {
            path: "/etc/passwd",
            kind: "modify",
            outsideSandbox: true,
            diff: "（越界路径，仅提示，无真实 diff）",
          },
        });
      }

      if (lower.includes("code")) {
        await emit({
          type: "TEXT_MESSAGE_CONTENT",
          messageId,
          delta: "\n\n```ts\nconst ok = true;\n```\n",
        });
      }

      await emit({ type: "TEXT_MESSAGE_END", messageId });

      await emit({
        type: "CUSTOM",
        name: "usage",
        value: { inputTokens: 12, outputTokens: 34, totalTokens: 46 },
      });

      const finalText =
        reply +
        (lower.includes("code") ? "\n\n```ts\nconst ok = true;\n```\n" : "");
      const content: AgentMessage["content"] = [
        { type: "text", text: finalText },
      ];
      if (caps.thinking) {
        content.unshift({
          type: "thinking",
          text: "规划：按能力分支演示 G5 特性。",
        });
      }
      entry.messages.push({
        id: messageId,
        role: "assistant",
        content,
        createdAt: new Date().toISOString(),
      });

      await emit({
        type: "RUN_FINISHED",
        threadId,
        runId,
        outcome: { type: "success" },
      });

      finishRun(entry, runId);
      handlers.onDone();
    } catch (error) {
      if ((error as Error).name === "AbortError") {
        const cancelEvent: AguiEvent = {
          type: "RUN_FINISHED",
          threadId,
          runId,
          outcome: { type: "interrupt", reason: "cancelled" },
          id: `${runId}:cancel`,
          timestamp: Date.now(),
        };
        entry.runBuffers.get(runId)?.push(cancelEvent);
        handlers.onEvent(cancelEvent);
        finishRun(entry, runId);
        handlers.onDone();
        return;
      }
      entry.activeRunId = null;
      entry.thread.status = "error";
      handlers.onError(
        error instanceof Error ? error : new Error(String(error)),
      );
    }
  })();

  return {
    abort: () => controller.abort(),
    runId: runIdPromise,
  };
}

function replayMockStream(
  store: Map<string, StoredThread>,
  threadId: string,
  runId: string,
  query: StreamReconnectQuery | undefined,
  handlers: StreamHandlers,
  delayMs: number,
): StreamHandle {
  const entry = requireThread(store, threadId);
  const buffer = entry.runBuffers.get(runId);
  if (!buffer) {
    const err = new AgentApiError(
      404,
      "AGENT_RUN_NOT_FOUND",
      "run buffer not found",
      { runId },
      "mock-trace",
    );
    queueMicrotask(() => handlers.onError(err));
    return {
      abort: () => undefined,
      runId: Promise.resolve(runId),
    };
  }

  const cursor = query?.cursor ?? query?.lastEventId;
  let startIdx = 0;
  if (cursor) {
    const idx = buffer.findIndex((e) => e.id === cursor);
    if (idx === -1) {
      const err = new AgentApiError(
        400,
        "AGENT_STREAM_CURSOR_INVALID",
        "stream cursor invalid",
        { cursor },
        "mock-trace",
      );
      queueMicrotask(() => handlers.onError(err));
      return {
        abort: () => undefined,
        runId: Promise.resolve(runId),
      };
    }
    startIdx = idx + 1;
  }

  const controller = new AbortController();
  void (async () => {
    try {
      for (const event of buffer.slice(startIdx)) {
        if (controller.signal.aborted) {
          break;
        }
        handlers.onEvent(event);
        if (delayMs > 0) {
          await sleep(Math.min(delayMs, 5), controller.signal);
        }
      }
      // 若 run 仍 active，保持连接直到 abort / 结束由主生成器继续——
      // mock 简化：重放缓冲后 onDone（主生成器若仍在跑会继续 push 到 buffer，
      // 真实场景应挂 live subscriber；G5 测试覆盖缓冲重放与去重）。
      handlers.onDone();
    } catch (error) {
      if ((error as Error).name === "AbortError") {
        handlers.onDone();
        return;
      }
      handlers.onError(
        error instanceof Error ? error : new Error(String(error)),
      );
    }
  })();

  return {
    abort: () => controller.abort(),
    runId: Promise.resolve(runId),
  };
}

function waitForHitl(
  entry: StoredThread,
  requestId: string,
  signal: AbortSignal,
): Promise<HitlRespondRequest> {
  const { promise, resolve, reject } =
    Promise.withResolvers<HitlRespondRequest>();
  if (signal.aborted) {
    reject(new DOMException("aborted", "AbortError"));
    return promise;
  }
  const onAbort = () => {
    entry.pendingHitl = undefined;
    reject(new DOMException("aborted", "AbortError"));
  };
  signal.addEventListener("abort", onAbort, { once: true });
  entry.pendingHitl = {
    requestId,
    resolve: (decision) => {
      signal.removeEventListener("abort", onAbort);
      resolve(decision);
    },
  };
  return promise;
}

function finishRun(entry: StoredThread, runId: string) {
  entry.activeRunId = null;
  entry.thread.status = "idle";
  entry.thread.updatedAt = new Date().toISOString();
  entry.abortControllers.delete(runId);
  entry.pendingHitl = undefined;
}

function buildMockReply(userText: string, runtimeId: string): string {
  if (!userText) {
    return `（${runtimeId}）空输入，mock 仍返回一条助手消息。`;
  }
  return `（${runtimeId} mock）收到：${userText}`;
}

function chunkText(text: string, size: number): string[] {
  const chunks: string[] = [];
  for (let i = 0; i < text.length; i += size) {
    chunks.push(text.slice(i, i + size));
  }
  return chunks.length > 0 ? chunks : [""];
}

function requireThread(
  store: Map<string, StoredThread>,
  threadId: string,
): StoredThread {
  const entry = store.get(threadId);
  if (!entry) {
    throw notFound(threadId);
  }
  return entry;
}

function notFound(threadId: string): AgentApiError {
  return new AgentApiError(
    404,
    "AGENT_THREAD_NOT_FOUND",
    "thread not found",
    { threadId },
    "mock-trace",
  );
}

function cryptoRandomId(prefix: string): string {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  const { promise, resolve, reject } = Promise.withResolvers<void>();
  if (signal.aborted) {
    reject(new DOMException("aborted", "AbortError"));
    return promise;
  }
  const timer = setTimeout(() => {
    signal.removeEventListener("abort", onAbort);
    resolve();
  }, ms);
  const onAbort = () => {
    clearTimeout(timer);
    reject(new DOMException("aborted", "AbortError"));
  };
  signal.addEventListener("abort", onAbort, { once: true });
  return promise;
}
