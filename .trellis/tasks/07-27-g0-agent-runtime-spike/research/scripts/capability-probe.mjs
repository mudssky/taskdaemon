#!/usr/bin/env node
/**
 * 能力矩阵实验：对 pi/omp 各跑一套最小可观察实验，事件流写入 samples/。
 *
 * 实验：
 *  - basic_prompt
 *  - session_persist (起→发→杀→重启→恢复)
 *  - tool_visibility (允许 bash，触发一次)
 *  - thinking_events
 *  - abort_semantics
 *  - model_switch
 *  - non_coding (temp cwd + no tools)
 *  - error_bad_model
 *  - multi_session (两进程并发)
 *  - get_messages / history
 */
import { createRpcDriver } from "./rpc-driver.mjs";
import { mkdirSync, writeFileSync, readdirSync, existsSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import { spawn } from "node:child_process";

const __dirname = dirname(fileURLToPath(import.meta.url));
const RESEARCH_ROOT = resolve(__dirname, "..");
const MODEL = process.env.G0_MODEL || "xh/grok-4.5";

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

function sample(runtime, name) {
  return join(RESEARCH_ROOT, "samples", runtime, `${name}.jsonl`);
}

/**
 * @param {"pi"|"omp"} runtime
 * @param {string} name
 * @param {(ctx: any) => Promise<object>} fn
 */
async function runCase(runtime, name, fn) {
  console.error(`\n[${runtime}] >>> ${name}`);
  const t0 = Date.now();
  try {
    const result = await fn();
    const out = {
      runtime,
      case: name,
      ok: true,
      ms: Date.now() - t0,
      model: MODEL,
      result,
    };
    console.error(`[${runtime}] OK ${name} (${out.ms}ms)`);
    return out;
  } catch (e) {
    const out = {
      runtime,
      case: name,
      ok: false,
      ms: Date.now() - t0,
      model: MODEL,
      error: String(e),
      stack: e?.stack,
    };
    console.error(`[${runtime}] FAIL ${name}:`, e.message || e);
    return out;
  }
}

async function basicPrompt(runtime) {
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: true,
    samplePath: sample(runtime, "basic_prompt"),
  });
  try {
    const ready = await d.start();
    const turn = await d.promptAndWait("Reply with exactly: PONG");
    const lastText = await d.request("get_last_assistant_text", {}).catch(() => null);
    const state = await d.request("get_state");
    return {
      ready,
      turn,
      lastText: lastText?.data ?? lastText,
      messageCount: state.data?.messageCount,
      eventTypes: [...new Set(d.events.filter((e) => e.kind === "stdout").map((e) => e.msg?.type))],
    };
  } finally {
    await d.stop();
  }
}

async function sessionPersist(runtime) {
  const sessionDir = join(tmpdir(), `g0-persist-${runtime}-${randomUUID()}`);
  mkdirSync(sessionDir, { recursive: true });
  let sessionId = null;
  let sessionFile = null;

  // phase 1
  {
    const d = createRpcDriver({
      runtime,
      model: MODEL,
      thinking: "off",
      noTools: true,
      persist: true,
      sessionDir,
      samplePath: sample(runtime, "session_persist_phase1"),
    });
    // createRpcDriver always may add --no-session; force persist via extra rebuild
    // We need to fix: pass persist true
    try {
      const ready = await d.start();
      sessionId = ready.state?.sessionId;
      sessionFile = ready.state?.sessionFile;
      await d.promptAndWait("Remember the secret codeword: BLUEBERRY. Reply OK.");
      const st = await d.request("get_state");
      sessionId = st.data?.sessionId;
      sessionFile = st.data?.sessionFile;
    } finally {
      await d.stop();
    }
  }

  const filesAfter = existsSync(sessionDir) ? readdirSync(sessionDir) : [];

  // phase 2: resume
  let resumed = null;
  {
    const extra =
      sessionFile != null
        ? ["--session", sessionFile]
        : sessionId
          ? ["--session-id", sessionId]
          : [];
    // OMP/Pi --session 可能接受 path
    const d = createRpcDriver({
      runtime,
      model: MODEL,
      thinking: "off",
      noTools: true,
      persist: true,
      sessionDir,
      extraArgs: extra,
      samplePath: sample(runtime, "session_persist_phase2"),
    });
    try {
      const ready = await d.start();
      const st = await d.request("get_state");
      let messages = null;
      try {
        messages = await d.request("get_messages", {});
      } catch (e) {
        messages = { error: String(e) };
      }
      // 问秘密词
      const turn = await d.promptAndWait(
        "What secret codeword did I tell you earlier? Reply with just the word or NONE.",
      );
      let lastText = null;
      try {
        lastText = await d.request("get_last_assistant_text", {});
      } catch {}
      resumed = {
        readyState: ready.state,
        state: st.data,
        messages: messages?.data ?? messages,
        turn,
        lastText: lastText?.data ?? lastText,
      };
    } finally {
      await d.stop();
    }
  }

  return {
    sessionDir,
    sessionId,
    sessionFile,
    filesAfterKill: filesAfter,
    resumed,
  };
}

async function toolVisibility(runtime) {
  // 允许 bash 工具
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: false,
    extraArgs: ["--tools", "bash"],
    samplePath: sample(runtime, "tool_visibility"),
  });
  try {
    await d.start();
    const turn = await d.promptAndWait(
      "Run this exact bash command and then report its stdout: echo G0_TOOL_OK. Use the bash tool. Do not invent output.",
      { timeoutMs: 180000 },
    );
    const toolRelated = d.events
      .filter((e) => e.kind === "stdout")
      .map((e) => e.msg)
      .filter((m) => {
        const t = JSON.stringify(m || {});
        return /tool|bash|function_call|tool_call/i.test(t);
      });
    return {
      turn,
      toolEventCount: toolRelated.length,
      toolEventTypes: [
        ...new Set(toolRelated.map((m) => m?.type || m?.event?.type || "unknown")),
      ],
      sampleToolEvents: toolRelated.slice(0, 8),
    };
  } finally {
    await d.stop();
  }
}

async function thinkingEvents(runtime) {
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "high",
    noTools: true,
    samplePath: sample(runtime, "thinking_events"),
  });
  try {
    await d.start();
    const turn = await d.promptAndWait(
      "Think step by step: what is 17*19? Show brief reasoning then the number.",
      { timeoutMs: 180000 },
    );
    const thinkingHits = d.events
      .filter((e) => e.kind === "stdout")
      .map((e) => e.msg)
      .filter((m) => /thinking|reasoning|thought/i.test(JSON.stringify(m || {})));
    return {
      turn,
      thinkingEventCount: thinkingHits.length,
      sample: thinkingHits.slice(0, 5),
      allTypes: [
        ...new Set(
          d.events.filter((e) => e.kind === "stdout").map((e) => e.msg?.type),
        ),
      ],
    };
  } finally {
    await d.stop();
  }
}

async function abortSemantics(runtime) {
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: true,
    samplePath: sample(runtime, "abort_semantics"),
  });
  try {
    await d.start();
    // 不 await prompt 完成
    const promptResp = d.request(
      "prompt",
      {
        message:
          "Write a very long essay about the history of tea, at least 2000 words. Keep going.",
      },
      120000,
    );
    await sleep(800);
    const st1 = await d.request("get_state");
    const abortResp = await d.request("abort", {});
    await sleep(500);
    const st2 = await d.request("get_state");
    // 再发一个短 prompt，看会话是否可继续
    let cont = null;
    try {
      cont = await d.promptAndWait("Reply with exactly: AFTER_ABORT", {
        timeoutMs: 120000,
      });
    } catch (e) {
      cont = { error: String(e) };
    }
    let promptResult = null;
    try {
      promptResult = await promptResp;
    } catch (e) {
      promptResult = { error: String(e) };
    }
    return {
      streamingBeforeAbort: st1.data?.isStreaming,
      abortResp,
      streamingAfterAbort: st2.data?.isStreaming,
      continueAfterAbort: cont,
      originalPromptResult: promptResult,
    };
  } finally {
    await d.stop();
  }
}

async function modelSwitch(runtime) {
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: true,
    samplePath: sample(runtime, "model_switch"),
  });
  try {
    await d.start();
    await d.promptAndWait("Say HI");
    const before = await d.request("get_state");
    const models = await d.request("get_available_models");
    // 尝试切到同一 provider 的另一个模型（若有）
    const list = models.data || models;
    let target = null;
    if (Array.isArray(list)) {
      target = list.find(
        (m) => m.provider === "xh" && m.id && m.id !== "grok-4.5",
      );
    }
    let switchResult = null;
    if (target) {
      switchResult = await d.request("set_model", {
        provider: target.provider,
        modelId: target.id,
      });
    } else {
      // 切回同一模型也验证命令
      switchResult = await d
        .request("set_model", { provider: "xh", modelId: "grok-4.5" })
        .catch((e) => ({ error: String(e) }));
    }
    const after = await d.request("get_state");
    // 历史是否还在
    const cont = await d.promptAndWait(
      "What did I ask you to say in the previous message? One word.",
    );
    return {
      beforeModel: before.data?.model,
      availableCount: Array.isArray(list) ? list.length : null,
      target,
      switchResult,
      afterModel: after.data?.model,
      messageCountAfter: after.data?.messageCount,
      cont,
    };
  } finally {
    await d.stop();
  }
}

async function nonCoding(runtime) {
  const cwd = join(tmpdir(), `g0-noncoding-${randomUUID()}`);
  mkdirSync(cwd, { recursive: true });
  // 不给仓库 workspace；no tools
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: true,
    cwd,
    samplePath: sample(runtime, "non_coding"),
    extraArgs:
      runtime === "omp"
        ? ["--no-context-files"]
        : ["--no-context-files", "--no-skills", "--no-extensions"],
  });
  try {
    const ready = await d.start();
    const turn = await d.promptAndWait(
      "You are a general assistant with no filesystem. Answer: capital of France? One word.",
    );
    return { cwd, ready, turn };
  } finally {
    await d.stop();
  }
}

async function errorBadModel(runtime) {
  const d = createRpcDriver({
    runtime,
    model: "xh/this-model-does-not-exist-g0",
    thinking: "off",
    noTools: true,
    samplePath: sample(runtime, "error_bad_model"),
  });
  try {
    let startErr = null;
    let ready = null;
    try {
      ready = await d.start();
    } catch (e) {
      startErr = String(e);
    }
    let promptErr = null;
    let turn = null;
    if (!startErr) {
      try {
        // set_model 到坏模型
        await d
          .request("set_model", {
            provider: "xh",
            modelId: "definitely-not-a-real-model-xyz",
          })
          .catch((e) => ({ setModelError: String(e) }));
        turn = await d.promptAndWait("hi", { timeoutMs: 60000 });
      } catch (e) {
        promptErr = String(e);
      }
    }
    const errEvents = d.events
      .filter((e) => e.kind === "stdout")
      .map((e) => e.msg)
      .filter((m) => /error|fail/i.test(JSON.stringify(m || {})))
      .slice(0, 10);
    return { startErr, ready, promptErr, turn, errEvents, stderr: d.getMeta().stderr?.slice(-1000) };
  } finally {
    await d.stop();
  }
}

async function multiSession(runtime) {
  // 两进程并发
  const mk = (tag) =>
    createRpcDriver({
      runtime,
      model: MODEL,
      thinking: "off",
      noTools: true,
      samplePath: sample(runtime, `multi_session_${tag}`),
    });
  const a = mk("a");
  const b = mk("b");
  try {
    const [ra, rb] = await Promise.all([a.start(), b.start()]);
    const [ta, tb] = await Promise.all([
      a.promptAndWait("Reply with exactly: A"),
      b.promptAndWait("Reply with exactly: B"),
    ]);
    return {
      processModel: "N_processes",
      sessionA: ra.state?.sessionId,
      sessionB: rb.state?.sessionId,
      turnA: ta,
      turnB: tb,
      note: "RPC CLI 路径：一会话一进程；未测 SDK 进程内多会话",
    };
  } finally {
    await Promise.all([a.stop(), b.stop()]);
  }
}

async function messageHistory(runtime) {
  const d = createRpcDriver({
    runtime,
    model: MODEL,
    thinking: "off",
    noTools: true,
    samplePath: sample(runtime, "message_history"),
  });
  try {
    await d.start();
    await d.promptAndWait("Say ONE");
    await d.promptAndWait("Say TWO");
    let messages = null;
    let entries = null;
    try {
      messages = await d.request("get_messages", {});
    } catch (e) {
      messages = { error: String(e) };
    }
    try {
      entries = await d.request("get_entries", {});
    } catch (e) {
      entries = { error: String(e) };
    }
    const st = await d.request("get_state");
    return {
      messageCount: st.data?.messageCount,
      messages: messages?.data ?? messages,
      entries: entries?.data ?? entries,
    };
  } finally {
    await d.stop();
  }
}

async function runRuntime(runtime) {
  const cases = [
    ["basic_prompt", () => basicPrompt(runtime)],
    ["session_persist", () => sessionPersist(runtime)],
    ["tool_visibility", () => toolVisibility(runtime)],
    ["thinking_events", () => thinkingEvents(runtime)],
    ["abort_semantics", () => abortSemantics(runtime)],
    ["model_switch", () => modelSwitch(runtime)],
    ["non_coding", () => nonCoding(runtime)],
    ["error_bad_model", () => errorBadModel(runtime)],
    ["multi_session", () => multiSession(runtime)],
    ["message_history", () => messageHistory(runtime)],
  ];
  const results = [];
  for (const [name, fn] of cases) {
    results.push(await runCase(runtime, name, fn));
  }
  const outPath = join(RESEARCH_ROOT, "samples", runtime, "capability-results.json");
  writeFileSync(outPath, JSON.stringify(results, null, 2));
  console.error(`wrote ${outPath}`);
  return results;
}

const which = process.argv[2] || "both";
const all = {};
if (which === "both" || which === "pi") all.pi = await runRuntime("pi");
if (which === "both" || which === "omp") all.omp = await runRuntime("omp");
writeFileSync(
  join(RESEARCH_ROOT, "samples", "capability-all.json"),
  JSON.stringify(all, null, 2),
);
console.log(JSON.stringify(all, null, 2));
