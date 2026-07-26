/**
 * 跨 adapter 一致性：同一 FakeTransport 剧本跑 pi 与 omp。
 * 差异只应体现在 capabilities。
 */

import type { AgentRuntime, RuntimeEvent } from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import { createOmpAdapter, createPiAdapter } from "../src/runtime/index.js";
import {
  FakeTransport,
  type NativeMessage,
} from "../src/runtime/rpc-client.js";

function makeTransport(): FakeTransport {
  const t = new FakeTransport();
  t.setAutoHandler((req) => {
    const id = req.id;
    if (req.type === "get_state") {
      return [
        {
          id,
          type: "response",
          command: "get_state",
          success: true,
          data: {
            sessionId: "native-sess",
            isStreaming: false,
            messageCount: 0,
            model: {
              id: "grok-4.5",
              provider: "xh",
              headers: { Authorization: "Bearer secret" },
            },
          },
        },
      ];
    }
    if (req.type === "prompt") {
      // response accept + 事件流
      queueMicrotask(() => {
        const stream: NativeMessage[] = [
          { type: "agent_start" },
          { type: "turn_start" },
          {
            type: "message_start",
            message: {
              role: "user",
              content: [{ type: "text", text: String(req.message ?? "") }],
            },
          },
          {
            type: "message_end",
            message: {
              role: "user",
              content: [{ type: "text", text: String(req.message ?? "") }],
            },
          },
          {
            type: "message_start",
            message: { role: "assistant", content: [] },
          },
          {
            type: "message_update",
            assistantMessageEvent: { type: "text_delta", delta: "PONG" },
          },
          {
            type: "message_end",
            message: {
              role: "assistant",
              content: [{ type: "text", text: "PONG" }],
              usage: { input: 1, output: 1, totalTokens: 2 },
            },
          },
          { type: "turn_end" },
          { type: "agent_end" },
          { type: "agent_settled" },
        ];
        for (const msg of stream) t.pushMessage(msg);
      });
      return [{ id, type: "response", command: "prompt", success: true }];
    }
    if (req.type === "abort") {
      return [{ id, type: "response", command: "abort", success: true }];
    }
    if (req.type === "get_messages") {
      return [
        {
          id,
          type: "response",
          command: "get_messages",
          success: true,
          data: {
            messages: [
              {
                id: "m1",
                role: "user",
                content: [{ type: "text", text: "hi" }],
                createdAt: new Date().toISOString(),
              },
            ],
            hasMore: false,
          },
        },
      ];
    }
    if (req.type === "set_model") {
      if (req.model === "bad-model") {
        return [
          {
            id,
            type: "response",
            command: "set_model",
            success: false,
            error: "unknown model",
          },
        ];
      }
      return [{ id, type: "response", command: "set_model", success: true }];
    }
    if (req.type === "steer" || req.type === "follow_up") {
      return [
        {
          id,
          type: "response",
          command: String(req.type),
          success: true,
        },
      ];
    }
    return [
      {
        id,
        type: "response",
        command: String(req.type),
        success: true,
      },
    ];
  });
  return t;
}

async function collect(
  runtime: AgentRuntime,
  sessionId: string,
  text: string,
): Promise<RuntimeEvent[]> {
  const out: RuntimeEvent[] = [];
  for await (const ev of runtime.prompt(sessionId, { text })) {
    out.push(ev);
  }
  return out;
}

const factories = [
  {
    name: "pi",
    create: () =>
      createPiAdapter({
        transportFactory: async () => makeTransport(),
      }),
  },
  {
    name: "omp",
    create: () =>
      createOmpAdapter({
        transportFactory: async () => makeTransport(),
      }),
  },
] as const;

describe.each(factories)("adapter $name lifecycle", ({ create }) => {
  it("creates general session without cwd", async () => {
    const rt = create();
    const handle = await rt.createSession({
      profile: "general",
      workspaceBinding: false,
      tools: "none",
    });
    expect(handle.runtimeId).toBe(rt.id);
    expect(handle.sessionId).toBeTruthy();
    const state = await rt.getState(handle.sessionId);
    expect(state.isStreaming).toBe(false);
    expect(JSON.stringify(state)).not.toContain("Bearer");
    expect(JSON.stringify(state)).not.toContain("secret");
    await rt.disposeSession(handle.sessionId);
  });

  it("prompt streams run_started → text → run_finished", async () => {
    const rt = create();
    const handle = await rt.createSession({
      profile: "coding",
      cwd: process.cwd(),
    });
    const events = await collect(rt, handle.sessionId, "Reply PONG");
    const types = events.map((e) => e.type);
    expect(types[0]).toBe("run_started");
    expect(types).toContain("message_text_delta");
    expect(types.at(-1)).toBe("run_finished");
    await rt.disposeSession(handle.sessionId);
  });

  it("abort returns without hanging", async () => {
    const rt = create();
    const handle = await rt.createSession({
      profile: "general",
      tools: "none",
    });
    await rt.abort(handle.sessionId);
    await rt.disposeSession(handle.sessionId);
  });

  it("getMessages works", async () => {
    const rt = create();
    const handle = await rt.createSession({ profile: "coding" });
    const page = await rt.getMessages(handle.sessionId, { limit: 10 });
    expect(page.messages.length).toBeGreaterThan(0);
    await rt.disposeSession(handle.sessionId);
  });

  it("setModel failure is explicit", async () => {
    const rt = create();
    const handle = await rt.createSession({ profile: "coding" });
    await expect(
      rt.setModel?.(handle.sessionId, { id: "bad-model" }),
    ).rejects.toMatchObject({ code: "AGENT_VALIDATION_FAILED" });
    await rt.disposeSession(handle.sessionId);
  });
});

describe("capabilities honesty", () => {
  it("omp history is messages only; pi allows messages+entries", () => {
    const pi = createPiAdapter({
      transportFactory: async () => makeTransport(),
    });
    const omp = createOmpAdapter({
      transportFactory: async () => makeTransport(),
    });
    expect(pi.capabilities().history).toBe("messages+entries");
    expect(omp.capabilities().history).toBe("messages");
    expect(omp.capabilities().memoryClass).toBe("heavy");
    expect(pi.capabilities().memoryClass).toBe("light");
    // 不模拟：两者 coldStart 均为 high（CLI）
    expect(pi.capabilities().coldStartCost).toBe("high");
    expect(omp.capabilities().coldStartCost).toBe("high");
  });
});
