#!/usr/bin/env node
/**
 * CLI RPC 协议异常实测：半包、粘包、超长行、非法 JSON、进程中途退出。
 * 只测协议层，不依赖模型返回（多数用例在 get_state 后直接写脏数据）。
 */
import { createRpcDriver } from "./rpc-driver.mjs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { writeFileSync, mkdirSync } from "node:fs";
import { performance } from "node:perf_hooks";

const __dirname = dirname(fileURLToPath(import.meta.url));
const RESEARCH_ROOT = resolve(__dirname, "..");

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

/**
 * @param {"pi"|"omp"} runtime
 */
async function runAnomalies(runtime) {
  const results = [];
  const samplePath = join(
    RESEARCH_ROOT,
    "samples",
    runtime,
    "protocol-anomalies.jsonl",
  );

  // --- 1 半包 ---
  {
    const d = createRpcDriver({
      runtime,
      thinking: "off",
      noTools: true,
      samplePath,
    });
    const caseName = "half_frame";
    try {
      await d.start();
      const id = "half-1";
      const full = JSON.stringify({ id, type: "get_state" }) + "\n";
      const mid = Math.floor(full.length / 2);
      await d.writeRaw([full.slice(0, mid)]);
      await sleep(200);
      // 此时应尚无 response
      const before = d.events.filter(
        (e) => e.kind === "stdout" && e.msg?.id === id,
      ).length;
      await d.writeRaw([full.slice(mid)]);
      // 等待 response
      const t0 = performance.now();
      let ok = false;
      while (performance.now() - t0 < 5000) {
        const hit = d.events.find(
          (e) =>
            e.kind === "stdout" &&
            e.msg?.type === "response" &&
            e.msg?.id === id,
        );
        if (hit) {
          ok = true;
          break;
        }
        await sleep(50);
      }
      results.push({
        case: caseName,
        observed: {
          responsesBeforeSecondWrite: before,
          gotResponseAfterComplete: ok,
        },
        pass: before === 0 && ok,
        note: "半包在补齐 \\n 前不应解析；补齐后应正常 response",
      });
    } catch (e) {
      results.push({ case: caseName, error: String(e) });
    } finally {
      await d.stop();
    }
  }

  // --- 2 粘包 ---
  {
    const d = createRpcDriver({
      runtime,
      thinking: "off",
      noTools: true,
      samplePath,
    });
    const caseName = "glued_frames";
    try {
      await d.start();
      const a = JSON.stringify({ id: "glue-a", type: "get_state" });
      const b = JSON.stringify({ id: "glue-b", type: "get_state" });
      await d.writeRaw([a + "\n" + b + "\n"]);
      const t0 = performance.now();
      let gotA = false;
      let gotB = false;
      while (performance.now() - t0 < 5000) {
        for (const e of d.events) {
          if (e.kind !== "stdout" || e.msg?.type !== "response") continue;
          if (e.msg.id === "glue-a") gotA = true;
          if (e.msg.id === "glue-b") gotB = true;
        }
        if (gotA && gotB) break;
        await sleep(50);
      }
      results.push({
        case: caseName,
        observed: { gotA, gotB },
        pass: gotA && gotB,
        note: "一次 write 两帧应都被处理",
      });
    } catch (e) {
      results.push({ case: caseName, error: String(e) });
    } finally {
      await d.stop();
    }
  }

  // --- 3 超长行 ---
  {
    const d = createRpcDriver({
      runtime,
      thinking: "off",
      noTools: true,
      samplePath,
    });
    const caseName = "overlong_line";
    try {
      await d.start();
      const big = "x".repeat(2_000_000);
      const frame =
        JSON.stringify({
          id: "big-1",
          type: "prompt",
          message: big,
        }) + "\n";
      const t0 = performance.now();
      d.child.stdin.write(frame);
      // 观察 3s 内进程是否崩溃 / 是否有 error response
      await sleep(3000);
      const exited = d.getMeta().exitCode != null;
      const resp = d.events.find(
        (e) =>
          e.kind === "stdout" &&
          e.msg?.type === "response" &&
          e.msg?.id === "big-1",
      );
      const parseErr = d.events.filter((e) => e.kind === "parse_error");
      results.push({
        case: caseName,
        observed: {
          bytes: frame.length,
          exitedWithin3s: exited,
          exitCode: d.getMeta().exitCode,
          response: resp?.msg || null,
          parseErrors: parseErr.length,
          elapsedMs: performance.now() - t0,
        },
        note: "记录 runtime 对 ~2MB JSONL 行的行为（接受/拒绝/崩溃）",
      });
    } catch (e) {
      results.push({ case: caseName, error: String(e) });
    } finally {
      await d.stop();
    }
  }

  // --- 4 非法 JSON ---
  {
    const d = createRpcDriver({
      runtime,
      thinking: "off",
      noTools: true,
      samplePath,
    });
    const caseName = "invalid_json";
    try {
      await d.start();
      await d.writeRaw(["{not-json\n"]);
      await sleep(500);
      // 随后发合法帧，看会话是否仍可用
      let stillWorks = false;
      try {
        await d.request("get_state", {}, 5000);
        stillWorks = true;
      } catch (e) {
        stillWorks = false;
      }
      const exited = d.getMeta().exitCode != null;
      results.push({
        case: caseName,
        observed: {
          processExited: exited,
          subsequentGetStateWorks: stillWorks,
          stderrTail: d.getMeta().stderr?.slice(-500),
        },
        note: "非法 JSON 行后，合法命令是否仍可用",
      });
    } catch (e) {
      results.push({ case: caseName, error: String(e) });
    } finally {
      await d.stop();
    }
  }

  // --- 5 进程中途被杀 ---
  {
    const d = createRpcDriver({
      runtime,
      thinking: "off",
      noTools: true,
      samplePath,
    });
    const caseName = "kill_midway";
    try {
      await d.start();
      // 发一个会跑一会儿的 prompt，然后立刻 SIGKILL
      const promptP = d
        .request(
          "prompt",
          {
            message:
              "Count slowly from 1 to 100 in words. Do not skip. Be verbose.",
          },
          60000,
        )
        .catch((e) => ({ error: String(e) }));
      await sleep(300);
      d.child.kill("SIGKILL");
      const promptResult = await promptP;
      await sleep(200);
      results.push({
        case: caseName,
        observed: {
          exitCode: d.getMeta().exitCode,
          promptResult,
        },
        note: "SIGKILL 后 pending request 应失败；exitCode 非 0 或 null→killed",
      });
    } catch (e) {
      results.push({ case: caseName, error: String(e) });
    } finally {
      await d.stop();
    }
  }

  return results;
}

const runtime = process.argv[2] || "both";
const list = runtime === "both" ? ["pi", "omp"] : [runtime];
const all = {};
for (const r of list) {
  console.error(`=== anomalies ${r} ===`);
  all[r] = await runAnomalies(r);
}
const out = join(RESEARCH_ROOT, "samples", "protocol-anomalies-summary.json");
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, JSON.stringify(all, null, 2));
console.log(JSON.stringify(all, null, 2));
console.error(`wrote ${out}`);
