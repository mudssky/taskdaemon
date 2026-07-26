/**
 * MCP tool 入参脱敏：audit 事件不得含敏感明文。
 */

const SENSITIVE_KEY =
  /^(password|passwd|secret|token|access_token|refresh_token|authorization|api[_-]?key|cookie|session)$/i;

/**
 * 对 tool 入参做摘要脱敏。
 *
 * 参数:
 *   - args: 原始入参。
 *   - maxDepth: 最大递归深度，默认 3。
 *
 * 返回值:
 *   - 可写入 audit.metadata 的脱敏摘要。
 */
export function redactToolArgs(
  args: unknown,
  maxDepth = 3,
): Record<string, unknown> | string | number | boolean | null {
  return redactValue(args, maxDepth) as
    | Record<string, unknown>
    | string
    | number
    | boolean
    | null;
}

/**
 * 递归脱敏。
 *
 * 参数:
 *   - value: 任意值。
 *   - depth: 剩余深度。
 *
 * 返回值:
 *   - 脱敏后的值。
 */
function redactValue(value: unknown, depth: number): unknown {
  if (value === null || value === undefined) {
    return null;
  }
  if (depth <= 0) {
    return "[truncated]";
  }
  if (
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean"
  ) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.slice(0, 20).map((item) => redactValue(item, depth - 1));
  }
  if (typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [key, nested] of Object.entries(
      value as Record<string, unknown>,
    )) {
      if (SENSITIVE_KEY.test(key)) {
        out[key] = "[REDACTED]";
      } else {
        out[key] = redactValue(nested, depth - 1);
      }
    }
    return out;
  }
  return String(value);
}
