import type {
  AudioConfig,
  AudioConfigWriteRequest,
  ConfigFieldError,
} from "../../lib/api/types";

/**
 * 判断 ApiClientError.details 是否含字段级错误列表。
 *
 * 参数:
 *   - details: API error.details
 *
 * 返回值:
 *   - ConfigFieldError[]: 字段错误；无法识别时为空数组
 */
export function extractConfigFieldErrors(details: unknown): ConfigFieldError[] {
  if (!details || typeof details !== "object") {
    return [];
  }
  if (!("fields" in details) || !Array.isArray(details.fields)) {
    return [];
  }
  const result: ConfigFieldError[] = [];
  for (const item of details.fields) {
    if (!item || typeof item !== "object") {
      continue;
    }
    if (
      !("path" in item) ||
      !("reason" in item) ||
      !("code" in item) ||
      typeof item.path !== "string" ||
      typeof item.reason !== "string" ||
      typeof item.code !== "string"
    ) {
      continue;
    }
    result.push({
      path: item.path,
      reason: item.reason,
      code: item.code,
    });
  }
  return result;
}

/** API 字段路径 → 表单字段名（RHF）。 */
const apiPathToFormField: Record<string, string> = {
  "audio.autoplay.enabled": "autoplayEnabled",
  "audio.autoplay.target": "autoplayTarget",
  "audio.playback.queueLimit": "queueLimit",
  "audio.inbound.token": "token",
  "audio.inbound.maxBytes": "maxBytes",
  "audio.inbound.url.allowedSchemes": "allowedSchemesText",
  "audio.inbound.url.allowPrivateNetworks": "allowPrivateNetworks",
  "audio.inbound.url.allowedHosts": "allowedHostsText",
  "audio.inbound.url.downloadTimeoutSeconds": "downloadTimeoutSeconds",
  "audio.inbound.url.maxRedirects": "maxRedirects",
  "audio.history.limit": "historyLimit",
  "audio.ffmpeg.transcodeTimeoutSeconds": "transcodeTimeoutSeconds",
};

/**
 * 将服务端字段路径映射到表单字段名。
 *
 * 参数:
 *   - path: 如 audio.playback.queueLimit
 *
 * 返回值:
 *   - string | null: 表单字段名；未知路径返回 null
 */
export function mapConfigPathToFormField(path: string): string | null {
  return apiPathToFormField[path] ?? null;
}

/**
 * 字段错误 reason 的用户可见文案（不依赖 API message 做分支）。
 *
 * 参数:
 *   - error: 字段级错误
 *
 * 返回值:
 *   - string: 中文提示
 */
export function configFieldErrorMessage(error: ConfigFieldError): string {
  switch (error.code) {
    case "CONFIG_FIELD_OUT_OF_RANGE":
      return "取值超出允许范围";
    case "CONFIG_FIELD_INVALID":
      return "字段格式不合法";
    case "CONFIG_FIELD_CONFLICT":
      return "与其他配置冲突";
    case "CONFIG_FIELD_NOT_WRITABLE":
      return "该字段不可通过页面写入";
    default:
      return "字段校验失败";
  }
}

/**
 * 证明敏感值不会被写入 localStorage 的纯函数边界（单测锁定）。
 *
 * 参数:
 *   - token: 明文 token
 *
 * 返回值:
 *   - void
 */
export function assertTokenNotPersisted(token: string): void {
  if (typeof localStorage === "undefined") {
    return;
  }
  for (let i = 0; i < localStorage.length; i += 1) {
    const key = localStorage.key(i);
    if (!key) {
      continue;
    }
    const value = localStorage.getItem(key) ?? "";
    if (value.includes(token)) {
      throw new Error("sensitive token must not enter localStorage");
    }
  }
}

/**
 * 从 AudioConfig 读取 fileOnly 展示项（只读状态，非输入框）。
 *
 * 参数:
 *   - config: 安全配置 DTO
 *
 * 返回值:
 *   - 只读项列表
 */
export function audioFileOnlyDisplayItems(config: AudioConfig): Array<{
  path: string;
  label: string;
  status: string;
}> {
  return [
    {
      path: "ffmpeg.path",
      label: "FFmpeg 路径",
      status: config.ffmpeg.pathConfigured ? "已配置" : "未配置（内置或 PATH）",
    },
    {
      path: "ffmpeg.probePath",
      label: "FFprobe 路径",
      status: config.ffmpeg.probePathConfigured
        ? "已配置"
        : "未配置（内置或 PATH）",
    },
    {
      path: "inbound.tokenHash",
      label: "Token Hash",
      status: config.inbound.tokenConfigured
        ? "已配置（仅 hash，不明文）"
        : "未配置",
    },
  ];
}

/**
 * 是否需要危险确认：开启私网拉取。
 *
 * 参数:
 *   - previous: 保存前配置
 *   - nextAllow: 表单目标值
 *
 * 返回值:
 *   - boolean
 */
export function needsPrivateNetworkConfirm(
  previous: AudioConfig | undefined,
  nextAllow: boolean,
): boolean {
  if (!nextAllow) {
    return false;
  }
  return previous?.inbound.url.allowPrivateNetworks !== true;
}

/**
 * 是否需要危险确认：重置 token。
 *
 * 参数:
 *   - resetToken: 表单是否进入重置
 *   - token: 新 token 明文
 *
 * 返回值:
 *   - boolean
 */
export function needsTokenResetConfirm(
  resetToken: boolean,
  token: string,
): boolean {
  return resetToken && token.trim().length > 0;
}

/** 仅组件内存使用的一次显示 token。 */
export type SensitiveWriteScratch = {
  oneTimeToken: string | null;
};

/**
 * 创建不会进入持久缓存的敏感 scratch 状态。
 *
 * 参数:
 *   - token: 明文
 *
 * 返回值:
 *   - SensitiveWriteScratch
 */
export function createOneTimeTokenScratch(
  token: string | null,
): SensitiveWriteScratch {
  return { oneTimeToken: token && token.length > 0 ? token : null };
}

/**
 * 根据 AudioConfigWriteRequest 判断是否包含 token 明文。
 *
 * 参数:
 *   - payload: 写入请求
 *
 * 返回值:
 *   - boolean
 */
export function payloadContainsToken(
  payload: AudioConfigWriteRequest,
): boolean {
  return typeof payload.inbound?.token === "string";
}
