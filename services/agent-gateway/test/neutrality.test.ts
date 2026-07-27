import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { createDefaultRuntimeRegistry } from "../src/runtime/index.js";
import { FakeTransport } from "../src/runtime/rpc-client.js";

/**
 * 收集目录下 ts 文件（非递归 adapter 实现细节检查用）。
 */
function listTsFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const st = statSync(p);
    if (st.isDirectory()) {
      if (name === "pi" || name === "omp") continue; // adapter 内部可有私有细节
      out.push(...listTsFiles(p));
    } else if (name.endsWith(".ts")) {
      out.push(p);
    }
  }
  return out;
}

describe("gateway neutrality", () => {
  it("registry surface is AgentRuntime only (swap by id)", async () => {
    const makeT = () => {
      const t = new FakeTransport();
      t.setAutoHandler((req) => [
        {
          id: req.id,
          type: "response",
          command: String(req.type),
          success: true,
          data: {
            sessionId: "x",
            isStreaming: false,
            model: { id: "m" },
          },
        },
      ]);
      return t;
    };
    const registry = createDefaultRuntimeRegistry({
      piTransportFactory: async () => makeT(),
      ompTransportFactory: async () => makeT(),
    });
    // 模拟 gateway：只认 id
    for (const id of ["pi", "omp"] as const) {
      const runtime = registry.resolve(id);
      const handle = await runtime.createSession({ profile: "general" });
      expect(handle.runtimeId).toBe(id);
      await runtime.disposeSession(handle.sessionId);
    }
  });

  it("shared runtime modules do not hardcode pi/omp protocol types", () => {
    const root = join(process.cwd(), "src/runtime");
    const shared = listTsFiles(root).filter(
      (p) =>
        !p.includes(`${join("runtime", "pi")}`) &&
        !p.includes(`${join("runtime", "omp")}`),
    );
    // 允许 registry/default 工厂与字符串 id，但禁止专有事件名硬编码进共享层业务分支以外的类型导出
    const bannedTypeNames = [
      "PiSession",
      "OmpSession",
      "PiRpc",
      "OmpRpc",
      "tool_execution_start as",
    ];
    for (const file of shared) {
      const text = readFileSync(file, "utf8");
      for (const ban of bannedTypeNames) {
        expect(text.includes(ban), `${file} contains ${ban}`).toBe(false);
      }
    }
  });
});
