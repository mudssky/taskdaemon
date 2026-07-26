/**
 * 可注入传输层 + JSONL RPC 客户端。
 * 默认测试不依赖真实进程：用 FakeTransport 或 fixture 回放。
 */

import { EventEmitter } from "node:events";
import { AgentRuntimeError, wrapInternalError } from "./errors.js";
import { createJsonlFramer } from "./jsonl-framer.js";
import type { RuntimeLogger } from "./log.js";
import { createRuntimeLogger } from "./log.js";

/** 原生 RPC/事件消息（Pi/OMP 超集，adapter 内部用）。 */
export type NativeMessage = {
  type?: string;
  id?: string;
  command?: string;
  success?: boolean;
  error?: string;
  code?: string | number;
  data?: unknown;
  [key: string]: unknown;
};

export type RpcTransport = {
  /**
   * 写入一行 JSON（调用方已含 \\n 与否由实现决定；客户端会追加 \\n）。
   *
   * 参数:
   *   - line: 不含换行的 JSON 文本。
   *
   * 返回值:
   *   - Promise<void>。
   */
  write(line: string): Promise<void> | void;
  /**
   * 订阅 stdout 数据。
   *
   * 参数:
   *   - listener: chunk 回调。
   *
   * 返回值:
   *   - 取消订阅函数。
   */
  onData(listener: (chunk: string | Buffer) => void): () => void;
  /**
   * 进程/流退出。
   *
   * 参数:
   *   - listener: exit code 回调。
   *
   * 返回值:
   *   - 取消订阅。
   */
  onExit(
    listener: (code: number | null, signal?: string | null) => void,
  ): () => void;
  /**
   * 关闭传输（杀进程 / 结束 mock）。
   *
   * 参数: 无。
   * 返回值: Promise。
   */
  close(): Promise<void> | void;
  /**
   * 是否仍可写。
   *
   * 参数: 无。
   * 返回值: boolean。
   */
  isWritable(): boolean;
};

export type RpcClientOptions = {
  logger?: RuntimeLogger;
  requestTimeoutMs?: number;
  maxLineChars?: number;
  /** 非法帧是否抛错；默认 false（记录后继续，对齐 G0）。 */
  throwOnInvalidFrame?: boolean;
};

type Pending = {
  command: string;
  resolve: (msg: NativeMessage) => void;
  reject: (err: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};

/**
 * JSONL RPC 客户端。
 */
export class RpcClient {
  private readonly transport: RpcTransport;
  private readonly logger: RuntimeLogger;
  private readonly requestTimeoutMs: number;
  private readonly throwOnInvalidFrame: boolean;
  private readonly pending = new Map<string, Pending>();
  private readonly bus = new EventEmitter();
  private seq = 0;
  private closed = false;
  private exitCode: number | null = null;
  private readonly unsubs: Array<() => void> = [];
  private readonly framer;

  /**
   * 创建 RPC 客户端。
   *
   * 参数:
   *   - transport: 字节传输。
   *   - options: 超时与日志。
   *
   * 返回值:
   *   - 无（构造函数）。
   */
  constructor(transport: RpcTransport, options: RpcClientOptions = {}) {
    this.transport = transport;
    this.logger = options.logger ?? createRuntimeLogger();
    this.requestTimeoutMs = options.requestTimeoutMs ?? 60_000;
    this.throwOnInvalidFrame = options.throwOnInvalidFrame ?? false;
    this.framer = createJsonlFramer(
      {
        onFrame: (value) => this.handleFrame(value),
        onInvalid: (raw, error) => {
          this.logger.log("warn", "jsonl invalid frame", {
            error: error.message,
            sample: raw.slice(0, 120),
          });
          this.bus.emit("invalid", { raw, error });
          if (this.throwOnInvalidFrame) {
            this.bus.emit("error", error);
          }
        },
        onOversize: (length, action) => {
          this.logger.log("warn", "jsonl oversize line", { length, action });
        },
      },
      { maxLineChars: options.maxLineChars },
    );

    this.unsubs.push(
      transport.onData((chunk) => {
        this.framer.push(chunk);
      }),
    );
    this.unsubs.push(
      transport.onExit((code, signal) => {
        this.exitCode = code;
        this.closed = true;
        const err = new AgentRuntimeError(
          "AGENT_RUNTIME_UNAVAILABLE",
          `rpc transport exited code=${code} signal=${signal ?? ""}`,
          { details: { code, signal } },
        );
        for (const [id, p] of this.pending) {
          this.pending.delete(id);
          clearTimeout(p.timer);
          p.reject(err);
        }
        this.bus.emit("exit", { code, signal });
      }),
    );
  }

  /**
   * 订阅原生事件（含 response 与 agent 事件）。
   *
   * 参数:
   *   - listener: 消息回调。
   *
   * 返回值:
   *   - 取消订阅。
   */
  onMessage(listener: (msg: NativeMessage) => void): () => void {
    this.bus.on("message", listener);
    return () => this.bus.off("message", listener);
  }

  /**
   * 订阅进程退出。
   *
   * 参数:
   *   - listener: exit 回调。
   *
   * 返回值:
   *   - 取消订阅。
   */
  onExit(
    listener: (info: { code: number | null; signal?: string | null }) => void,
  ): () => void {
    this.bus.on("exit", listener);
    return () => this.bus.off("exit", listener);
  }

  /**
   * 发送 RPC 并等待 type=response 且 id 匹配。
   *
   * 参数:
   *   - type: 命令名（get_state / prompt / abort / ...）。
   *   - payload: 额外字段。
   *   - timeoutMs: 超时。
   *
   * 返回值:
   *   - 响应消息。
   */
  async request(
    type: string,
    payload: Record<string, unknown> = {},
    timeoutMs = this.requestTimeoutMs,
  ): Promise<NativeMessage> {
    if (this.closed || !this.transport.isWritable()) {
      throw new AgentRuntimeError(
        "AGENT_RUNTIME_UNAVAILABLE",
        "rpc process not running",
      );
    }
    const id = `req-${++this.seq}`;
    const frame = { id, type, ...payload };
    const line = JSON.stringify(frame);
    return new Promise<NativeMessage>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(
          new AgentRuntimeError(
            "AGENT_INTERNAL_ERROR",
            `timeout ${timeoutMs}ms waiting for ${type}`,
            { details: { type, id } },
          ),
        );
      }, timeoutMs);
      this.pending.set(id, {
        command: type,
        resolve,
        reject,
        timer,
      });
      Promise.resolve(this.transport.write(line))
        .then(() => undefined)
        .catch((err) => {
          this.pending.delete(id);
          clearTimeout(timer);
          reject(wrapInternalError(err, `failed to write ${type}`));
        });
    });
  }

  /**
   * 关闭客户端与传输。
   *
   * 参数: 无。
   * 返回值: Promise。
   */
  async close(): Promise<void> {
    if (this.closed) {
      await this.transport.close();
      return;
    }
    this.closed = true;
    for (const [id, p] of this.pending) {
      this.pending.delete(id);
      clearTimeout(p.timer);
      p.reject(
        new AgentRuntimeError("AGENT_RUNTIME_UNAVAILABLE", "rpc client closed"),
      );
    }
    for (const off of this.unsubs) off();
    this.framer.flush();
    await this.transport.close();
  }

  /**
   * 最近退出码。
   *
   * 参数: 无。
   * 返回值: code 或 null。
   */
  getExitCode(): number | null {
    return this.exitCode;
  }

  /**
   * 是否已关闭。
   *
   * 参数: 无。
   * 返回值: boolean。
   */
  isClosed(): boolean {
    return this.closed;
  }

  private handleFrame(value: unknown): void {
    if (!value || typeof value !== "object") {
      this.logger.log("warn", "jsonl non-object frame");
      return;
    }
    const msg = value as NativeMessage;
    this.bus.emit("message", msg);
    if (msg.type === "response" && typeof msg.id === "string") {
      const p = this.pending.get(msg.id);
      if (!p) return;
      this.pending.delete(msg.id);
      clearTimeout(p.timer);
      if (msg.success === false) {
        p.reject(
          new AgentRuntimeError(
            "AGENT_INTERNAL_ERROR",
            msg.error || `RPC ${p.command} failed`,
            {
              details: {
                command: p.command,
                code: msg.code,
              },
            },
          ),
        );
        return;
      }
      p.resolve(msg);
    }
  }
}

/**
 * 内存假传输：测试半包/粘包/退出，不 spawn 进程。
 */
export class FakeTransport implements RpcTransport {
  private readonly dataListeners = new Set<(c: string | Buffer) => void>();
  private readonly exitListeners = new Set<
    (code: number | null, signal?: string | null) => void
  >();
  private writable = true;
  /** 收到的请求行（无 \\n）。 */
  readonly writes: string[] = [];
  private autoHandler?: (req: NativeMessage) => NativeMessage[] | undefined;

  /**
   * 设置自动应答器（按请求生成响应/事件）。
   *
   * 参数:
   *   - handler: 请求 → 出站消息列表。
   *
   * 返回值: 无。
   */
  setAutoHandler(
    handler: (req: NativeMessage) => NativeMessage[] | undefined,
  ): void {
    this.autoHandler = handler;
  }

  /**
   * 向客户端推送原始 stdout 字节（可半包）。
   *
   * 参数:
   *   - chunk: 数据。
   *
   * 返回值: 无。
   */
  push(chunk: string | Buffer): void {
    for (const l of this.dataListeners) l(chunk);
  }

  /**
   * 推送完整 JSON 对象帧。
   *
   * 参数:
   *   - msg: 对象。
   *
   * 返回值: 无。
   */
  pushMessage(msg: unknown): void {
    this.push(`${JSON.stringify(msg)}\n`);
  }

  /**
   * 模拟进程退出。
   *
   * 参数:
   *   - code: 退出码。
   *   - signal: 信号。
   *
   * 返回值: 无。
   */
  exit(code: number | null = 0, signal?: string | null): void {
    this.writable = false;
    for (const l of this.exitListeners) l(code, signal);
  }

  write(line: string): void {
    if (!this.writable) throw new Error("transport not writable");
    this.writes.push(line);
    if (!this.autoHandler) return;
    let req: NativeMessage;
    try {
      req = JSON.parse(line) as NativeMessage;
    } catch {
      return;
    }
    const outs = this.autoHandler(req);
    if (!outs) return;
    for (const out of outs) this.pushMessage(out);
  }

  onData(listener: (chunk: string | Buffer) => void): () => void {
    this.dataListeners.add(listener);
    return () => this.dataListeners.delete(listener);
  }

  onExit(
    listener: (code: number | null, signal?: string | null) => void,
  ): () => void {
    this.exitListeners.add(listener);
    return () => this.exitListeners.delete(listener);
  }

  async close(): Promise<void> {
    this.writable = false;
  }

  isWritable(): boolean {
    return this.writable;
  }
}
