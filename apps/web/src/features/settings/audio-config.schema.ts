import { z } from "zod";
import type { AudioConfig, AudioConfigWriteRequest } from "../../lib/api/types";

const autoplayTargetSchema = z.enum(["backend", "frontend"]);

/**
 * 音频配置表单 schema（校验不宽于 C-1 服务端）。
 */
export const audioConfigFormSchema = z
  .object({
    autoplayEnabled: z.boolean(),
    autoplayTarget: autoplayTargetSchema,
    queueLimit: z.number().int("队列上限须为整数").min(0, "队列上限须 ≥ 0"),
    maxBytes: z.number().int("大小上限须为整数").positive("大小上限须 > 0"),
    allowedSchemesText: z.string().trim().min(1, "至少保留一个协议"),
    allowPrivateNetworks: z.boolean(),
    allowedHostsText: z.string(),
    downloadTimeoutSeconds: z
      .number()
      .int("下载超时须为整数")
      .positive("下载超时须 > 0"),
    maxRedirects: z
      .number()
      .int("重定向次数须为整数")
      .min(0, "重定向次数须 ≥ 0"),
    historyLimit: z.number().int("历史条数须为整数").min(0, "历史条数须 ≥ 0"),
    transcodeTimeoutSeconds: z
      .number()
      .int("转码超时须为整数")
      .positive("转码超时须 > 0"),
    /** 是否进入 token 重置输入。 */
    resetToken: z.boolean(),
    /** 仅 resetToken=true 时提交。 */
    token: z.string(),
  })
  .superRefine((values, ctx) => {
    const schemes = parseStringList(values.allowedSchemesText);
    if (schemes.length === 0) {
      ctx.addIssue({
        code: "custom",
        path: ["allowedSchemesText"],
        message: "协议列表不能为空",
      });
    }
    for (const scheme of schemes) {
      if (scheme !== "http" && scheme !== "https") {
        ctx.addIssue({
          code: "custom",
          path: ["allowedSchemesText"],
          message: "仅允许 http / https",
        });
        break;
      }
    }
    if (values.resetToken && values.token.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["token"],
        message: "请输入新的 Token",
      });
    }
  });

export type AudioConfigFormValues = z.infer<typeof audioConfigFormSchema>;

/**
 * DTO → 表单默认值。
 *
 * 参数:
 *   - config: GET /api/config/audio 安全 DTO；缺省用安全默认
 *
 * 返回值:
 *   - AudioConfigFormValues
 */
export function defaultAudioConfigFormValues(
  config?: AudioConfig | null,
): AudioConfigFormValues {
  return {
    autoplayEnabled: config?.autoplay.enabled ?? true,
    autoplayTarget:
      config?.autoplay.target === "frontend" ? "frontend" : "backend",
    queueLimit: config?.playback.queueLimit ?? 20,
    maxBytes: config?.inbound.maxBytes ?? 209_715_200,
    allowedSchemesText: (config?.inbound.url.allowedSchemes ?? ["https"]).join(
      ", ",
    ),
    allowPrivateNetworks: config?.inbound.url.allowPrivateNetworks ?? false,
    allowedHostsText: (config?.inbound.url.allowedHosts ?? []).join(", "),
    downloadTimeoutSeconds: config?.inbound.url.downloadTimeoutSeconds ?? 60,
    maxRedirects: config?.inbound.url.maxRedirects ?? 3,
    historyLimit: config?.history.limit ?? 50,
    transcodeTimeoutSeconds: config?.ffmpeg.transcodeTimeoutSeconds ?? 120,
    resetToken: false,
    token: "",
  };
}

/** RHF dirtyFields 的宽松形状（仅关心叶子 dirty）。 */
export type AudioConfigDirtyFields = {
  [K in keyof AudioConfigFormValues]?: boolean | object;
};

/**
 * 表单值 → 部分更新 payload（仅 dirty 字段；token 仅 reset 时）。
 *
 * 参数:
 *   - values: 表单值
 *   - dirtyFields: RHF dirtyFields
 *
 * 返回值:
 *   - AudioConfigWriteRequest
 */
export function toAudioConfigWritePayload(
  values: AudioConfigFormValues,
  dirtyFields: AudioConfigDirtyFields,
): AudioConfigWriteRequest {
  const payload: AudioConfigWriteRequest = {};
  const dirty = (key: keyof AudioConfigFormValues) => Boolean(dirtyFields[key]);

  if (dirty("autoplayEnabled") || dirty("autoplayTarget")) {
    payload.autoplay = {};
    if (dirty("autoplayEnabled")) {
      payload.autoplay.enabled = values.autoplayEnabled;
    }
    if (dirty("autoplayTarget")) {
      payload.autoplay.target = values.autoplayTarget;
    }
  }

  if (dirty("queueLimit")) {
    payload.playback = { queueLimit: values.queueLimit };
  }

  const inbound: NonNullable<AudioConfigWriteRequest["inbound"]> = {};
  let hasInbound = false;

  if (dirty("maxBytes")) {
    inbound.maxBytes = values.maxBytes;
    hasInbound = true;
  }
  if (values.resetToken && values.token.trim() !== "") {
    inbound.token = values.token.trim();
    hasInbound = true;
  }

  const url: NonNullable<
    NonNullable<AudioConfigWriteRequest["inbound"]>["url"]
  > = {};
  let hasUrl = false;
  if (dirty("allowedSchemesText")) {
    url.allowedSchemes = parseStringList(values.allowedSchemesText);
    hasUrl = true;
  }
  if (dirty("allowPrivateNetworks")) {
    url.allowPrivateNetworks = values.allowPrivateNetworks;
    hasUrl = true;
  }
  if (dirty("allowedHostsText")) {
    url.allowedHosts = parseStringList(values.allowedHostsText);
    hasUrl = true;
  }
  if (dirty("downloadTimeoutSeconds")) {
    url.downloadTimeoutSeconds = values.downloadTimeoutSeconds;
    hasUrl = true;
  }
  if (dirty("maxRedirects")) {
    url.maxRedirects = values.maxRedirects;
    hasUrl = true;
  }
  if (hasUrl) {
    inbound.url = url;
    hasInbound = true;
  }
  if (hasInbound) {
    payload.inbound = inbound;
  }

  if (dirty("historyLimit")) {
    payload.history = { limit: values.historyLimit };
  }

  if (dirty("transcodeTimeoutSeconds")) {
    payload.ffmpeg = {
      transcodeTimeoutSeconds: values.transcodeTimeoutSeconds,
    };
  }

  return payload;
}

/**
 * 判断 payload 是否为空（无任何可提交字段）。
 *
 * 参数:
 *   - payload: 写入请求
 *
 * 返回值:
 *   - boolean
 */
export function isEmptyAudioConfigPayload(
  payload: AudioConfigWriteRequest,
): boolean {
  return Object.keys(payload).length === 0;
}

/**
 * 解析逗号/空白分隔列表。
 *
 * 参数:
 *   - value: 原始文本
 *
 * 返回值:
 *   - string[]
 */
export function parseStringList(value: string): string[] {
  return value
    .split(/[,\s]+/)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean);
}
