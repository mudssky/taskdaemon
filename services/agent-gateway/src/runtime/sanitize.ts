/**
 * Secret sanitize 不变量：所有 adapter 出口必须剥离密钥。
 * 这不是 capabilities 位，是硬约束。
 */

const SENSITIVE_KEY_RE =
  /^(authorization|api[_-]?key|token|secret|password|passwd|access[_-]?token|refresh[_-]?token|private[_-]?key|credential|x-api-key)$/i;

/** 脱敏占位符。 */
export const REDACTED = "***REDACTED***";

/**
 * 判断字段名是否敏感。
 *
 * 参数:
 *   - key: 对象键名。
 *
 * 返回值:
 *   - 是否应剥离。
 */
export function isSensitiveKey(key: string): boolean {
  return SENSITIVE_KEY_RE.test(key);
}

/**
 * 深度剥离敏感字段。
 * - 匹配敏感 key 的值替换为 REDACTED
 * - 对 `headers` 对象：若无法白名单，整段替换为 REDACTED
 * - 不修改入参（返回新值）
 *
 * 参数:
 *   - value: 任意 JSON 可序列化值。
 *   - depth: 递归深度上限（防环/过深）。
 *
 * 返回值:
 *   - 消毒后的值。
 */
export function sanitizeSecrets<T>(value: T, depth = 12): T {
  if (depth <= 0 || value === null || value === undefined) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => sanitizeSecrets(item, depth - 1)) as T;
  }
  if (typeof value !== "object") {
    return value;
  }

  const input = value as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [key, child] of Object.entries(input)) {
    if (isSensitiveKey(key)) {
      out[key] = REDACTED;
      continue;
    }
    // provider headers 整段视为敏感（G0：OMP get_state 可含 Authorization）
    if (key === "headers" && child && typeof child === "object") {
      out[key] = REDACTED;
      continue;
    }
    out[key] = sanitizeSecrets(child, depth - 1);
  }
  return out as T;
}

/**
 * 从可能含密钥的 model 对象构造安全 ModelInfo 字段。
 *
 * 参数:
 *   - raw: runtime 原始 model。
 *
 * 返回值:
 *   - 消毒后的 { id, name?, provider?, metadata? }。
 */
export function sanitizeModelInfo(raw: unknown): {
  id: string;
  name?: string;
  provider?: string;
  metadata?: Record<string, unknown>;
} {
  if (!raw || typeof raw !== "object") {
    return { id: "unknown" };
  }
  const m = raw as Record<string, unknown>;
  const id =
    typeof m.id === "string"
      ? m.id
      : typeof m.name === "string"
        ? m.name
        : "unknown";
  const name = typeof m.name === "string" ? m.name : undefined;
  const provider = typeof m.provider === "string" ? m.provider : undefined;
  const safe = sanitizeSecrets(m);
  const {
    id: _i,
    name: _n,
    provider: _p,
    headers: _h,
    ...rest
  } = safe as Record<string, unknown>;
  const metadata =
    Object.keys(rest).length > 0
      ? (rest as Record<string, unknown>)
      : undefined;
  return {
    id,
    ...(name !== undefined ? { name } : {}),
    ...(provider !== undefined ? { provider } : {}),
    ...(metadata !== undefined ? { metadata } : {}),
  };
}
