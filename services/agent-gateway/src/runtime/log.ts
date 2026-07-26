/**
 * Adapter 侧日志：强制脱敏，支持 traceId 透传。
 */

import { sanitizeSecrets } from "./sanitize.js";

export type LogLevel = "debug" | "info" | "warn" | "error";

export type RuntimeLogFields = {
  traceId?: string;
  runtimeId?: string;
  sessionId?: string;
  [key: string]: unknown;
};

export type RuntimeLogger = {
  /**
   * 写一条日志。
   *
   * 参数:
   *   - level: 级别。
   *   - message: 摘要（不得写 prompt 全文）。
   *   - fields: 结构化字段（自动脱敏）。
   *
   * 返回值:
   *   - 无。
   */
  log(level: LogLevel, message: string, fields?: RuntimeLogFields): void;
};

/**
 * 创建默认 console logger。
 *
 * 参数:
 *   - defaults: 默认字段（如 traceId）。
 *
 * 返回值:
 *   - RuntimeLogger。
 */
export function createRuntimeLogger(
  defaults: RuntimeLogFields = {},
): RuntimeLogger {
  return {
    log(level, message, fields) {
      const payload = sanitizeSecrets({
        ...defaults,
        ...fields,
        msg: message,
        level,
        t: new Date().toISOString(),
      });
      const line = JSON.stringify(payload);
      if (level === "error") {
        console.error(line);
        return;
      }
      if (level === "warn") {
        console.warn(line);
        return;
      }
      // info/debug 默认 stdout；避免打印 prompt
      console.log(line);
    },
  };
}

/**
 * 绑定 traceId 的子 logger。
 *
 * 参数:
 *   - parent: 父 logger。
 *   - fields: 追加字段。
 *
 * 返回值:
 *   - RuntimeLogger。
 */
export function withLogFields(
  parent: RuntimeLogger,
  fields: RuntimeLogFields,
): RuntimeLogger {
  return {
    log(level, message, extra) {
      parent.log(level, message, { ...fields, ...extra });
    },
  };
}
