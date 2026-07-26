/**
 * Mock AgentRuntime：覆盖 coldStartCost high/low，供编排测试。
 * G3 真实 adapter 替换本模块即可。
 */

import { randomUUID } from "node:crypto";
import type {
  AgentRuntime,
  HistoryPage,
  MessagePage,
  ModelInfo,
  ModelRef,
  PromptInput,
  RuntimeCapabilities,
  RuntimeEvent,
  RuntimeState,
  SessionHandle,
  SessionOpts,
} from "@taskdaemon/agent-protocol";

export type MockAdapterOptions = {
  id: string;
  capabilities: RuntimeCapabilities;
  /** 模拟冷启动延迟（毫秒）。 */
  coldStartDelayMs?: number;
  /** 创建时是否立即标记进程存活。 */
  autoCrashAfterMs?: number;
};

type MockSession = {
  handle: SessionHandle;
  opts: SessionOpts;
  messages: Array<{
    id: string;
    role: "user" | "assistant";
    text: string;
    createdAt: string;
  }>;
  isStreaming: boolean;
  aborted: boolean;
  disposed: boolean;
  crashed: boolean;
  model?: string;
  abortControllers: Set<AbortController>;
};

/**
 * 创建 mock capabilities。
 *
 * 参数:
 *   - coldStartCost: high | low。
 *   - memoryClass: light | heavy。
 *
 * 返回值:
 *   - RuntimeCapabilities。
 */
export function mockCapabilities(
  coldStartCost: "high" | "low",
  memoryClass: "light" | "heavy" = "light",
): RuntimeCapabilities {
  return {
    profile: ["coding", "general"],
    attachMode: coldStartCost === "high" ? "cli-spawn" : "sdk-inprocess",
    coldStartCost,
    steering: true,
    followUp: true,
    thinking: true,
    toolCallDetail: "full",
    modelSwitch: "session",
    workspaceBinding: true,
    sessionPersistence: "runtime",
    abortContinuesSession: true,
    history: "messages",
    memoryClass,
  };
}

/** Mock 运行时实现。 */
export class MockAgentRuntime implements AgentRuntime {
  readonly id: string;
  private readonly caps: RuntimeCapabilities;
  private readonly coldStartDelayMs: number;
  private readonly sessions = new Map<string, MockSession>();
  /** 已创建但未 dispose 的「进程」数（测试观测）。 */
  processCount = 0;
  /** 模拟预热槽位（由 pool 管理；runtime 只记 create 次数）。 */
  createCount = 0;

  /**
   * 参数:
   *   - options: MockAdapterOptions。
   */
  constructor(options: MockAdapterOptions) {
    this.id = options.id;
    this.caps = options.capabilities;
    this.coldStartDelayMs = options.coldStartDelayMs ?? 0;
  }

  /**
   * 返回能力声明（只读拷贝语义：返回同一引用，gateway 不得改写）。
   */
  capabilities(): RuntimeCapabilities {
    return this.caps;
  }

  /**
   * 创建会话。
   */
  async createSession(opts: SessionOpts): Promise<SessionHandle> {
    if (this.coldStartDelayMs > 0) {
      await delay(this.coldStartDelayMs);
    }
    const sessionId = randomUUID();
    const handle: SessionHandle = {
      sessionId,
      runtimeId: this.id,
      sessionFile: `/tmp/mock-${this.id}-${sessionId}.jsonl`,
      createdAt: new Date().toISOString(),
    };
    this.sessions.set(sessionId, {
      handle,
      opts,
      messages: [],
      isStreaming: false,
      aborted: false,
      disposed: false,
      crashed: false,
      model: opts.model,
      abortControllers: new Set(),
    });
    this.processCount += 1;
    this.createCount += 1;
    return handle;
  }

  /**
   * 流式 prompt。
   */
  async *prompt(
    sessionId: string,
    input: PromptInput,
  ): AsyncIterable<RuntimeEvent> {
    const session = this.requireSession(sessionId);
    if (session.crashed) {
      yield {
        type: "run_error",
        sessionId,
        message: "runtime process crashed",
        code: "AGENT_RUNTIME_UNAVAILABLE",
      };
      return;
    }
    const ac = new AbortController();
    session.abortControllers.add(ac);
    session.isStreaming = true;
    session.aborted = false;

    const userText = input.text ?? "";
    const userMsgId = randomUUID();
    session.messages.push({
      id: userMsgId,
      role: "user",
      text: userText,
      createdAt: new Date().toISOString(),
    });

    yield { type: "run_started", sessionId };
    if (ac.signal.aborted) {
      session.isStreaming = false;
      yield { type: "run_finished", sessionId };
      session.abortControllers.delete(ac);
      return;
    }

    const assistantId = randomUUID();
    yield {
      type: "message_start",
      sessionId,
      messageId: assistantId,
      role: "assistant",
    };
    const reply = `echo:${userText}`;
    yield {
      type: "message_text_delta",
      sessionId,
      messageId: assistantId,
      delta: reply,
    };
    yield { type: "message_end", sessionId, messageId: assistantId };
    session.messages.push({
      id: assistantId,
      role: "assistant",
      text: reply,
      createdAt: new Date().toISOString(),
    });

    yield {
      type: "usage",
      sessionId,
      inputTokens: Math.max(1, userText.length),
      outputTokens: reply.length,
      totalTokens: Math.max(1, userText.length) + reply.length,
      model: session.model ?? "mock-model",
    };

    if (ac.signal.aborted) {
      session.isStreaming = false;
      yield { type: "run_finished", sessionId };
      session.abortControllers.delete(ac);
      return;
    }

    session.isStreaming = false;
    yield { type: "run_finished", sessionId };
    session.abortControllers.delete(ac);
  }

  async steer(sessionId: string, input: PromptInput): Promise<void> {
    this.requireSession(sessionId);
    void input;
  }

  async followUp(sessionId: string, input: PromptInput): Promise<void> {
    this.requireSession(sessionId);
    void input;
  }

  async abort(sessionId: string): Promise<void> {
    const session = this.requireSession(sessionId);
    session.aborted = true;
    session.isStreaming = false;
    for (const ac of session.abortControllers) {
      ac.abort();
    }
  }

  async getState(sessionId: string): Promise<RuntimeState> {
    const session = this.requireSession(sessionId);
    return {
      sessionId,
      isStreaming: session.isStreaming,
      model: session.model
        ? { id: session.model, name: session.model }
        : undefined,
      messageCount: session.messages.length,
    };
  }

  async getMessages(
    sessionId: string,
    page?: HistoryPage,
  ): Promise<MessagePage> {
    const session = this.requireSession(sessionId);
    const offset = page?.offset ?? 0;
    const limit = page?.limit ?? 50;
    const slice = session.messages.slice(offset, offset + limit);
    return {
      messages: slice.map((m) => ({
        id: m.id,
        role: m.role,
        content: [{ type: "text" as const, text: m.text }],
        createdAt: m.createdAt,
      })),
      offset,
      limit,
      hasMore: offset + limit < session.messages.length,
    };
  }

  async setModel(sessionId: string, model: ModelRef): Promise<void> {
    const session = this.requireSession(sessionId);
    session.model = model.id;
  }

  async listModels(sessionId: string): Promise<ModelInfo[]> {
    this.requireSession(sessionId);
    return [{ id: "mock-model", name: "Mock Model" }];
  }

  async disposeSession(sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId);
    if (!session) return;
    session.disposed = true;
    for (const ac of session.abortControllers) {
      ac.abort();
    }
    this.sessions.delete(sessionId);
    this.processCount = Math.max(0, this.processCount - 1);
  }

  /**
   * 测试辅助：模拟进程崩溃。
   *
   * 参数:
   *   - sessionId: 会话 id。
   */
  crashSession(sessionId: string): void {
    const session = this.sessions.get(sessionId);
    if (!session) return;
    session.crashed = true;
    session.isStreaming = false;
    for (const ac of session.abortControllers) {
      ac.abort();
    }
  }

  /**
   * 测试辅助：会话是否已 dispose。
   */
  isDisposed(sessionId: string): boolean {
    return !this.sessions.has(sessionId);
  }

  /**
   * 测试辅助：是否收到 abort。
   */
  wasAborted(sessionId: string): boolean {
    return this.sessions.get(sessionId)?.aborted ?? false;
  }

  private requireSession(sessionId: string): MockSession {
    const session = this.sessions.get(sessionId);
    if (!session || session.disposed) {
      throw new Error(`session not found: ${sessionId}`);
    }
    return session;
  }
}

/**
 * 创建默认 mock 运行时集合。
 *
 * 返回值:
 *   - MockAgentRuntime[]。
 */
export function createDefaultMockRuntimes(): MockAgentRuntime[] {
  return [
    new MockAgentRuntime({
      id: "mock-high",
      capabilities: mockCapabilities("high", "light"),
      coldStartDelayMs: 0,
    }),
    new MockAgentRuntime({
      id: "mock-low",
      capabilities: mockCapabilities("low", "light"),
      coldStartDelayMs: 0,
    }),
    new MockAgentRuntime({
      id: "mock-high-heavy",
      capabilities: mockCapabilities("high", "heavy"),
      coldStartDelayMs: 0,
    }),
  ];
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}
