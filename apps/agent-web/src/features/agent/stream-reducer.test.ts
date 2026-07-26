import {
  type AguiEvent,
  OMP_CLI_CAPABILITIES,
  PI_CLI_CAPABILITIES,
} from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import {
  appendLocalUserMessage,
  appendSteerMessage,
  applyHitlDecision,
  applyLocalCancel,
  createEmptyStreamState,
  hydrateTimelineFromMessages,
  markHitlTimedOut,
  reduceAguiEvent,
  toggleThinkingCollapsed,
  toggleToolCollapsed,
} from "./stream-reducer";

describe("reduceAguiEvent", () => {
  it("归约文本流与 run 生命周期", () => {
    let state = createEmptyStreamState();
    const events: AguiEvent[] = [
      { type: "RUN_STARTED", threadId: "t1", runId: "r1", id: "r1:0" },
      {
        type: "TEXT_MESSAGE_START",
        messageId: "m1",
        role: "assistant",
        id: "r1:1",
      },
      {
        type: "TEXT_MESSAGE_CONTENT",
        messageId: "m1",
        delta: "Hello",
        id: "r1:2",
      },
      { type: "TEXT_MESSAGE_END", messageId: "m1", id: "r1:3" },
      {
        type: "RUN_FINISHED",
        threadId: "t1",
        runId: "r1",
        outcome: { type: "success" },
        id: "r1:4",
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

  it("续传时按 event.id 去重不重复渲染", () => {
    let state = createEmptyStreamState();
    const event: AguiEvent = {
      type: "TEXT_MESSAGE_START",
      messageId: "m1",
      role: "assistant",
      id: "r1:1",
    };
    state = reduceAguiEvent(state, {
      type: "RUN_STARTED",
      threadId: "t",
      runId: "r1",
      id: "r1:0",
    });
    state = reduceAguiEvent(state, event);
    state = reduceAguiEvent(state, {
      ...event,
      // 相同 id 再推一次
    });
    state = reduceAguiEvent(state, {
      type: "TEXT_MESSAGE_CONTENT",
      messageId: "m1",
      delta: "A",
      id: "r1:1",
    });
    expect(state.items.filter((i) => i.kind === "message")).toHaveLength(1);
    const msg = state.items.find((i) => i.kind === "message");
    if (msg?.kind === "message") {
      expect(msg.text).toBe("");
    }
  });

  it("toolCallDetail=name-only 忽略 ARGS 且保留工具名", () => {
    let state = createEmptyStreamState();
    const opts = {
      toolCallDetail: "name-only" as const,
      includeThinking: true,
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
        toolCallName: "echo",
        id: "2",
      },
      opts,
    );
    state = reduceAguiEvent(
      state,
      { type: "TOOL_CALL_ARGS", toolCallId: "tc1", delta: "{x:1}", id: "3" },
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
    expect(tool?.kind).toBe("tool");
    if (tool?.kind === "tool") {
      expect(tool.name).toBe("echo");
      expect(tool.args).toBeUndefined();
      expect(tool.result).toBe("ok");
    }
  });

  it("toolCallDetail=none 不展示任何工具条目", () => {
    let state = createEmptyStreamState();
    const opts = { toolCallDetail: "none" as const, includeThinking: true };
    state = reduceAguiEvent(
      state,
      {
        type: "TOOL_CALL_START",
        toolCallId: "tc1",
        toolCallName: "echo",
        id: "1",
      },
      opts,
    );
    expect(state.items).toHaveLength(0);
  });

  it("includeThinking=false 丢弃 REASONING 事件", () => {
    let state = createEmptyStreamState();
    const opts = { toolCallDetail: "full" as const, includeThinking: false };
    state = reduceAguiEvent(
      state,
      { type: "REASONING_START", messageId: "m1", id: "1" },
      opts,
    );
    state = reduceAguiEvent(
      state,
      {
        type: "REASONING_CONTENT",
        messageId: "m1",
        delta: "secret",
        id: "2",
      },
      opts,
    );
    expect(state.items).toHaveLength(0);
  });

  it("thinking 默认折叠并可切换", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "REASONING_START",
      messageId: "m1",
      id: "1",
    });
    state = reduceAguiEvent(state, {
      type: "REASONING_CONTENT",
      messageId: "m1",
      delta: "plan",
      id: "2",
    });
    const msg = state.items[0];
    expect(msg?.kind).toBe("message");
    if (msg?.kind === "message") {
      expect(msg.thinkingCollapsed).toBe(true);
    }
    state = toggleThinkingCollapsed(state, "m1");
    const opened = state.items[0];
    if (opened?.kind === "message") {
      expect(opened.thinkingCollapsed).toBe(false);
    }
  });

  it("CUSTOM hitl_request 进入 awaiting_human", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "RUN_STARTED",
      threadId: "t",
      runId: "r",
      id: "0",
    });
    state = reduceAguiEvent(state, {
      type: "CUSTOM",
      name: "hitl_request",
      id: "1",
      value: {
        requestId: "h1",
        title: "确认",
        context: "ctx",
      },
    });
    expect(state.status).toBe("awaiting_human");
    expect(state.pendingHitlId).toBe("h1");
    state = applyHitlDecision(state, "h1", "approve");
    expect(state.status).toBe("running");
    expect(state.pendingHitlId).toBeUndefined();
  });

  it("HITL 超时标记", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "CUSTOM",
      name: "hitl_request",
      id: "1",
      value: {
        requestId: "h1",
        title: "确认",
        context: "ctx",
        timeoutMs: 1,
      },
    });
    state = markHitlTimedOut(state, "h1");
    expect(state.status).toBe("error");
    const hitl = state.items.find((i) => i.kind === "hitl");
    if (hitl?.kind === "hitl") {
      expect(hitl.status).toBe("timed_out");
    }
  });

  it("file_change 与 usage CUSTOM 归约", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "CUSTOM",
      name: "file_change",
      id: "1",
      value: {
        path: "a.ts",
        kind: "modify",
        diff: "+x",
        outsideSandbox: false,
      },
    });
    state = reduceAguiEvent(state, {
      type: "CUSTOM",
      name: "usage",
      id: "2",
      value: { totalTokens: 9 },
    });
    expect(state.fileChanges).toHaveLength(1);
    expect(state.fileChanges[0]?.path).toBe("a.ts");
    expect(state.usage?.totalTokens).toBe(9);
  });

  it("steering 时间线条目可区分", () => {
    let state = createEmptyStreamState();
    state = appendSteerMessage(state, "please focus", "s1");
    expect(state.items[0]?.kind).toBe("steer");
  });

  it("RUN_ERROR 进入 error 并停止 streaming", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "TEXT_MESSAGE_START",
      messageId: "m1",
      role: "assistant",
      id: "1",
    });
    state = reduceAguiEvent(state, {
      type: "RUN_ERROR",
      threadId: "t1",
      runId: "r1",
      message: "boom",
      code: "X",
      id: "2",
    });
    expect(state.status).toBe("error");
    const msg = state.items[0];
    if (msg?.kind === "message") {
      expect(msg.streaming).toBe(false);
    }
  });

  it("interrupt 结局标记 cancelled", () => {
    let state = createEmptyStreamState();
    state = reduceAguiEvent(state, {
      type: "RUN_FINISHED",
      threadId: "t",
      runId: "r",
      outcome: { type: "interrupt", reason: "cancelled" },
      id: "1",
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
          createdAt: new Date().toISOString(),
        },
        {
          id: "a1",
          role: "assistant",
          content: [
            { type: "thinking", text: "t" },
            { type: "text", text: "yo" },
            {
              type: "tool_use",
              toolCallId: "tc",
              name: "echo",
              args: { a: 1 },
            },
            {
              type: "tool_result",
              toolCallId: "tc",
              content: "ok",
              isError: false,
            },
          ],
          createdAt: new Date().toISOString(),
        },
      ],
      { includeThinking: true, toolCallDetail: "full" },
    );
    expect(items.some((i) => i.kind === "message")).toBe(true);
    expect(items.some((i) => i.kind === "tool")).toBe(true);
  });
});

describe("stream helpers", () => {
  it("appendLocalUserMessage / cancel / toggle collapse", () => {
    let state = createEmptyStreamState();
    state = appendLocalUserMessage(state, "hello", "u1");
    state = {
      ...state,
      items: [
        ...state.items,
        {
          kind: "tool",
          id: "tc",
          name: "x",
          status: "success",
          collapsed: true,
          result: "long",
        },
      ],
    };
    state = toggleToolCollapsed(state, "tc");
    const tool = state.items.find((i) => i.kind === "tool");
    if (tool?.kind === "tool") {
      expect(tool.collapsed).toBe(false);
    }
    state = applyLocalCancel(state, "cancelled");
    expect(state.status).toBe("cancelled");
  });

  it("capabilities 常量可被测试引用（双 adapter 模板存在）", () => {
    expect(PI_CLI_CAPABILITIES.thinking).toBe(true);
    expect(OMP_CLI_CAPABILITIES.history).toBeTruthy();
  });
});
