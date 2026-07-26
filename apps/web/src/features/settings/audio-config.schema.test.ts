import { describe, expect, it } from "vitest";
import type { AudioConfig } from "../../lib/api/types";
import {
  audioConfigFormSchema,
  defaultAudioConfigFormValues,
  isEmptyAudioConfigPayload,
  parseStringList,
  toAudioConfigWritePayload,
} from "./audio-config.schema";

const sampleConfig: AudioConfig = {
  autoplay: { enabled: true, target: "backend" },
  playback: { queueLimit: 20 },
  inbound: {
    tokenConfigured: true,
    maxBytes: 209_715_200,
    url: {
      allowedSchemes: ["https"],
      allowPrivateNetworks: false,
      allowedHosts: [],
      downloadTimeoutSeconds: 60,
      maxRedirects: 3,
    },
  },
  history: { limit: 50 },
  ffmpeg: {
    pathConfigured: false,
    probePathConfigured: false,
    transcodeTimeoutSeconds: 120,
  },
  configuration: {
    runtimeEditable: ["autoplay", "playback", "inbound", "history"],
    restartRequired: ["ffmpeg.transcodeTimeoutSeconds"],
    fileOnly: ["ffmpeg.path", "ffmpeg.probePath", "inbound.tokenHash"],
  },
};

describe("audio config schema", () => {
  it("maps DTO defaults without exposing token plaintext", () => {
    const values = defaultAudioConfigFormValues(sampleConfig);
    expect(values.autoplayEnabled).toBe(true);
    expect(values.autoplayTarget).toBe("backend");
    expect(values.token).toBe("");
    expect(values.resetToken).toBe(false);
    expect(values).not.toHaveProperty("tokenHash");
  });

  it("rejects invalid schemes and empty schemes", () => {
    const invalid = audioConfigFormSchema.safeParse({
      ...defaultAudioConfigFormValues(sampleConfig),
      allowedSchemesText: "ftp",
    });
    expect(invalid.success).toBe(false);

    const empty = audioConfigFormSchema.safeParse({
      ...defaultAudioConfigFormValues(sampleConfig),
      allowedSchemesText: "   ",
    });
    expect(empty.success).toBe(false);
  });

  it("rejects non-positive maxBytes and transcode timeout", () => {
    const maxBytes = audioConfigFormSchema.safeParse({
      ...defaultAudioConfigFormValues(sampleConfig),
      maxBytes: 0,
    });
    expect(maxBytes.success).toBe(false);

    const timeout = audioConfigFormSchema.safeParse({
      ...defaultAudioConfigFormValues(sampleConfig),
      transcodeTimeoutSeconds: 0,
    });
    expect(timeout.success).toBe(false);
  });

  it("requires token when resetToken is enabled", () => {
    const result = audioConfigFormSchema.safeParse({
      ...defaultAudioConfigFormValues(sampleConfig),
      resetToken: true,
      token: "  ",
    });
    expect(result.success).toBe(false);
  });

  it("builds partial payload only for dirty fields", () => {
    const values = defaultAudioConfigFormValues(sampleConfig);
    values.queueLimit = 8;
    values.resetToken = true;
    values.token = "plain-secret-token";

    const payload = toAudioConfigWritePayload(values, {
      queueLimit: true,
      resetToken: true,
      token: true,
    });

    expect(payload).toEqual({
      playback: { queueLimit: 8 },
      inbound: { token: "plain-secret-token" },
    });
    expect(payload.ffmpeg).toBeUndefined();
    expect(payload.autoplay).toBeUndefined();
  });

  it("does not put token in payload when resetToken is false", () => {
    const values = defaultAudioConfigFormValues(sampleConfig);
    values.token = "should-not-send";
    const payload = toAudioConfigWritePayload(values, {
      token: true,
    });
    expect(payload.inbound?.token).toBeUndefined();
    expect(isEmptyAudioConfigPayload(payload)).toBe(true);
  });

  it("parses scheme lists", () => {
    expect(parseStringList("HTTPS, http")).toEqual(["https", "http"]);
  });
});
