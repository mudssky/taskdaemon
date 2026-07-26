/**
 * 策略：tool allowlist 与 workspace 沙箱。
 */

import path from "node:path";
import { AgentHttpError } from "../errors.js";

/**
 * 校验 tool 是否在 allowlist 内。
 *
 * 参数:
 *   - toolName: 工具名。
 *   - allowlist: 允许列表；空 = 全部拒绝具名 tool。
 *   - sessionTools: 会话 tools 配置；"none" 表示不期望 tool。
 *
 * 返回值:
 *   - void。
 * 异常:
 *   - 越界 → AGENT_FORBIDDEN。
 */
export function assertToolAllowed(
  toolName: string,
  allowlist: string[],
  sessionTools?: "none" | string[],
): void {
  if (sessionTools === "none") {
    throw new AgentHttpError(
      "AGENT_FORBIDDEN",
      `tool "${toolName}" denied: session tools=none`,
      { details: { toolName } },
    );
  }
  if (sessionTools && Array.isArray(sessionTools)) {
    if (!sessionTools.includes(toolName)) {
      throw new AgentHttpError(
        "AGENT_FORBIDDEN",
        `tool "${toolName}" denied by session tools`,
        { details: { toolName } },
      );
    }
  }
  if (!allowlist.includes(toolName)) {
    throw new AgentHttpError(
      "AGENT_FORBIDDEN",
      `tool "${toolName}" denied by gateway allowlist`,
      { details: { toolName, allowlist } },
    );
  }
}

/**
 * 校验 workspace 路径落在沙箱根内。
 * workspaceBinding=false 时跳过。
 *
 * 参数:
 *   - workspaceRoot: 请求路径（可为 null）。
 *   - sandboxRoot: 配置沙箱根。
 *   - workspaceBinding: 是否绑定 workspace。
 *
 * 返回值:
 *   - 规范化后的绝对路径；未绑定时 null。
 * 异常:
 *   - 路径穿越 / 越界 → AGENT_FORBIDDEN。
 */
export function resolveWorkspaceRoot(
  workspaceRoot: string | null | undefined,
  sandboxRoot: string,
  workspaceBinding: boolean,
): string | null {
  if (!workspaceBinding) {
    return null;
  }
  const sandbox = path.resolve(sandboxRoot);
  const target = path.resolve(workspaceRoot ?? sandbox);
  const relative = path.relative(sandbox, target);
  if (
    relative.startsWith("..") ||
    path.isAbsolute(relative) ||
    relative.includes(`..${path.sep}`)
  ) {
    throw new AgentHttpError(
      "AGENT_FORBIDDEN",
      "workspace path escapes sandbox root",
      { details: { workspaceRoot: target, sandboxRoot: sandbox } },
    );
  }
  // 额外拒绝明显的穿越片段
  if (
    (workspaceRoot ?? "").includes("..") ||
    (workspaceRoot ?? "").includes("\0")
  ) {
    throw new AgentHttpError(
      "AGENT_FORBIDDEN",
      "workspace path traversal rejected",
      { details: { workspaceRoot } },
    );
  }
  return target;
}
