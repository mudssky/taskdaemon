/**
 * 真实 CLI 子进程传输（pi/omp --mode rpc）。
 */

import { type ChildProcessWithoutNullStreams, spawn } from "node:child_process";
import type { RpcTransport } from "./rpc-client.js";

export type ProcessTransportOptions = {
  command: string;
  args: string[];
  cwd?: string;
  env?: NodeJS.ProcessEnv;
};

/**
 * 基于 child_process 的 RpcTransport。
 */
export class ProcessTransport implements RpcTransport {
  private child: ChildProcessWithoutNullStreams | null = null;
  private readonly opts: ProcessTransportOptions;
  private writable = false;
  private readonly dataListeners = new Set<(c: string | Buffer) => void>();
  private readonly exitListeners = new Set<
    (code: number | null, signal?: string | null) => void
  >();
  private started = false;

  /**
   * 创建传输（尚未 spawn）。
   *
   * 参数:
   *   - opts: 命令与参数。
   *
   * 返回值: 无。
   */
  constructor(opts: ProcessTransportOptions) {
    this.opts = opts;
  }

  /**
   * 启动子进程。
   *
   * 参数: 无。
   * 返回值: Promise。
   */
  start(): void {
    if (this.started) return;
    this.started = true;
    const child = spawn(this.opts.command, this.opts.args, {
      cwd: this.opts.cwd,
      env: { ...process.env, ...this.opts.env },
      stdio: ["pipe", "pipe", "pipe"],
    });
    this.child = child;
    this.writable = true;
    child.stdout.on("data", (chunk: Buffer) => {
      for (const l of this.dataListeners) l(chunk);
    });
    child.stderr?.on("data", () => {
      // stderr 可能含 TUI 转义；不解析，不回显密钥
    });
    child.on("error", () => {
      this.writable = false;
    });
    child.on("exit", (code, signal) => {
      this.writable = false;
      for (const l of this.exitListeners) l(code, signal);
    });
  }

  write(line: string): void {
    if (!this.child || !this.writable || !this.child.stdin.writable) {
      throw new Error("process transport not writable");
    }
    this.child.stdin.write(`${line}\n`);
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
    const child = this.child;
    this.child = null;
    if (!child || child.killed) return;
    await new Promise<void>((resolve) => {
      const done = () => resolve();
      child.once("exit", done);
      // 先优雅，再强杀
      try {
        child.stdin.end();
      } catch {
        /* ignore */
      }
      child.kill("SIGTERM");
      setTimeout(() => {
        if (!child.killed) {
          try {
            child.kill("SIGKILL");
          } catch {
            /* ignore */
          }
        }
        resolve();
      }, 2000).unref?.();
    });
  }

  isWritable(): boolean {
    return this.writable;
  }

  /**
   * 底层 pid（测试资源释放用）。
   *
   * 参数: 无。
   * 返回值: pid 或 undefined。
   */
  pid(): number | undefined {
    return this.child?.pid;
  }
}
