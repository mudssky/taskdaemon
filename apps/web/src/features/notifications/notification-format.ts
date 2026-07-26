import type { Notification, NotificationSeverity } from "../../lib/api/types";
import { dayjs } from "../../lib/dayjs";

/** 角标显示上限。 */
export const UNREAD_BADGE_CAP = 99;

/**
 * 格式化未读角标文本；0 返回 null（不展示）。
 *
 * 参数:
 *   - count: 未读数量。
 *
 * 返回值:
 *   - string | null: 角标文本；为 0 时 null。
 */
export function formatUnreadBadge(count: number): string | null {
  if (!Number.isFinite(count) || count <= 0) {
    return null;
  }
  if (count > UNREAD_BADGE_CAP) {
    return `${UNREAD_BADGE_CAP}+`;
  }
  return String(Math.floor(count));
}

/**
 * 严重级别中文标签。
 *
 * 参数:
 *   - severity: 通知严重级别。
 *
 * 返回值:
 *   - string: 展示文案。
 */
export function severityLabel(severity: string): string {
  switch (severity as NotificationSeverity) {
    case "info":
      return "信息";
    case "warning":
      return "警告";
    case "error":
      return "错误";
    case "critical":
      return "严重";
    default:
      return severity;
  }
}

/**
 * 严重级别 badge class。
 *
 * 参数:
 *   - severity: 通知严重级别。
 *
 * 返回值:
 *   - string: CSS class 名。
 */
export function severityBadgeClass(severity: string): string {
  switch (severity as NotificationSeverity) {
    case "info":
      return "badge";
    case "warning":
      return "badge cancelled";
    case "error":
      return "badge failed";
    case "critical":
      return "badge failed";
    default:
      return "badge";
  }
}

/**
 * 格式化通知发生时间。
 *
 * 参数:
 *   - iso: ISO 时间字符串。
 *
 * 返回值:
 *   - string: 本地化展示文本。
 */
export function formatNotificationTime(iso: string): string {
  const value = dayjs(iso);
  if (!value.isValid()) {
    return iso;
  }
  return value.format("YYYY-MM-DD HH:mm:ss");
}

export type NotificationEmptyKind = "never" | "filtered";

/**
 * 判定空状态文案类型。
 *
 * 参数:
 *   - total: 当前查询 total。
 *   - filtered: 是否带筛选条件。
 *
 * 返回值:
 *   - NotificationEmptyKind | null: 有数据时 null。
 */
export function resolveNotificationEmptyKind(
  total: number,
  filtered: boolean,
): NotificationEmptyKind | null {
  if (total > 0) {
    return null;
  }
  return filtered ? "filtered" : "never";
}

/**
 * 空状态文案。
 *
 * 参数:
 *   - kind: 空状态类型。
 *
 * 返回值:
 *   - string: 用户可见文案。
 */
export function notificationEmptyMessage(kind: NotificationEmptyKind): string {
  if (kind === "filtered") {
    return "当前筛选条件下没有通知。";
  }
  return "还没有任何站内通知。";
}

export type NotificationSubjectLink =
  | {
      available: true;
      target: { type: "task"; taskId: number } | { type: "runs" };
      label: string;
    }
  | {
      available: false;
      label: string;
      reason: string;
    };

/**
 * 解析通知关联实体跳转。
 * 任务不存在时禁用入口，避免落到 404。
 *
 * 参数:
 *   - notification: 通知 DTO。
 *   - existingTaskIds: 当前任务列表中的 ID 集合。
 *
 * 返回值:
 *   - NotificationSubjectLink | null: 无可跳转主体时 null。
 */
export function resolveSubjectLink(
  notification: Notification,
  existingTaskIds: ReadonlySet<number>,
): NotificationSubjectLink | null {
  const kind = notification.subjectKind;
  if (!kind || kind === "scheduler") {
    return null;
  }

  if (kind === "task") {
    const taskId = Number(notification.subjectId);
    if (!Number.isInteger(taskId) || taskId <= 0) {
      return {
        available: false,
        label: "关联任务",
        reason: "关联任务标识无效",
      };
    }
    if (!existingTaskIds.has(taskId)) {
      return {
        available: false,
        label: `任务 #${taskId}`,
        reason: "关联任务已删除",
      };
    }
    return {
      available: true,
      target: { type: "task", taskId },
      label: `任务 #${taskId}`,
    };
  }

  if (kind === "run") {
    const runId = notification.subjectId?.trim() || "";
    const detailTaskId = Number(notification.detail?.taskId);
    const label = runId ? `运行 #${runId}` : "运行记录";
    if (Number.isInteger(detailTaskId) && detailTaskId > 0) {
      if (!existingTaskIds.has(detailTaskId)) {
        return {
          available: false,
          label,
          reason: "关联任务已删除",
        };
      }
    }
    return {
      available: true,
      target: { type: "runs" },
      label,
    };
  }

  return null;
}

/**
 * 判断通知是否已读。
 *
 * 参数:
 *   - notification: 通知 DTO。
 *
 * 返回值:
 *   - boolean: 已读为 true。
 */
export function isNotificationRead(notification: Notification): boolean {
  return Boolean(notification.readAt);
}
