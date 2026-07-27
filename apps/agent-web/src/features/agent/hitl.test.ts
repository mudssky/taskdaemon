import { describe, expect, it } from "vitest";
import { hitlPhase, runStatusLabel, validateHitlResponse } from "./hitl";

describe("hitl helpers", () => {
  it("hitlPhase 映射", () => {
    expect(hitlPhase("h1", "awaiting")).toBe("awaiting");
    expect(hitlPhase(undefined, "approved")).toBe("resolved");
    expect(hitlPhase(undefined, "timed_out")).toBe("timed_out");
    expect(hitlPhase(undefined)).toBe("none");
  });

  it("validateHitlResponse 校验 modify 与超时", () => {
    const request = {
      requestId: "h1",
      title: "t",
      context: "c",
      timeoutMs: 1000,
    };
    expect(validateHitlResponse(request, "modify", "  ", 2000, 0).ok).toBe(
      false,
    );
    expect(validateHitlResponse(request, "modify", "ok", 500, 0).ok).toBe(true);
    expect(
      validateHitlResponse(request, "approve", undefined, 5000, 0).ok,
    ).toBe(false);
  });

  it("runStatusLabel 含等待人工", () => {
    expect(runStatusLabel("awaiting_human")).toContain("确认");
  });
});
