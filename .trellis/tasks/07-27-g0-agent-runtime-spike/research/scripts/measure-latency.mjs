#!/usr/bin/env node
/**
 * 冷启动 / 预热 / 首字节延迟分布测量（≥10 次）。
 * 测量点：
 *   coldStartMs = spawn → get_state 成功（可接受 prompt）
 *   firstByteMs = prompt 发出 → 第一个 agent 事件
 *   turnMs      = prompt 发出 → isStreaming=false
 *
 * 用法：
 *   node measure-latency.mjs --runtime pi --runs 10
 *   node measure-latency.mjs --runtime omp --runs 10
 *   node measure-latency.mjs --runtime both --runs 10
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createRpcDriver } from "./rpc-driver.mjs";

const __dirname = dirname(fileURLToPath(import.meta.url));
const RESEARCH_ROOT = resolve(__dirname, "..");

function parseArgs(argv) {
  const out = { runtime: "both", runs: 10, warm: 5, model: process.env.G0_MODEL || "xh/grok-4.5" };
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--runtime") out.runtime = argv[++i];
    else if (a === "--runs") out.runs = Number(argv[++i]);
    else if (a === "--warm") out.warm = Number(argv[++i]);
    else if (a === "--model") out.model = argv[++i];
  }
  return out;
}

function percentile(sorted, p) {
  if (!sorted.length) return null;
  const idx = Math.min(sorted.length - 1, Math.ceil((p / 100) * sorted.length) - 1);
  return sorted[Math.max(0, idx)];
}

function summarize(arr) {
  const s = [...arr].filter((x) => x != null && !Number.isNaN(x)).sort((a, b) => a - b);
  if (!s.length) return null;
  return {
    n: s.length,
    min: s[0],
    median: percentile(s, 50),
    p95: percentile(s, 95),
    max: s[s.length - 1],
    mean: s.reduce((a, b) => a + b, 0) / s.length,
  };
}

/**
 * 完全冷启动：每次新进程。
 * @param {"pi"|"omp"} runtime
 * @param {number} runs
 * @param {string} model
 */
async function measureCold(runtime, runs, model) {
  const rows = [];
  for (let i = 0; i < runs; i++) {
    const samplePath = join(
      RESEARCH_ROOT,
      "samples",
      runtime,
      `latency-cold-${i}.jsonl`,
    );
    const d = createRpcDriver({
      runtime,
      model,
      thinking: "off",
      noTools: true,
      persist: false,
      samplePath,
    });
    try {
      const ready = await d.start();
      const turn = await d.promptAndWait(
        "Reply with exactly one word: PONG",
        { timeoutMs: 180000 },
      );
      rows.push({
        i,
        coldStartMs: ready.spawnMs,
        firstStdoutMs: ready.firstStdoutMs,
        firstByteMs: turn.firstEventMs,
        turnMs: turn.totalMs,
        sessionId: turn.state?.sessionId,
      });
      console.error(
        `[${runtime}] cold ${i + 1}/${runs} start=${ready.spawnMs.toFixed(0)}ms firstByte=${turn.firstEventMs?.toFixed?.(0)} turn=${turn.totalMs.toFixed(0)}`,
      );
    } catch (e) {
      rows.push({ i, error: String(e), meta: d.getMeta() });
      console.error(`[${runtime}] cold ${i + 1} FAIL`, e.message || e);
    } finally {
      await d.stop();
    }
  }
  return rows;
}

/**
 * 进程已预热：同一进程多次 prompt。
 * @param {"pi"|"omp"} runtime
 * @param {number} runs
 * @param {string} model
 */
async function measureWarm(runtime, runs, model) {
  const samplePath = join(
    RESEARCH_ROOT,
    "samples",
    runtime,
    `latency-warm-session.jsonl`,
  );
  const d = createRpcDriver({
    runtime,
    model,
    thinking: "off",
    noTools: true,
    persist: false,
    samplePath,
  });
  const rows = [];
  try {
    const ready = await d.start();
    rows.push({ kind: "process_start", coldStartMs: ready.spawnMs });
    for (let i = 0; i < runs; i++) {
      try {
        const turn = await d.promptAndWait(
          `Reply with exactly: WARM-${i}`,
          { timeoutMs: 180000 },
        );
        rows.push({
          i,
          firstByteMs: turn.firstEventMs,
          turnMs: turn.totalMs,
        });
        console.error(
          `[${runtime}] warm ${i + 1}/${runs} firstByte=${turn.firstEventMs?.toFixed?.(0)} turn=${turn.totalMs.toFixed(0)}`,
        );
      } catch (e) {
        rows.push({ i, error: String(e) });
        console.error(`[${runtime}] warm ${i + 1} FAIL`, e.message || e);
      }
    }
  } finally {
    await d.stop();
  }
  return rows;
}

async function runOne(runtime, args) {
  console.error(`=== measuring ${runtime} model=${args.model} ===`);
  const cold = await measureCold(runtime, args.runs, args.model);
  const warm = await measureWarm(runtime, args.warm, args.model);
  const coldStarts = cold.map((r) => r.coldStartMs).filter((x) => x != null);
  const coldFirst = cold.map((r) => r.firstByteMs).filter((x) => x != null);
  const coldTurn = cold.map((r) => r.turnMs).filter((x) => x != null);
  const warmFirst = warm
    .filter((r) => r.kind !== "process_start")
    .map((r) => r.firstByteMs)
    .filter((x) => x != null);
  const warmTurn = warm
    .filter((r) => r.kind !== "process_start")
    .map((r) => r.turnMs)
    .filter((x) => x != null);

  const summary = {
    runtime,
    model: args.model,
    measuredAt: new Date().toISOString(),
    host: {
      platform: process.platform,
      arch: process.arch,
      node: process.version,
    },
    cold: {
      rows: cold,
      coldStartMs: summarize(coldStarts),
      firstByteMs: summarize(coldFirst),
      turnMs: summarize(coldTurn),
    },
    warm: {
      rows: warm,
      firstByteMs: summarize(warmFirst),
      turnMs: summarize(warmTurn),
    },
  };

  const outDir = join(RESEARCH_ROOT, "samples", runtime);
  mkdirSync(outDir, { recursive: true });
  const outPath = join(outDir, `latency-summary.json`);
  writeFileSync(outPath, JSON.stringify(summary, null, 2));
  console.log(JSON.stringify(summary, null, 2));
  console.error(`wrote ${outPath}`);
  return summary;
}

const args = parseArgs(process.argv);
const runtimes =
  args.runtime === "both" ? ["pi", "omp"] : [args.runtime];

const all = {};
for (const r of runtimes) {
  all[r] = await runOne(r, args);
}
writeFileSync(
  join(RESEARCH_ROOT, "samples", "latency-all.json"),
  JSON.stringify(all, null, 2),
);
