import { describe, expect, it } from "vitest";
import { createJsonlFramer } from "../src/runtime/jsonl-framer.js";

describe("jsonl framer", () => {
  it("handles half packets then completes", () => {
    const frames: unknown[] = [];
    const invalid: string[] = [];
    const framer = createJsonlFramer({
      onFrame: (v) => frames.push(v),
      onInvalid: (raw) => invalid.push(raw),
    });
    framer.push('{"type":"a"');
    expect(frames).toHaveLength(0);
    framer.push(',"ok":true}\n');
    expect(frames).toEqual([{ type: "a", ok: true }]);
    expect(invalid).toHaveLength(0);
  });

  it("handles sticky packets (two frames in one chunk)", () => {
    const frames: unknown[] = [];
    const framer = createJsonlFramer({
      onFrame: (v) => frames.push(v),
    });
    framer.push('{"id":1}\n{"id":2}\n');
    expect(frames).toEqual([{ id: 1 }, { id: 2 }]);
  });

  it("reports invalid JSON without throwing", () => {
    const frames: unknown[] = [];
    const invalid: string[] = [];
    const framer = createJsonlFramer({
      onFrame: (v) => frames.push(v),
      onInvalid: (raw) => invalid.push(raw),
    });
    framer.push("not-json\n");
    framer.push('{"ok":1}\n');
    expect(invalid).toEqual(["not-json"]);
    expect(frames).toEqual([{ ok: 1 }]);
  });

  it("drops oversize unterminated lines", () => {
    const oversize: number[] = [];
    const framer = createJsonlFramer(
      {
        onFrame: () => undefined,
        onInvalid: () => undefined,
        onOversize: (len) => oversize.push(len),
      },
      { maxLineChars: 16 },
    );
    framer.push("x".repeat(32));
    expect(oversize[0]).toBe(32);
    expect(framer.pendingChars()).toBe(0);
  });
});
