/**
 * PrincipalResolver 实现：dev-single-principal / gateway-headers。
 */

import { randomUUID } from "node:crypto";
import {
  DEV_SINGLE_PRINCIPAL,
  type Principal,
  type PrincipalResolver,
  type PrincipalResolverMode,
  TRUST_HEADERS,
} from "@taskdaemon/agent-protocol";
import { AgentHttpError } from "../errors.js";

/**
 * 规范化头映射为小写键。
 *
 * 参数:
 *   - headers: 原始头。
 *
 * 返回值:
 *   - 小写键映射。
 */
export function normalizeHeaders(
  headers: Record<string, string | undefined> | Headers,
): Record<string, string | undefined> {
  const out: Record<string, string | undefined> = {};
  if (headers instanceof Headers) {
    headers.forEach((value, key) => {
      out[key.toLowerCase()] = value;
    });
    return out;
  }
  for (const [key, value] of Object.entries(headers)) {
    out[key.toLowerCase()] = value;
  }
  return out;
}

/**
 * 读取或生成 traceId；禁止覆盖已有值。
 *
 * 参数:
 *   - headers: 小写头。
 *
 * 返回值:
 *   - traceId。
 */
export function resolveTraceId(
  headers: Record<string, string | undefined>,
): string {
  const existing = headers[TRUST_HEADERS.traceId]?.trim();
  return existing && existing.length > 0 ? existing : randomUUID();
}

/** 开发默认解析器。 */
export class DevSinglePrincipalResolver implements PrincipalResolver {
  /**
   * 参数:
   *   - headers: 小写头映射。
   * 返回值:
   *   - Principal（固定 subject/tenant，trace 透传或生成）。
   */
  async resolve(
    headers: Record<string, string | undefined>,
  ): Promise<Principal> {
    const normalized = normalizeHeaders(headers);
    return {
      ...DEV_SINGLE_PRINCIPAL,
      // 开发期仍允许头覆盖 subject/tenant 便于联调
      subject:
        normalized[TRUST_HEADERS.subject]?.trim() ||
        DEV_SINGLE_PRINCIPAL.subject,
      tenantId:
        normalized[TRUST_HEADERS.tenantId]?.trim() ||
        DEV_SINGLE_PRINCIPAL.tenantId,
      traceId: resolveTraceId(normalized),
      source: "dev-single-principal",
    };
  }
}

/** 生产：强制注入头。 */
export class GatewayHeadersPrincipalResolver implements PrincipalResolver {
  /**
   * 参数:
   *   - headers: 小写头映射。
   * 返回值:
   *   - Principal。
   * 异常:
   *   - 缺 subject → AGENT_UNAUTHORIZED。
   */
  async resolve(
    headers: Record<string, string | undefined>,
  ): Promise<Principal> {
    const normalized = normalizeHeaders(headers);
    const subject = normalized[TRUST_HEADERS.subject]?.trim();
    if (!subject) {
      throw new AgentHttpError("AGENT_UNAUTHORIZED", "missing x-auth-subject", {
        traceId: resolveTraceId(normalized),
      });
    }
    const tenantId = normalized[TRUST_HEADERS.tenantId]?.trim();
    if (!tenantId) {
      throw new AgentHttpError("AGENT_UNAUTHORIZED", "missing x-tenant-id", {
        traceId: resolveTraceId(normalized),
      });
    }
    return {
      subject,
      tenantId,
      traceId: resolveTraceId(normalized),
      source: "gateway-headers",
    };
  }
}

/**
 * 按模式创建 PrincipalResolver。
 *
 * 参数:
 *   - mode: 解析模式。
 *
 * 返回值:
 *   - PrincipalResolver。
 */
export function createPrincipalResolver(
  mode: PrincipalResolverMode,
): PrincipalResolver {
  return mode === "gateway-headers"
    ? new GatewayHeadersPrincipalResolver()
    : new DevSinglePrincipalResolver();
}
