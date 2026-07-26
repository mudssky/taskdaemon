import { z } from "zod";
import {
  type NotificationListParams,
  notificationSeverities,
} from "../../lib/api/types";

/** 已读筛选：全部 / 未读 / 已读。 */
export const notificationReadFilters = ["all", "unread", "read"] as const;
export type NotificationReadFilter = (typeof notificationReadFilters)[number];

const DEFAULT_PAGE = 1;
const DEFAULT_PAGE_SIZE = 20;
const MAX_PAGE_SIZE = 100;

/** 列表页 URL search 规范化结果。 */
export type NotificationSearchParams = {
  page: number;
  pageSize: number;
  read: NotificationReadFilter;
  severity: (typeof notificationSeverities)[number] | "all";
};

/** 通知列表默认 URL 状态。 */
export const DEFAULT_NOTIFICATION_SEARCH: NotificationSearchParams = {
  page: DEFAULT_PAGE,
  pageSize: DEFAULT_PAGE_SIZE,
  read: "all",
  severity: "all",
};

const notificationSearchSchema = z.object({
  page: z.coerce.number().int().positive().catch(DEFAULT_PAGE),
  pageSize: z.coerce
    .number()
    .int()
    .positive()
    .max(MAX_PAGE_SIZE)
    .catch(DEFAULT_PAGE_SIZE),
  read: z.enum(notificationReadFilters).catch("all"),
  severity: z.enum([...notificationSeverities, "all"] as const).catch("all"),
});

/**
 * 解析并兜底通知列表 URL search 参数。
 *
 * 参数:
 *   - raw: 路由 search 原始对象（字段可为 string/number/unknown）。
 *
 * 返回值:
 *   - NotificationSearchParams: 规范化后的筛选与分页。
 */
export function parseNotificationSearch(
  raw: Record<string, unknown>,
): NotificationSearchParams {
  const parsed = notificationSearchSchema.parse({
    page: raw.page ?? DEFAULT_PAGE,
    pageSize: raw.pageSize ?? DEFAULT_PAGE_SIZE,
    read: raw.read ?? "all",
    severity: raw.severity ?? "all",
  });
  return {
    page: parsed.page,
    pageSize: parsed.pageSize,
    read: parsed.read,
    severity: parsed.severity,
  };
}

/**
 * 将 URL 筛选状态映射为 API 列表查询参数。
 *
 * 参数:
 *   - search: 规范化后的 URL 筛选。
 *
 * 返回值:
 *   - NotificationListParams: 发给 /api/notifications 的 query。
 */
export function toNotificationListParams(
  search: NotificationSearchParams,
): NotificationListParams {
  const params: NotificationListParams = {
    page: search.page,
    pageSize: search.pageSize,
  };
  if (search.read === "unread") {
    params.read = false;
  } else if (search.read === "read") {
    params.read = true;
  }
  if (search.severity !== "all") {
    params.severity = search.severity;
  }
  return params;
}

/**
 * 判断当前筛选是否为“有筛选条件”。
 *
 * 参数:
 *   - search: 规范化后的 URL 筛选。
 *
 * 返回值:
 *   - boolean: 存在已读或严重级别筛选时为 true。
 */
export function hasNotificationFilters(
  search: NotificationSearchParams,
): boolean {
  return search.read !== "all" || search.severity !== "all";
}
