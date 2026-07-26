#!/usr/bin/env node
/**
 * 最小 JSONL RPC 驱动：spawn pi/omp --mode rpc，按 \n 分帧读写。
 * 用法见同目录其他脚本；也可直接：
 *   node rpc-driver.mjs pi get_state
 *   node rpc-driver.mjs omp prompt "hello"
 *
 * @param {string} runtime - "pi" | "omp"
 * @returns {Promise<void>}
 */
import { spawn } from "node:child_process";
import { createWriteStream, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";

const __dirname = dirname(fileURLToPath(import.meta.url));
const RESEARCH_ROOT = resolve(__dirname, "..");
const DEFAULT_MODEL = process.env.G0_MODEL || "xh/grok-4.5";
const DEFAULT_THINKING = process.env.G0_THINKING || "off";

/**
 * @typedef {object} RpcDriverOptions
 * @property {"pi"|"omp"} runtime
 * @property {string} [model]
 * @property {string} [thinking]
 * @property {string} [cwd]
 * @property {string} [sessionDir]
 * @property {boolean} [noSession]
 * @property {boolean} [noTools]
 * @property {string[]} [extraArgs]
 * @property {Record<string,string>} [env]
 * @property {string} [samplePath]  原始事件流 JSONL 输出路径
 * @property {boolean} [inheritStderr]
 */

/**
 * @param {RpcDriverOptions} opts
 */
export function createRpcDriver(opts) {
  const runtime = opts.runtime;
  const bin = runtime === "omp" ? "omp" : "pi";
  const model = opts.model || DEFAULT_MODEL;
  const thinking = opts.thinking || DEFAULT_THINKING;
  const cwd = opts.cwd || process.cwd();
  const sessionDir =
    opts.sessionDir ||
    join(tmpdir(), `g0-${runtime}-session-${randomUUID()}`);
  mkdirSync(sessionDir, { recursive: true });

  const args = [
    "--mode",
    "rpc",
    "--model",
    model,
    "--thinking",
    thinking,
    "--session-dir",
    sessionDir,
  ];
  if (opts.noSession !== false) {
    // 默认 ephemeral，除非显式要持久化
    if (opts.noSession === true || opts.noSession === undefined) {
      if (opts.persist !== true) args.push("--no-session");
    }
  }
  if (opts.persist === true) {
    // 去掉 --no-session
    const i = args.indexOf("--no-session");
    if (i >= 0) args.splice(i, 1);
  }
  if (opts.noTools !== false) {
    args.push("--no-tools");
  }
  if (runtime === "omp") {
    args.push("--cwd", cwd);
    // 降低扩展噪声：OMP 可能加载大量扩展
    if (opts.minimalProfile !== false) {
      // 用隔离 profile 避免污染默认 agent
      // args.push("--profile", `g0-spike-${randomUUID().slice(0, 8)}`);
    }
  }
  if (opts.extraArgs?.length) args.push(...opts.extraArgs);

  /** @type {import('node:child_process').ChildProcessWithoutNullStreams | null} */
  let child = null;
  let buffer = "";
  let seq = 0;
  /** @type {Map<string, {resolve:Function, reject:Function, command:string, t0:number}>} */
  const pending = new Map();
  /** @type {Array<object>} */
  const events = [];
  /** @type {((ev: object) => void)[]} */
  const listeners = [];
  let sampleStream = null;
  let ready = false;
  let exitCode = null;
  let exitError = null;
  let stderrBuf = "";
  let firstStdoutAt = null;
  let spawnAt = null;
  let firstEventAt = null;

  if (opts.samplePath) {
    mkdirSync(dirname(opts.samplePath), { recursive: true });
    sampleStream = createWriteStream(opts.samplePath, { flags: "a" });
  }

  function logSample(kind, obj) {
    const row = { t: Date.now(), kind, ...obj };
    events.push(row);
    if (sampleStream) sampleStream.write(JSON.stringify(row) + "\n");
  }

  function handleLine(line) {
    if (!line.trim()) return;
    if (firstStdoutAt == null) firstStdoutAt = performance.now();
    let msg;
    try {
      msg = JSON.parse(line);
    } catch (e) {
      logSample("parse_error", { line, error: String(e) });
      return;
    }
    logSample("stdout", { msg });
    if (firstEventAt == null) firstEventAt = performance.now();
    for (const l of listeners) l(msg);

    if (msg?.type === "response") {
      const id = msg.id;
      if (id && pending.has(id)) {
        const p = pending.get(id);
        pending.delete(id);
        if (msg.success === false) {
          p.reject(
            Object.assign(new Error(msg.error || `RPC ${p.command} failed`), {
              code: msg.code,
              response: msg,
            }),
          );
        } else {
          p.resolve({ ...msg, _rttMs: performance.now() - p.t0 });
        }
      }
    }
  }

  function onData(chunk) {
    buffer += chunk.toString("utf8");
    // 严格 \n 分帧（协议要求）
    while (true) {
      const idx = buffer.indexOf("\n");
      if (idx < 0) break;
      let line = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 1);
      if (line.endsWith("\r")) line = line.slice(0, -1);
      handleLine(line);
    }
  }

  /**
   * 启动子进程。
   * @returns {Promise<{spawnMs:number, sessionDir:string, args:string[]}>}
   */
  async function start() {
    spawnAt = performance.now();
    child = spawn(bin, args, {
      cwd,
      env: { ...process.env, ...(opts.env || {}) },
      stdio: ["pipe", "pipe", opts.inheritStderr ? "inherit" : "pipe"],
    });
    child.stdout.on("data", onData);
    if (child.stderr && !opts.inheritStderr) {
      child.stderr.on("data", (c) => {
        stderrBuf += c.toString("utf8");
        logSample("stderr", { chunk: c.toString("utf8") });
      });
    }
    child.on("error", (err) => {
      exitError = err;
    });
    child.on("exit", (code) => {
      exitCode = code;
      for (const [id, p] of pending) {
        pending.delete(id);
        p.reject(new Error(`process exited ${code} while waiting ${id}`));
      }
    });

    // 探测就绪：get_state 成功即认为可接受 prompt
    const state = await request("get_state", {}, 30000);
    ready = true;
    const readyMs = performance.now() - spawnAt;
    return {
      spawnMs: readyMs,
      firstStdoutMs:
        firstStdoutAt != null && spawnAt != null
          ? firstStdoutAt - spawnAt
          : null,
      sessionDir,
      args: [bin, ...args],
      state: state.data,
    };
  }

  /**
   * 发送 RPC 命令并等待 response。
   * @param {string} type
   * @param {object} [payload]
   * @param {number} [timeoutMs]
   */
  function request(type, payload = {}, timeoutMs = 60000) {
    if (!child || !child.stdin.writable) {
      return Promise.reject(new Error("RPC process not running"));
    }
    const id = `req-${++seq}`;
    const frame = { id, type, ...payload };
    const line = JSON.stringify(frame) + "\n";
    logSample("stdin", { frame });
    const t0 = performance.now();
    return new Promise((resolveP, rejectP) => {
      const timer = setTimeout(() => {
        pending.delete(id);
        rejectP(new Error(`timeout ${timeoutMs}ms waiting for ${type}`));
      }, timeoutMs);
      pending.set(id, {
        command: type,
        t0,
        resolve: (v) => {
          clearTimeout(timer);
          resolveP(v);
        },
        reject: (e) => {
          clearTimeout(timer);
          rejectP(e);
        },
      });
      child.stdin.write(line);
    });
  }

  /**
   * 发 prompt 并等到 agent 空闲（isStreaming=false 且有 agent_end 或响应后轮询）。
   * @param {string} message
   * @param {object} [opts2]
   */
  async function promptAndWait(message, opts2 = {}) {
    const t0 = performance.now();
    let firstDeltaAt = null;
    const unsub = onEvent((msg) => {
      if (firstDeltaAt != null) return;
      // 常见首字节：message_update / agent_start / text
      const t = msg?.type || msg?.event?.type;
      if (
        t === "message_update" ||
        t === "agent_start" ||
        t === "turn_start" ||
        t === "text_delta" ||
        msg?.assistantMessageEvent?.type === "text_delta"
      ) {
        firstDeltaAt = performance.now();
      }
      // OMP / Pi 事件可能包在 session event 里
      if (msg?.type && msg.type !== "response") {
        if (firstDeltaAt == null && msg.type !== "response") {
          // 任意非 response 事件都算首事件
          if (
            msg.type === "agent_start" ||
            msg.type === "message_start" ||
            msg.type === "message_update" ||
            msg.type === "turn_start"
          ) {
            firstDeltaAt = performance.now();
          }
        }
      }
    });

    await request("prompt", { message, ...opts2.promptExtra }, opts2.timeoutMs || 120000);

    // 等到 isStreaming false
    const deadline = performance.now() + (opts2.timeoutMs || 120000);
    while (performance.now() < deadline) {
      const st = await request("get_state", {}, 15000);
      if (!st.data?.isStreaming) {
        unsub();
        const t1 = performance.now();
        return {
          totalMs: t1 - t0,
          firstEventMs: firstDeltaAt != null ? firstDeltaAt - t0 : null,
          state: st.data,
        };
      }
      await sleep(50);
    }
    unsub();
    throw new Error("promptAndWait timeout");
  }

  function onEvent(fn) {
    listeners.push(fn);
    return () => {
      const i = listeners.indexOf(fn);
      if (i >= 0) listeners.splice(i, 1);
    };
  }

  async function stop() {
    if (!child) return;
    try {
      child.stdin.end();
    } catch {}
    // 给一点优雅退出时间
    await sleep(100);
    if (exitCode == null) {
      child.kill("SIGTERM");
      await sleep(200);
      if (exitCode == null) child.kill("SIGKILL");
    }
    if (sampleStream) {
      await new Promise((r) => sampleStream.end(r));
    }
  }

  function getMeta() {
    return {
      runtime,
      model,
      thinking,
      sessionDir,
      args: [bin, ...args],
      exitCode,
      exitError: exitError ? String(exitError) : null,
      stderr: stderrBuf.slice(-8000),
      eventCount: events.length,
    };
  }

  function dumpEvents(path) {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, events.map((e) => JSON.stringify(e)).join("\n") + "\n");
  }

  /** 半包写入：把一帧拆成两次 write */
  function writeRaw(chunks, delayMs = 0) {
    return new Promise(async (resolveP, rejectP) => {
      try {
        for (const c of chunks) {
          child.stdin.write(c);
          if (delayMs) await sleep(delayMs);
        }
        resolveP();
      } catch (e) {
        rejectP(e);
      }
    });
  }

  return {
    start,
    request,
    promptAndWait,
    onEvent,
    stop,
    getMeta,
    dumpEvents,
    writeRaw,
    get events() {
      return events;
    },
    get child() {
      return child;
    },
  };
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

// CLI 入口
const isMain =
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url);

if (isMain) {
  const runtime = process.argv[2] || "pi";
  const cmd = process.argv[3] || "get_state";
  const msg = process.argv[4] || "Reply with exactly: PONG";
  const d = createRpcDriver({
    runtime,
    samplePath: join(
      RESEARCH_ROOT,
      "samples",
      runtime,
      `cli-${cmd}-${Date.now()}.jsonl`,
    ),
  });
  try {
    const ready = await d.start();
    console.log(JSON.stringify({ phase: "ready", ready }, null, 2));
    if (cmd === "prompt") {
      const r = await d.promptAndWait(msg);
      console.log(JSON.stringify({ phase: "prompt", r }, null, 2));
    } else if (cmd !== "get_state") {
      const r = await d.request(cmd, msg ? { message: msg } : {});
      console.log(JSON.stringify({ phase: "cmd", r }, null, 2));
    }
  } catch (e) {
    console.error("FAIL", e);
    console.error(d.getMeta());
    process.exitCode = 1;
  } finally {
    await d.stop();
  }
}
