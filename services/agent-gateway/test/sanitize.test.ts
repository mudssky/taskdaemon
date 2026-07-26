import { describe, expect, it } from "vitest";
import {
  isSensitiveKey,
  REDACTED,
  sanitizeModelInfo,
  sanitizeSecrets,
} from "../src/runtime/sanitize.js";

describe("sanitizeSecrets", () => {
  it("redacts Authorization and nested apiKey", () => {
    const input = {
      model: {
        id: "grok",
        headers: { Authorization: "Bearer secret-token" },
        apiKey: "sk-xxx",
      },
      token: "abc",
      safe: "ok",
    };
    const out = sanitizeSecrets(input);
    expect(out.token).toBe(REDACTED);
    expect(out.model.apiKey).toBe(REDACTED);
    expect(out.model.headers).toBe(REDACTED);
    expect(out.safe).toBe("ok");
    // 不修改入参
    expect(input.token).toBe("abc");
  });

  it("detects sensitive keys case-insensitively", () => {
    expect(isSensitiveKey("Authorization")).toBe(true);
    expect(isSensitiveKey("API_KEY")).toBe(true);
    expect(isSensitiveKey("model")).toBe(false);
  });

  it("sanitizeModelInfo strips headers metadata", () => {
    const info = sanitizeModelInfo({
      id: "grok-4.5",
      name: "Grok",
      provider: "xh",
      headers: { Authorization: "Bearer x" },
      contextWindow: 100,
    });
    expect(info.id).toBe("grok-4.5");
    expect(info.metadata?.headers).toBeUndefined();
    expect(JSON.stringify(info)).not.toContain("Bearer");
  });
});
