import { afterEach, describe, expect, it } from "vitest";
import type { AudioConfig } from "../../lib/api/types";
import {
  assertTokenNotPersisted,
  configFieldErrorMessage,
  createOneTimeTokenScratch,
  extractConfigFieldErrors,
  mapConfigPathToFormField,
  needsPrivateNetworkConfirm,
  needsTokenResetConfirm,
  payloadContainsToken,
} from "./audio-config-errors";

const baseConfig: AudioConfig = {
  autoplay: { enabled: true, target: "backend" },
  playback: { queueLimit: 20 },
  inbound: {
    tokenConfigured: false,
    maxBytes: 100,
    url: {
      allowedSchemes: ["https"],
      allowPrivateNetworks: false,
      allowedHosts: [],
      downloadTimeoutSeconds: 30,
      maxRedirects: 1,
    },
  },
  history: { limit: 10 },
  ffmpeg: {
    pathConfigured: false,
    probePathConfigured: false,
    transcodeTimeoutSeconds: 60,
  },
  configuration: {
    runtimeEditable: [],
    restartRequired: [],
    fileOnly: [],
  },
};

describe("audio config error helpers", () => {
  afterEach(() => {
    localStorage.clear();
  });

  it("maps server field paths to form fields", () => {
    expect(mapConfigPathToFormField("audio.playback.queueLimit")).toBe(
      "queueLimit",
    );
    expect(mapConfigPathToFormField("audio.inbound.token")).toBe("token");
    expect(mapConfigPathToFormField("unknown")).toBeNull();
  });

  it("extracts field errors from details.fields", () => {
    const fields = extractConfigFieldErrors({
      fields: [
        {
          path: "audio.history.limit",
          reason: "must be >= 0",
          code: "CONFIG_FIELD_OUT_OF_RANGE",
        },
      ],
    });
    expect(fields).toHaveLength(1);
    const first = fields[0];
    expect(first).toBeDefined();
    if (!first) {
      return;
    }
    expect(configFieldErrorMessage(first)).toBe("取值超出允许范围");
  });

  it("detects dangerous confirms", () => {
    expect(needsPrivateNetworkConfirm(baseConfig, true)).toBe(true);
    expect(needsPrivateNetworkConfirm(baseConfig, false)).toBe(false);
    expect(needsTokenResetConfirm(true, "abc")).toBe(true);
    expect(needsTokenResetConfirm(true, "")).toBe(false);
  });

  it("keeps one-time token only in scratch memory shape", () => {
    const scratch = createOneTimeTokenScratch("once-only-token");
    expect(scratch.oneTimeToken).toBe("once-only-token");
    expect(payloadContainsToken({ inbound: { token: "x" } })).toBe(true);
    expect(payloadContainsToken({ playback: { queueLimit: 1 } })).toBe(false);
  });

  it("fails if sensitive token is written to localStorage", () => {
    const token = "super-secret-token-value";
    assertTokenNotPersisted(token);
    localStorage.setItem("leak", token);
    expect(() => assertTokenNotPersisted(token)).toThrow(
      /must not enter localStorage/,
    );
  });
});
