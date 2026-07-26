import { describe, expect, it } from "vitest";
import {
  formatActivityTime,
  splitMessageSegments,
  threadStatusLabel,
} from "./message-format";

describe("splitMessageSegments", () => {
  it("拆分 fenced code 与普通文本并保留换行", () => {
    const source = "前言\n```ts\nconst a = 1;\n```\n后记";
    const segments = splitMessageSegments(source);
    expect(segments).toEqual([
      { type: "text", value: "前言\n" },
      { type: "code", language: "ts", value: "const a = 1;\n" },
      { type: "text", value: "\n后记" },
    ]);
  });

  it("无代码块时整段为 text", () => {
    expect(splitMessageSegments("hello\nworld")).toEqual([
      { type: "text", value: "hello\nworld" },
    ]);
  });
});

describe("format helpers", () => {
  it("formatActivityTime 相对时间", () => {
    const now = new Date("2026-07-27T12:00:00.000Z");
    expect(formatActivityTime("2026-07-27T11:59:30.000Z", now)).toBe("刚刚");
    expect(formatActivityTime("2026-07-27T11:30:00.000Z", now)).toBe(
      "30 分钟前",
    );
  });

  it("threadStatusLabel", () => {
    expect(threadStatusLabel("busy")).toBe("运行中");
    expect(threadStatusLabel("unknown")).toBe("unknown");
  });
});
