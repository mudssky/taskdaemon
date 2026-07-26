/**
 * 进程内 mock gateway：G2 未就绪时支撑 G4 开发与单测。
 * 暴露两个 runtime 模板（pi 全能力 / omp 降级）以验证能力驱动分支。
 */

import {
  type AgentMessage,
  type AguiEvent,
  type CancelRunResponse,
  type CreateRunRequest,
  type CreateThreadRequest,
  type ListMessagesQuery,
  type ListThreadsQuery,
  type ListThreadsResponse,
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
  type RuntimeCapabilities,
  type Thread,
  type ThreadCapabilitiesResponse,
} from "@taskdaemon/agent-protocol";
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
};

const RUNTIMES: RuntimeCatalogItem[] = [
  {
    runtimeId: "pi",
    label: "Pi (full)",
    capabilities: PI_CLI_CAPABILITIES,
  },
  {
    runtimeId: "omp",
    label: "OMP (degraded mock)",
    // 有意降级：验证 name-only / 无 steering / 无 thinking / 无 modelSwitch / general。
    capabilities: {
      ...OMP_CLI_CAPABILITIES,
      steering: false,
      followUp: false,
      thinking: false,
      toolCallDetail: "name-only",
      modelSwitch: "none",
      workspaceBinding: false,
      history: "messages",
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
      let items = [...store.values()].map((s) => s.thread);

      if (query?.status) {
        items = items.filter((t) => t.status === query.status);
      }
      if (query?.runtimeId) {
        items = items.filter((t) => t.runtimeId === query.runtimeId);
      }
      if (query?.profile) {
        items = items.filter((t) => t.profile === query.profile);
      }

      items.sort(
        (a, b) =>
          new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
      );

      const page = items.slice(offset, offset + limit);
      const result: ListThreadsResponse = {
        items: page,
        offset,
        limit,
        hasMore: offset + limit < items.length,
        total: items.length,
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
      });
      return thread;
    },

    async getThread(threadId: string) {
      return requireThread(store, threadId).thread;
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

    async cancelRun(threadId, runId): Promise<CancelRunResponse> {
      const entry = requireThread(store, threadId);
      const controller = entry.abortControllers.get(runId);
      controller?.abort();
      entry.activeRunId = null;
      entry.thread.status = "idle";
      entry.thread.updatedAt = new Date().toISOString();
      return { runId, status: "cancelled" };
    },
  };
}

function seedDemoThreads(store: Map<string, StoredThread>) {
  const now = new Date().toISOString();
  const piId = "thr_demo_pi";
  store.set(piId, {
    thread: {
      threadId: piId,
      status: "idle",
      runtimeId: "pi",
      profile: "coding",
      workspaceBound: true,
      workspaceRoot: "/tmp/demo-pi",
      model: "mock-model",
      metadata: { title: "演示 · Pi 全能力" },
      createdAt: now,
      updatedAt: now,
      tenantId: "dev",
      subject: "dev-user",
    },
    messages: [
      {
        id: "msg_seed_user",
        role: "user",
        content: [{ type: "text", text: "列出当前目录" }],
        createdAt: now,
      },
      {
        id: "msg_seed_asst",
        role: "assistant",
        content: [
          { type: "text", text: "好的，我来查看工作区。" },
          {
            type: "tool_use",
            toolCallId: "tool_seed_1",
            name: "bash",
            args: { command: "ls -la" },
          },
          {
            type: "tool_result",
            toolCallId: "tool_seed_1",
            content: "README.md\nsrc/\npackage.json",
          },
        ],
        createdAt: now,
      },
    ],
    capabilities: PI_CLI_CAPABILITIES,
    activeRunId: null,
    abortControllers: new Map(),
  });

  const ompId = "thr_demo_omp";
  const ompCaps: RuntimeCapabilities = {
    ...OMP_CLI_CAPABILITIES,
    steering: false,
    followUp: false,
    thinking: false,
    toolCallDetail: "name-only",
    modelSwitch: "none",
    workspaceBinding: false,
  };
  store.set(ompId, {
    thread: {
      threadId: ompId,
      status: "idle",
      runtimeId: "omp",
      profile: "general",
      workspaceBound: false,
      workspaceRoot: null,
      model: "mock-omp",
      metadata: { title: "演示 · OMP 降级" },
      createdAt: now,
      updatedAt: now,
      tenantId: "dev",
      subject: "dev-user",
    },
    messages: [
      {
        id: "msg_omp_user",
        role: "user",
        content: [{ type: "text", text: "解释一下能力降级" }],
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
        const withId = {
          ...event,
          id: `${runId}:${seq++}`,
          timestamp: Date.now(),
        };
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

      if (caps.thinking) {
        await emit({ type: "REASONING_START", messageId });
        await emit({
          type: "REASONING_CONTENT",
          messageId,
          delta: "规划：先回答再演示工具。",
        });
        await emit({ type: "REASONING_END", messageId });
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

      if (caps.toolCallDetail !== "none" && userText.includes("tool")) {
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

      // 代码块示例
      if (userText.includes("code")) {
        await emit({
          type: "TEXT_MESSAGE_CONTENT",
          messageId,
          delta: "\n\n```ts\nconst ok = true;\n```\n",
        });
      }

      await emit({ type: "TEXT_MESSAGE_END", messageId });

      const finalText =
        reply +
        (userText.includes("code") ? "\n\n```ts\nconst ok = true;\n```\n" : "");
      const content: AgentMessage["content"] = [
        { type: "text", text: finalText },
      ];
      if (caps.thinking) {
        content.unshift({
          type: "thinking",
          text: "规划：先回答再演示工具。",
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

      entry.activeRunId = null;
      entry.thread.status = "idle";
      entry.thread.updatedAt = new Date().toISOString();
      entry.abortControllers.delete(runId);
      handlers.onDone();
    } catch (error) {
      if ((error as Error).name === "AbortError") {
        handlers.onEvent({
          type: "RUN_FINISHED",
          threadId,
          runId,
          outcome: { type: "interrupt", reason: "cancelled" },
          id: `${runId}:cancel`,
          timestamp: Date.now(),
        });
        entry.activeRunId = null;
        entry.thread.status = "idle";
        entry.thread.updatedAt = new Date().toISOString();
        entry.abortControllers.delete(runId);
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
    `thread not found: ${threadId}`,
    { threadId },
    "mock-trace",
  );
}

function cryptoRandomId(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `${prefix}_${crypto.randomUUID().slice(0, 8)}`;
  }
  return `${prefix}_${Math.random().toString(16).slice(2, 10)}`;
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException("aborted", "AbortError"));
      return;
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
  });
}
