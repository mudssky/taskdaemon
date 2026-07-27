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

  it("重命名 / 归档 / 搜索 / fork 不改源", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const created = await api.createThread({
      runtimeId: "pi",
      metadata: { title: "原标题" },
    });
    {
      const { promise, resolve, reject } = Promise.withResolvers<void>();
      api.streamRun(
        created.threadId,
        { input: { text: "hello unique-token" } },
        {
          onEvent: () => undefined,
          onError: reject,
          onDone: () => resolve(),
        },
      );
      await promise;
    }

    const renamed = await api.updateThread(created.threadId, {
      title: "新标题",
    });
    expect(renamed.metadata?.title).toBe("新标题");

    const found = await api.listThreads({ q: "unique-token" });
    expect(found.items.some((t) => t.threadId === created.threadId)).toBe(true);

    const forked = await api.forkThread(created.threadId, {
      title: "分叉会话",
    });
    expect(forked.threadId).not.toBe(created.threadId);
    expect(forked.metadata?.forkedFrom).toBe(created.threadId);

    const sourceMsgs = await api.listMessages(created.threadId);
    const forkMsgs = await api.listMessages(forked.threadId);
    expect(sourceMsgs.total).toBe(forkMsgs.total);

    await api.updateThread(created.threadId, { archived: true });
    const withoutArchived = await api.listThreads({});
    expect(
      withoutArchived.items.some((t) => t.threadId === created.threadId),
    ).toBe(false);
    const withArchived = await api.listThreads({ includeArchived: true });
    expect(
      withArchived.items.some((t) => t.threadId === created.threadId),
    ).toBe(true);
  });

  it("流式 run 产出 RUN_STARTED 与文本", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const thread = await api.createThread({ runtimeId: "pi" });
    const events: string[] = [];

    const { promise, resolve, reject } = Promise.withResolvers<void>();
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
    await promise;

    expect(events[0]).toBe("RUN_STARTED");
    expect(events).toContain("TEXT_MESSAGE_CONTENT");
    expect(events).toContain("TOOL_CALL_START");
    expect(events.at(-1)).toBe("RUN_FINISHED");
  });

  it("file 关键词产生 file_change；重连按 cursor 去重续传", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    const thread = await api.createThread({ runtimeId: "pi" });
    const events: { type: string; id?: string; name?: string }[] = [];
    let runId = "";

    const { promise, resolve, reject } = Promise.withResolvers<void>();
    api.streamRun(
      thread.threadId,
      { input: { text: "please file edit" } },
      {
        onEvent: (event) => {
          if (event.type === "RUN_STARTED") {
            runId = event.runId;
          }
          events.push({
            type: event.type,
            id: event.id,
            name: event.type === "CUSTOM" ? event.name : undefined,
          });
        },
        onError: reject,
        onDone: () => resolve(),
      },
    );
    await promise;

    expect(events.some((e) => e.name === "file_change")).toBe(true);
    const mid = events[2]?.id;
    expect(mid).toBeTruthy();

    const replayed: string[] = [];
    const {
      promise: p2,
      resolve: r2,
      reject: j2,
    } = Promise.withResolvers<void>();
    api.reconnectRunStream(
      thread.threadId,
      runId,
      { cursor: mid },
      {
        onEvent: (event) => {
          replayed.push(event.id ?? "");
        },
        onError: j2,
        onDone: () => r2(),
      },
    );
    await p2;
    expect(replayed.includes(mid as string)).toBe(false);
    expect(replayed.length).toBeGreaterThan(0);
  });

  it("steer 在无能力时 400", async () => {
    const api = createMockAgentApi({ delayMs: 0 });
    await expect(
      api.steer("thr_demo_omp", { input: { text: "x" } }),
    ).rejects.toMatchObject({ code: "AGENT_CAPABILITY_UNSUPPORTED" });
  });
});
