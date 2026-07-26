/**
 * 共享 CLI RPC AgentRuntime 实现。
 * Pi / OMP 仅在 spawn 参数、capabilities、general 裁剪上分化。
 */

import { randomUUID } from "node:crypto";
import { mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type {
  AgentMessage,
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
import {
  AgentRuntimeError,
  capabilityUnsupportedError,
  sessionNotFoundError,
  wrapInternalError,
} from "./errors.js";
import {
  createEventMapperState,
  mapNativeToRuntimeEvents,
} from "./event-mapper.js";
import type { RuntimeLogger } from "./log.js";
import { createRuntimeLogger, withLogFields } from "./log.js";
import { ProcessTransport } from "./process-transport.js";
import {
  type NativeMessage,
  RpcClient,
  type RpcTransport,
} from "./rpc-client.js";
import { sanitizeModelInfo, sanitizeSecrets } from "./sanitize.js";

export type TransportFactory = (ctx: {
  sessionLocalId: string;
  opts: SessionOpts;
  sessionDir: string;
}) => RpcTransport | Promise<RpcTransport>;

export type CliAdapterConfig = {
  id: string;
  capabilities: RuntimeCapabilities;
  /**
   * 构造 CLI 参数（不含 bin）。
   * 参数: opts, sessionDir。
   * 返回值: { command, args, cwd? }。
   */
  buildSpawn: (
    opts: SessionOpts,
    sessionDir: string,
  ) => { command: string; args: string[]; cwd?: string };
  /**
   * general profile 额外 RPC（OMP 裁剪人格/工具）。
   */
  afterReady?: (client: RpcClient, opts: SessionOpts) => Promise<void>;
  /** 注入传输（测试用）；默认 ProcessTransport。 */
  transportFactory?: TransportFactory;
  logger?: RuntimeLogger;
  /** 就绪探测超时。 */
  readyTimeoutMs?: number;
  defaultModel?: string;
  defaultThinking?: string;
  traceId?: string;
};

type SessionRecord = {
  handle: SessionHandle;
  client: RpcClient;
  transport: RpcTransport;
  opts: SessionOpts;
  allowFileChange: boolean;
  disposed: boolean;
  abortRequested: boolean;
  processPid?: number;
};

/**
 * 基于 JSONL RPC 的 AgentRuntime。
 */
export class CliRpcAgentRuntime implements AgentRuntime {
  readonly id: string;
  private readonly config: CliAdapterConfig;
  private readonly logger: RuntimeLogger;
  private readonly sessions = new Map<string, SessionRecord>();

  /**
   * 创建 CLI adapter。
   *
   * 参数:
   *   - config: 分化配置。
   *
   * 返回值: 无。
   */
  constructor(config: CliAdapterConfig) {
    this.id = config.id;
    this.config = config;
    this.logger =
      config.logger ??
      createRuntimeLogger({
        runtimeId: config.id,
        traceId: config.traceId,
      });
  }

  /**
   * 返回能力声明。
   *
   * 参数: 无。
   * 返回值: RuntimeCapabilities。
   */
  capabilities(): RuntimeCapabilities {
    return this.config.capabilities;
  }

  /**
   * 创建会话（spawn 或注入传输）。
   *
   * 参数:
   *   - opts: 会话选项。
   *
   * 返回值: SessionHandle。
   */
  async createSession(opts: SessionOpts): Promise<SessionHandle> {
    const sessionLocalId = randomUUID();
    const sessionDir = join(
      tmpdir(),
      `agent-gateway-${this.id}-${sessionLocalId}`,
    );
    mkdirSync(sessionDir, { recursive: true });

    const workspaceBinding = opts.workspaceBinding ?? opts.profile === "coding";
    // general 或不绑定 workspace：不要求 cwd
    if (opts.profile === "coding" && workspaceBinding && !opts.cwd) {
      // coding 默认可退到 process.cwd，但不强制失败
    }

    let transport: RpcTransport;
    try {
      if (this.config.transportFactory) {
        transport = await this.config.transportFactory({
          sessionLocalId,
          opts,
          sessionDir,
        });
      } else {
        const spawnSpec = this.config.buildSpawn(opts, sessionDir);
        const pt = new ProcessTransport({
          command: spawnSpec.command,
          args: spawnSpec.args,
          cwd: spawnSpec.cwd ?? opts.cwd,
        });
        pt.start();
        transport = pt;
      }
    } catch (err) {
      throw new AgentRuntimeError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `failed to start runtime ${this.id}`,
        { cause: err },
      );
    }

    const log = withLogFields(this.logger, {
      sessionId: sessionLocalId,
      traceId: this.config.traceId,
    });
    const client = new RpcClient(transport, { logger: log });

    // 就绪：get_state 成功
    let readyData: unknown;
    try {
      const resp = await client.request(
        "get_state",
        {},
        this.config.readyTimeoutMs ?? 30_000,
      );
      readyData = resp.data;
    } catch (err) {
      await client.close();
      throw new AgentRuntimeError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `runtime ${this.id} not ready`,
        { cause: err },
      );
    }

    if (this.config.afterReady) {
      try {
        await this.config.afterReady(client, opts);
      } catch (err) {
        await client.close();
        throw wrapInternalError(err, "afterReady failed");
      }
    }

    // 若 opts.model 与默认不同，尝试 set_model
    if (opts.model && typeof this.setModel === "function") {
      try {
        await client.request("set_model", { model: opts.model });
      } catch (err) {
        // 不静默成功：记录并抛明确错误
        await client.close();
        throw new AgentRuntimeError(
          "AGENT_VALIDATION_FAILED",
          `set_model failed for ${opts.model}`,
          { cause: err, details: { model: opts.model } },
        );
      }
    }

    const runtimeSessionId =
      extractRuntimeSessionId(readyData) ?? sessionLocalId;
    const handle: SessionHandle = {
      sessionId: sessionLocalId,
      runtimeId: this.id,
      sessionFile:
        opts.sessionPersistence === "runtime-file"
          ? join(sessionDir, "session.jsonl")
          : undefined,
      createdAt: new Date().toISOString(),
    };

    const processPid =
      transport instanceof ProcessTransport ? transport.pid() : undefined;

    this.sessions.set(sessionLocalId, {
      handle,
      client,
      transport,
      opts: { ...opts, workspaceBinding },
      allowFileChange: workspaceBinding && opts.profile === "coding",
      disposed: false,
      abortRequested: false,
      processPid,
    });

    log.log("info", "session created", {
      runtimeSessionId,
      profile: opts.profile,
      workspaceBinding,
    });
    return handle;
  }

  /**
   * 发起 prompt，流式产出 RuntimeEvent。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - input: 输入。
   *
   * 返回值: AsyncIterable<RuntimeEvent>。
   */
  async *prompt(
    sessionId: string,
    input: PromptInput,
  ): AsyncIterable<RuntimeEvent> {
    const session = this.requireSession(sessionId);
    const text = resolvePromptText(input);
    if (!text) {
      throw new AgentRuntimeError(
        "AGENT_VALIDATION_FAILED",
        "prompt text is required",
      );
    }

    const mapState = createEventMapperState();
    const ctx = {
      sessionId,
      allowFileChange: session.allowFileChange,
      thinkingEnabled: this.config.capabilities.thinking,
    };

    const queue: RuntimeEvent[] = [];
    let done = false;
    const exitHold: { err: AgentRuntimeError | null } = { err: null };

    const onMsg = (msg: NativeMessage) => {
      const mapped = mapNativeToRuntimeEvents(msg, ctx, mapState);
      for (const ev of mapped) queue.push(ev);
      if (msg.type === "agent_end" || msg.type === "agent_settled") {
        done = true;
      }
    };
    const offMsg = session.client.onMessage(onMsg);
    const offExit = session.client.onExit(({ code, signal }) => {
      exitHold.err = new AgentRuntimeError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `process exited during prompt code=${code} signal=${signal ?? ""}`,
        { details: { code, signal } },
      );
      done = true;
    });

    try {
      await session.client.request("prompt", { message: text });
      // accept ≠ 完成：等到 agent_end / settled / abort / exit
      while (!done) {
        while (queue.length > 0) {
          const ev = queue.shift();
          if (ev) yield ev;
        }
        if (session.abortRequested) {
          // abort 后仍可能有收尾事件；短等后结束流
          await sleep(50);
          if (queue.length === 0) break;
          continue;
        }
        await sleep(10);
      }
      while (queue.length > 0) {
        const ev = queue.shift();
        if (ev) yield ev;
      }
      if (exitHold.err && !session.abortRequested) {
        yield {
          type: "run_error",
          sessionId,
          message: exitHold.err.message,
          code: exitHold.err.code,
        };
      }
    } catch (err) {
      const wrapped = wrapInternalError(err, "prompt failed");
      yield {
        type: "run_error",
        sessionId,
        message: wrapped.message,
        code: wrapped.code,
      };
    } finally {
      offMsg();
      offExit();
      session.abortRequested = false;
    }
  }

  /**
   * 运行中转向。
   *
   * 参数:
   *   - sessionId: 会话。
   *   - input: 输入。
   *
   * 返回值: Promise。
   */
  async steer(sessionId: string, input: PromptInput): Promise<void> {
    if (!this.config.capabilities.steering) {
      throw capabilityUnsupportedError(this.id, "steer");
    }
    const session = this.requireSession(sessionId);
    const text = resolvePromptText(input);
    await session.client.request("steer", { message: text });
  }

  /**
   * follow-up。
   *
   * 参数:
   *   - sessionId: 会话。
   *   - input: 输入。
   *
   * 返回值: Promise。
   */
  async followUp(sessionId: string, input: PromptInput): Promise<void> {
    if (!this.config.capabilities.followUp) {
      throw capabilityUnsupportedError(this.id, "followUp");
    }
    const session = this.requireSession(sessionId);
    const text = resolvePromptText(input);
    await session.client.request("follow_up", { message: text });
  }

  /**
   * 真停当前生成（RPC abort，非仅断监听）。
   *
   * 参数:
   *   - sessionId: 会话。
   *
   * 返回值: Promise。
   */
  async abort(sessionId: string): Promise<void> {
    const session = this.requireSession(sessionId);
    session.abortRequested = true;
    try {
      await session.client.request("abort", {}, 15_000);
    } catch (err) {
      // 进程已死则视为已中断
      if (session.client.isClosed()) return;
      throw wrapInternalError(err, "abort failed");
    }
  }

  /**
   * 读取消毒后的状态。
   *
   * 参数:
   *   - sessionId: 会话。
   *
   * 返回值: RuntimeState。
   */
  async getState(sessionId: string): Promise<RuntimeState> {
    const session = this.requireSession(sessionId);
    const resp = await session.client.request("get_state");
    const data = (resp.data ?? {}) as Record<string, unknown>;
    const safe = sanitizeSecrets(data);
    const model = safe.model ? sanitizeModelInfo(safe.model) : undefined;
    return {
      sessionId,
      isStreaming: Boolean(safe.isStreaming),
      model,
      messageCount:
        typeof safe.messageCount === "number" ? safe.messageCount : undefined,
      extensions: stripKnownStateKeys(safe),
    };
  }

  /**
   * 读取消息历史。
   *
   * 参数:
   *   - sessionId: 会话。
   *   - page: 分页。
   *
   * 返回值: MessagePage。
   */
  async getMessages(
    sessionId: string,
    page?: HistoryPage,
  ): Promise<MessagePage> {
    const session = this.requireSession(sessionId);
    const offset = page?.offset ?? 0;
    const limit = page?.limit ?? 50;
    const resp = await session.client.request("get_messages", {
      offset,
      limit,
    });
    const data = (resp.data ?? {}) as Record<string, unknown>;
    const rawList = Array.isArray(data.messages)
      ? data.messages
      : Array.isArray(data)
        ? data
        : [];
    const messages = rawList.map((m) => normalizeAgentMessage(m));
    const hasMore =
      typeof data.hasMore === "boolean"
        ? data.hasMore
        : messages.length >= limit;
    return {
      messages: sanitizeSecrets(messages),
      offset,
      limit,
      hasMore,
    };
  }

  /**
   * 会话级切换模型。
   *
   * 参数:
   *   - sessionId: 会话。
   *   - model: 模型引用。
   *
   * 返回值: Promise。
   */
  async setModel(sessionId: string, model: ModelRef): Promise<void> {
    if (this.config.capabilities.modelSwitch === "none") {
      throw capabilityUnsupportedError(this.id, "setModel");
    }
    const session = this.requireSession(sessionId);
    try {
      await session.client.request("set_model", {
        model: model.id,
        ...(model.provider ? { provider: model.provider } : {}),
      });
    } catch (err) {
      throw new AgentRuntimeError(
        "AGENT_VALIDATION_FAILED",
        `setModel failed: ${model.id}`,
        { cause: err, details: { model } },
      );
    }
  }

  /**
   * 列出模型（若 runtime 提供）。
   *
   * 参数:
   *   - sessionId: 会话。
   *
   * 返回值: ModelInfo[]。
   */
  async listModels(sessionId: string): Promise<ModelInfo[]> {
    const session = this.requireSession(sessionId);
    try {
      const resp = await session.client.request("get_models", {});
      const data = resp.data;
      const list = Array.isArray(data)
        ? data
        : data &&
            typeof data === "object" &&
            Array.isArray((data as { models?: unknown }).models)
          ? (data as { models: unknown[] }).models
          : [];
      return list.map((m) => sanitizeModelInfo(m));
    } catch (err) {
      // 部分 runtime 无 get_models：明确错误
      throw new AgentRuntimeError(
        "AGENT_CAPABILITY_UNSUPPORTED",
        `listModels unavailable on ${this.id}`,
        { cause: err },
      );
    }
  }

  /**
   * 释放会话与子进程。
   *
   * 参数:
   *   - sessionId: 会话。
   *
   * 返回值: Promise。
   */
  async disposeSession(sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId);
    if (!session || session.disposed) {
      this.sessions.delete(sessionId);
      return;
    }
    session.disposed = true;
    try {
      await session.client.close();
    } finally {
      this.sessions.delete(sessionId);
    }
  }

  /**
   * 测试辅助：取会话 pid。
   *
   * 参数:
   *   - sessionId: 会话。
   *
   * 返回值: pid 或 undefined。
   */
  getSessionPid(sessionId: string): number | undefined {
    return this.sessions.get(sessionId)?.processPid;
  }

  private requireSession(sessionId: string): SessionRecord {
    const session = this.sessions.get(sessionId);
    if (!session || session.disposed) {
      throw sessionNotFoundError(sessionId);
    }
    return session;
  }
}

/**
 * 从 PromptInput 提取文本。
 */
function resolvePromptText(input: PromptInput): string {
  if (typeof input.text === "string" && input.text.length > 0) {
    return input.text;
  }
  if (input.messages?.length) {
    const lastUser = [...input.messages]
      .reverse()
      .find((m) => m.role === "user");
    if (lastUser) {
      return lastUser.content
        .map((b) => (b.type === "text" ? b.text : ""))
        .join("");
    }
  }
  return "";
}

function extractRuntimeSessionId(data: unknown): string | undefined {
  if (!data || typeof data !== "object") return undefined;
  const d = data as Record<string, unknown>;
  return typeof d.sessionId === "string" ? d.sessionId : undefined;
}

function stripKnownStateKeys(
  safe: Record<string, unknown>,
): Record<string, unknown> | undefined {
  const {
    model: _m,
    isStreaming: _s,
    messageCount: _c,
    sessionId: _id,
    ...rest
  } = safe;
  return Object.keys(rest).length > 0 ? rest : undefined;
}

function normalizeAgentMessage(raw: unknown): AgentMessage {
  if (!raw || typeof raw !== "object") {
    return {
      id: randomUUID(),
      role: "assistant",
      content: [],
      createdAt: new Date().toISOString(),
    };
  }
  const m = raw as Record<string, unknown>;
  const role =
    m.role === "system" ||
    m.role === "user" ||
    m.role === "assistant" ||
    m.role === "tool" ||
    m.role === "developer"
      ? m.role
      : "assistant";
  const content = Array.isArray(m.content)
    ? m.content.map((block) => {
        if (!block || typeof block !== "object") {
          return { type: "text" as const, text: String(block) };
        }
        const b = block as Record<string, unknown>;
        if (b.type === "thinking") {
          return {
            type: "thinking" as const,
            text: String(b.thinking ?? b.text ?? ""),
          };
        }
        if (b.type === "tool_use" || b.type === "toolCall") {
          return {
            type: "tool_use" as const,
            toolCallId: String(b.toolCallId ?? b.id ?? ""),
            name: String(b.name ?? b.toolName ?? ""),
            args: b.args ?? b.input,
          };
        }
        return { type: "text" as const, text: String(b.text ?? "") };
      })
    : typeof m.content === "string"
      ? [{ type: "text" as const, text: m.content }]
      : [];
  return {
    id: typeof m.id === "string" ? m.id : randomUUID(),
    role,
    content,
    createdAt:
      typeof m.timestamp === "number"
        ? new Date(m.timestamp).toISOString()
        : typeof m.createdAt === "string"
          ? m.createdAt
          : new Date().toISOString(),
  };
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
