import { describe, expect, it } from "vitest";
import {
  createEventMapperState,
  mapNativeToRuntimeEvents,
} from "../src/runtime/event-mapper.js";

const ctx = {
  sessionId: "s1",
  allowFileChange: true,
  thinkingEnabled: true,
};

describe("event mapper", () => {
  it("maps agent/turn/message/tool skeleton", () => {
    const state = createEventMapperState();
    const events = [
      ...mapNativeToRuntimeEvents({ type: "agent_start" }, ctx, state),
      ...mapNativeToRuntimeEvents({ type: "turn_start" }, ctx, state),
      ...mapNativeToRuntimeEvents(
        {
          type: "message_start",
          message: { role: "assistant", content: [] },
        },
        ctx,
        state,
      ),
      ...mapNativeToRuntimeEvents(
        {
          type: "message_update",
          assistantMessageEvent: { type: "text_delta", delta: "hi" },
        },
        ctx,
        state,
      ),
      ...mapNativeToRuntimeEvents(
        {
          type: "tool_execution_start",
          toolCallId: "c1",
          toolName: "bash",
          args: { command: "echo" },
        },
        ctx,
        state,
      ),
      ...mapNativeToRuntimeEvents(
        {
          type: "tool_execution_end",
          toolCallId: "c1",
          toolName: "bash",
          result: { content: [{ type: "text", text: "ok" }] },
          isError: false,
        },
        ctx,
        state,
      ),
      ...mapNativeToRuntimeEvents({ type: "agent_end" }, ctx, state),
    ];
    const types = events.map((e) => e.type);
    expect(types).toContain("run_started");
    expect(types).toContain("step_started");
    expect(types).toContain("message_start");
    expect(types).toContain("message_text_delta");
    expect(types).toContain("tool_call_start");
    expect(types).toContain("tool_call_args_delta");
    expect(types).toContain("tool_call_result");
    expect(types).toContain("run_finished");
  });

  it("ignores extension_ui_request and does not fabricate data", () => {
    const state = createEventMapperState();
    const events = mapNativeToRuntimeEvents(
      { type: "extension_ui_request", method: "setStatus" },
      ctx,
      state,
    );
    expect(events).toEqual([
      expect.objectContaining({
        type: "raw_ignored",
        sourceType: "extension_ui_request",
      }),
    ]);
  });

  it("drops thinking when disabled", () => {
    const state = createEventMapperState();
    mapNativeToRuntimeEvents(
      { type: "message_start", message: { role: "assistant" } },
      { ...ctx, thinkingEnabled: false },
      state,
    );
    const events = mapNativeToRuntimeEvents(
      {
        type: "message_update",
        assistantMessageEvent: {
          type: "thinking_delta",
          delta: "secret think",
        },
      },
      { ...ctx, thinkingEnabled: false },
      state,
    );
    expect(events).toEqual([]);
  });
});
