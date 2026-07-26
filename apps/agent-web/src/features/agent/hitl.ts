/**
 * HITL 状态机辅助（纯函数）。
 */

import type {
  HitlDecision,
  HitlRequestPayload,
} from "@taskdaemon/agent-protocol";

export type HitlUiPhase = "none" | "awaiting" | "resolved" | "timed_out";

/**
 * 由 pending id 与条目状态推导 UI 阶段。
 *
 * 参数:
 *   - pendingHitlId: 当前等待的 requestId。
 *   - itemStatus: 时间线条目状态。
 *
 * 返回值:
 *   - HitlUiPhase。
 */
export function hitlPhase(
  pendingHitlId: string | undefined,
  itemStatus?: "awaiting" | "approved" | "rejected" | "modified" | "timed_out",
): HitlUiPhase {
  if (itemStatus === "timed_out") {
    return "timed_out";
  }
  if (
    itemStatus === "approved" ||
    itemStatus === "rejected" ||
    itemStatus === "modified"
  ) {
    return "resolved";
  }
  if (pendingHitlId || itemStatus === "awaiting") {
    return "awaiting";
  }
  return "none";
}

/**
 * 校验 HITL 响应是否可提交。
 *
 * 参数:
 *   - request: 请求载荷。
 *   - decision: 决策。
 *   - modifiedText: modify 文本。
 *   - nowMs: 当前时间。
 *   - startedAtMs: 收到请求时间（可选）。
 *
 * 返回值:
 *   - ok 与错误原因。
 */
export function validateHitlResponse(
  request: HitlRequestPayload,
  decision: HitlDecision,
  modifiedText: string | undefined,
  nowMs: number,
  startedAtMs?: number,
): { ok: true } | { ok: false; reason: string } {
  if (
    request.timeoutMs !== undefined &&
    startedAtMs !== undefined &&
    nowMs - startedAtMs > request.timeoutMs
  ) {
    return { ok: false, reason: "HITL 已超时" };
  }
  if (decision === "modify" && !modifiedText?.trim()) {
    return { ok: false, reason: "修改后继续需要输入文本" };
  }
  return { ok: true };
}

/**
 * 运行态文案：等待人工时必须显眼。
 *
 * 参数:
 *   - status: stream status。
 *
 * 返回值:
 *   - 中文标签。
 */
export function runStatusLabel(
  status: "idle" | "running" | "error" | "cancelled" | "awaiting_human",
): string {
  switch (status) {
    case "idle":
      return "空闲";
    case "running":
      return "运行中";
    case "error":
      return "出错";
    case "cancelled":
      return "已中断";
    case "awaiting_human":
      return "等待您确认";
    default:
      return status;
  }
}
