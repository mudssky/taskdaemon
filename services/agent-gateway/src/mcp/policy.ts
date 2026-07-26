/**
 * MCP tool 策略：复用 G2 allowlist 字符串列表，不重构框架。
 * 粒度：tool 级；支持 `server/tool` 与 `server/*`。
 */

import { AgentHttpError } from "../errors.js";
import { assertToolAllowed } from "../policy/policy.js";
import type { McpCatalog } from "./catalog.js";
import { splitQualified } from "./catalog.js";

/**
 * 校验工具是否允许调用。
 * - MCP 工具越界 → MCP_TOOL_NOT_ALLOWED
 * - 非 MCP 工具 → 复用 assertToolAllowed（AGENT_FORBIDDEN）
 *
 * 参数:
 *   - toolName: 工具名（可为 server/tool）。
 *   - allowlist: gateway toolAllowlist。
 *   - sessionTools: 会话 tools。
 *   - catalog: MCP 目录。
 *   - tenantId: 租户。
 *
 * 返回值:
 *   - void。
 * 异常:
 *   - 策略拒绝时抛 AgentHttpError。
 */
export function assertGatewayToolAllowed(
  toolName: string,
  allowlist: string[],
  sessionTools: "none" | string[] | undefined,
  catalog: McpCatalog,
  tenantId: string,
): void {
  if (sessionTools === "none") {
    throw new AgentHttpError(
      catalog.isMcpTool(toolName, tenantId)
        ? "MCP_TOOL_NOT_ALLOWED"
        : "AGENT_FORBIDDEN",
      `tool "${toolName}" denied: session tools=none`,
      { details: { toolName } },
    );
  }

  if (sessionTools && Array.isArray(sessionTools)) {
    if (
      !sessionTools.includes(toolName) &&
      !sessionMatches(sessionTools, toolName)
    ) {
      throw new AgentHttpError(
        catalog.isMcpTool(toolName, tenantId)
          ? "MCP_TOOL_NOT_ALLOWED"
          : "AGENT_FORBIDDEN",
        `tool "${toolName}" denied by session tools`,
        { details: { toolName } },
      );
    }
  }

  const isMcp = catalog.isMcpTool(toolName, tenantId);
  if (isMcp) {
    if (!isAllowlisted(toolName, allowlist, catalog, tenantId)) {
      throw new AgentHttpError(
        "MCP_TOOL_NOT_ALLOWED",
        `MCP tool "${toolName}" denied by gateway allowlist`,
        { details: { toolName, allowlist } },
      );
    }
    return;
  }

  // 非 MCP：原 G2 框架
  assertToolAllowed(toolName, allowlist, sessionTools);
}

/**
 * 判断 allowlist 是否放行。
 *
 * 参数:
 *   - toolName: 工具名。
 *   - allowlist: 允许列表。
 *   - catalog: 目录。
 *   - tenantId: 租户。
 *
 * 返回值:
 *   - true 表示允许。
 */
export function isAllowlisted(
  toolName: string,
  allowlist: string[],
  catalog: McpCatalog,
  tenantId: string,
): boolean {
  if (allowlist.length === 0) {
    return false;
  }
  const qualified = catalog.qualifyToolName(toolName, tenantId);
  const [serverId, name] = splitQualified(
    qualified.includes("/") ? qualified : `/${toolName}`,
  );
  for (const entry of allowlist) {
    if (entry === toolName || entry === qualified) return true;
    if (entry === `${serverId}/*` && serverId) return true;
    if (entry === name && name) return true;
    if (entry.endsWith(`/${name}`) && name) return true;
  }
  return false;
}

/**
 * 会话 tools 是否匹配（含 server/*）。
 *
 * 参数:
 *   - sessionTools: 会话允许列表。
 *   - toolName: 工具名。
 *
 * 返回值:
 *   - true 表示匹配。
 */
function sessionMatches(sessionTools: string[], toolName: string): boolean {
  if (sessionTools.includes(toolName)) return true;
  if (!toolName.includes("/")) return false;
  const [serverId] = splitQualified(toolName);
  return sessionTools.includes(`${serverId}/*`);
}
