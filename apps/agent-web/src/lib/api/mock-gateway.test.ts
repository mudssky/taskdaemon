import { describe, expect, it } from "vitest";
import { createMockAgentApi } from "../../lib/api/mock-gateway";

describe("mock gateway dual runtime", () => {
  it("列出 pi/omp 两个 runtime 模板", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const runtimes = await api.listRuntimes();
    expect(runtimes.map((r) => r.runtimeId).sort()).toEqual(["omp", "pi"]);
  });

  it("pi 会话 capabilities 为 full；omp 降级", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const piCaps = await api.getCapabilities("thr_demo_pi");
    expect(piCaps.capabilities.toolCallDetail).toBe("full");
    expect(piCaps.capabilities.thinking).toBe(true);

    const ompCaps = await api.getCapabilities("thr_demo_omp");
    expect(ompCaps.capabilities.toolCallDetail).toBe("name-only");
    expect(ompCaps.capabilities.steering).toBe(false);
    expect(ompCaps.capabilities.workspaceBinding).toBe(false);
  });

  it("创建 / 列表 / 删除会话", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const created = await api.createThread({
      runtimeId: "pi",
      metadata: { title: "test" },
    });
    const list = await api.listThreads({ limit: 50 });
    expect(list.items.some((t) => t.threadId === created.threadId)).toBe(true);
    await api.deleteThread(created.threadId);
    const after = await api.listThreads({ limit: 50 });
    expect(after.items.some((t) => t.threadId === created.threadId)).toBe(
      false,
    );
  });

  it("流式 run 产出 RUN_STARTED 与文本", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const thread = await api.createThread({ runtimeId: "pi" });
    const events: string[] = [];

    await new Promise<void>((resolve, reject) => {
      api.streamRun(
        thread.threadId,
        { input: { text: "hello tool" } },
        {
          onEvent: (event) => {
            events.push(event.type);
          },
          onError: reject,
          onDone: () => resolve(),
        },
      );
    });

    expect(events[0]).toBe("RUN_STARTED");
    expect(events).toContain("TEXT_MESSAGE_CONTENT");
    expect(events).toContain("TOOL_CALL_START");
    expect(events.at(-1)).toBe("RUN_FINISHED");
  });
});
