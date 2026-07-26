import {
  type AguiEvent,
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
} from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import {
  appendLocalUserMessage,
  applyLocalCancel,
  createEmptyStreamState,
  hydrateTimelineFromMessages,
  reduceAguiEvent,
  toggleToolCollapsed,
} from "./stream-reducer";

describe("reduceAguiEvent", () => {
  it("归约文本流与 run 生命周期", () => {
    let state = createEmptyStreamState();
    const events: AguiEvent[] = [
      { type: "RUN_STARTED", threadId: "t1", runId: "r1", id: "1" },
      {
        type: "TEXT_MESSAGE_START",
        messageId: "m1",
        role: "assistant",
        id: "2",
      },
      {
        type: "TEXT_MESSAGE_CONTENT",
        messageId: "m1",
        delta: "Hel",
        id: "3",
      },
      {
        type: "TEXT_MESSAGE_CONTENT",
        messageId: "m1",
        delta: "lo",
        id: "4",
      },
      { type: "TEXT_MESSAGE_END", messageId: "m1", id: "5" },
      {
        type: "RUN_FINISHED",
        threadId: "t1",
        runId: "r1",
        outcome: { type: "success" },
        id: "6",
      },
    ];

    for (const event of events) {
      state = reduceAguiEvent(state, event);
    }

    expect(state.status).toBe("idle");
    expect(state.runId).toBe("r1");
    expect(state.items).toHaveLength(1);
    const msg = state.items[0];
    expect(msg?.kind).toBe("message");
    if (msg?.kind === "message") {
      expect(msg.text).toBe("Hello");
      expect(msg.streaming).toBe(false);
    }
  });

  it("toolCallDetail=name-only 忽略 ARGS 且保留工具名", () => {
    let state = createEmptyStreamState();
    const opts = {
      toolCallDetail: "name-only" as const,
      includeThinking: false,
    };
    state = reduceAguiEvent(
      state,
      { type: "RUN_STARTED", threadId: "t", runId: "r", id: "1" },
      opts,
    );
    state = reduceAguiEvent(
      state,
      {
        type: "TOOL_CALL_START",
        toolCallId: "tc1",
        toolCallName: "bash",
        id: "2",
      },
      opts,
    );
    state = reduceAguiEvent(
      state,
      {
        type: "TOOL_CALL_ARGS",
        toolCallId: "tc1",
        delta: '{"cmd":"rm"}',
        id: "3",
      },
      opts,
    );
    state = reduceAguiEvent(
      state,
      {
        type: "TOOL_CALL_RESULT",
        toolCallId: "tc1",
        content: "ok",
        id: "4",
      },
      opts,
    );

    const tool = state.items.find((i) => i.kind === "tool");
    expect(tool).toBeDefined();
    if (tool?.kind === "tool") {
      expect(tool.name).toBe("bash");
      expect(tool.args).toBeUndefined();
      expect(tool.result).toBe("ok");
      expect(tool.status).toBe("success");
    }
  });

  it("toolCallDetail=none 不展示任何工具条目", () => {
    let state = createEmptyStreamState();
    const opts = {
      toolCallDetail: "none" as const,
      includeThinking: true,
    };
    state = reduceAguiEvent(
      state,
      {
        type: "TOOL_CALL_START",
        toolCallId: "tc1",
        toolCallName: "bash",
      },
      opts,
    );
    state = reduceAguiEvent(
      state,
      { type: "TOOL_CALL_RESULT", toolCallId: "tc1", content: "x" },
      opts,
    );
    expect(state.items.filter((i) => i.kind === "tool")).toHaveLength(0);
  });

  it("includeThinking=false 丢弃 REASONING 事件", () => {
    let state = createEmptyStreamState();
    const opts = {
      toolCallDetail: "full" as const,
      includeThinking: false,
    };
    state = reduceAguiEvent(
      state,
      { type: "REASONING_START", messageId: "m1" },
      opts,
    );
    state = reduceAguiEvent(
      state,
      { type: "REASONING_CONTENT", messageId: "m1", delta: "secret" },
      opts,
    );
    expect(state.items).toHaveLength(0);
  });

  it("RUN_ERROR 进入 error 并停止 streaming", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "RUN_STARTED",
      threadId: "t",
      runId: "r",
    });
    state = reduceAguiEvent(state, {
      type: "TEXT_MESSAGE_START",
      messageId: "m",
      role: "assistant",
    });
    state = reduceAguiEvent(state, {
      type: "RUN_ERROR",
      threadId: "t",
      runId: "r",
      message: "boom",
      code: "AGENT_INTERNAL_ERROR",
    });
    expect(state.status).toBe("error");
    expect(state.error?.message).toBe("boom");
    const msg = state.items[0];
    if (msg?.kind === "message") {
      expect(msg.streaming).toBe(false);
    }
  });

  it("interrupt 结局标记 cancelled", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "RUN_STARTED",
      threadId: "t",
      runId: "r",
    });
    state = reduceAguiEvent(state, {
      type: "RUN_FINISHED",
      threadId: "t",
      runId: "r",
      outcome: { type: "interrupt", reason: "cancelled" },
    });
    expect(state.status).toBe("cancelled");
  });
});

describe("hydrateTimelineFromMessages", () => {
  it("水合用户/助手与工具块", () => {
    const items = hydrateTimelineFromMessages(
      [
        {
          id: "u1",
          role: "user",
          content: [{ type: "text", text: "hi" }],
          createdAt: "2026-07-27T00:00:00.000Z",
        },
        {
          id: "a1",
          role: "assistant",
          content: [
            { type: "thinking", text: "plan" },
            { type: "text", text: "ok" },
            {
              type: "tool_use",
              toolCallId: "t1",
              name: "bash",
              args: { c: "ls" },
            },
            {
              type: "tool_result",
              toolCallId: "t1",
              content: "a\nb",
            },
          ],
          createdAt: "2026-07-27T00:00:01.000Z",
        },
      ],
      { includeThinking: true, toolCallDetail: "full" },
    );

    expect(items.some((i) => i.kind === "message" && i.role === "user")).toBe(
      true,
    );
    const asst = items.find(
      (i) => i.kind === "message" && i.role === "assistant",
    );
    expect(asst?.kind === "message" && asst.thinking).toBe("plan");
    const tool = items.find((i) => i.kind === "tool");
    expect(tool?.kind === "tool" && tool.name).toBe("bash");
  });
});

describe("stream helpers", () => {
  it("appendLocalUserMessage / cancel / toggle collapse", () => {
    let state = createEmptyStreamState();
    state = appendLocalUserMessage(state, "hello", "local1");
    expect(state.items[0]?.kind).toBe("message");

    state = reduceAguiEvent(state, {
      type: "TOOL_CALL_START",
      toolCallId: "tc",
      toolCallName: "x",
    });
    state = reduceAguiEvent(state, {
      type: "TOOL_CALL_RESULT",
      toolCallId: "tc",
      content: "y".repeat(300),
    });
    const before = state.items.find((i) => i.kind === "tool");
    expect(before?.kind === "tool" && before.collapsed).toBe(true);
    state = toggleToolCollapsed(state, "tc");
    const after = state.items.find((i) => i.kind === "tool");
    expect(after?.kind === "tool" && after.collapsed).toBe(false);

    state = applyLocalCancel(state, "cancelled");
    expect(state.status).toBe("cancelled");
  });

  it("capabilities 常量可被测试引用（双 adapter 模板存在）", () => {
    expect(PI_CLI_CAPABILITIES.toolCallDetail).toBe("full");
    expect(OMP_CLI_CAPABILITIES.history).toBe("messages");
  });
});
