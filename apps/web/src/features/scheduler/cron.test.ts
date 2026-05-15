import { describe, expect, it } from "vitest";
import { validateCronExpression } from "./cron";

describe("validateCronExpression", () => {
  it("accepts five field cron without warnings", () => {
    const result = validateCronExpression("30 9 * * *", "Asia/Hong_Kong");

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.warnings).toEqual([]);
    }
  });

  it("returns second and high frequency warnings for frequent six field cron", () => {
    const result = validateCronExpression("*/5 * * * * *", "Asia/Hong_Kong");

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.warnings).toEqual([
        "second_level_cron",
        "high_frequency_cron",
      ]);
    }
  });

  it("rejects invalid field count and timezone shape", () => {
    expect(validateCronExpression("* * *", "Asia/Hong_Kong")).toEqual({
      ok: false,
      error: "cron 表达式必须是 5 或 6 个字段",
    });
    expect(validateCronExpression("30 9 * * *", "HongKong")).toEqual({
      ok: false,
      error: "timezone 需要使用 Local 或 Area/City 格式",
    });
  });
});
