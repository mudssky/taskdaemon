import {
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { apiClient } from "../../lib/api/client";
import type { NotificationListParams } from "../../lib/api/types";

/** 未读计数轮询间隔（毫秒）。 */
export const NOTIFICATION_UNREAD_REFETCH_INTERVAL_MS = 15_000;

/** 快览条数。 */
export const NOTIFICATION_PREVIEW_PAGE_SIZE = 5;

/** 路线图 §9.2 分配的 query key 前缀。 */
export const notificationKeys = {
  all: ["notifications"] as const,
  unreadCount: ["notifications", "unread-count"] as const,
  preview: ["notifications", "preview"] as const,
  list: (params: NotificationListParams) =>
    ["notifications", "list", params] as const,
};

/**
 * 根据页面可见性解析未读轮询间隔。
 * 页面不可见时停止轮询。
 *
 * 参数:
 *   - visible: document 是否可见。
 *
 * 返回值:
 *   - number | false: 轮询间隔或 false（停止）。
 */
export function resolveUnreadRefetchInterval(visible: boolean): number | false {
  return visible ? NOTIFICATION_UNREAD_REFETCH_INTERVAL_MS : false;
}

/**
 * 订阅 document.visibilityState。
 *
 * 返回值:
 *   - boolean: 当前是否可见。
 */
function useDocumentVisible(): boolean {
  const [visible, setVisible] = useState(() => {
    if (typeof document === "undefined") {
      return true;
    }
    return document.visibilityState === "visible";
  });

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    const onChange = () => {
      setVisible(document.visibilityState === "visible");
    };
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);

  return visible;
}

/**
 * 失效未读计数、列表、快览三处 cache。
 *
 * 参数:
 *   - queryClient: TanStack QueryClient。
 *
 * 返回值:
 *   - Promise<void>
 */
export async function invalidateNotificationCaches(
  queryClient: QueryClient,
): Promise<void> {
  await queryClient.invalidateQueries({ queryKey: notificationKeys.all });
}

/**
 * 未读计数 query；页面不可见时停止轮询。
 *
 * 返回值:
 *   - UseQueryResult<{ count: number }>
 */
export function useUnreadCountQuery() {
  const visible = useDocumentVisible();
  return useQuery({
    queryKey: notificationKeys.unreadCount,
    queryFn: apiClient.notificationUnreadCount,
    refetchInterval: resolveUnreadRefetchInterval(visible),
  });
}

/**
 * 铃铛快览最近通知。
 *
 * 返回值:
 *   - UseQueryResult<NotificationListResponse>
 */
export function useNotificationPreviewQuery(enabled = true) {
  return useQuery({
    queryKey: notificationKeys.preview,
    queryFn: () =>
      apiClient.listNotifications({
        page: 1,
        pageSize: NOTIFICATION_PREVIEW_PAGE_SIZE,
      }),
    enabled,
  });
}

/**
 * 通知列表 query。
 *
 * 参数:
 *   - params: 规范化后的列表筛选。
 *
 * 返回值:
 *   - UseQueryResult<NotificationListResponse>
 */
export function useNotificationListQuery(params: NotificationListParams) {
  return useQuery({
    queryKey: notificationKeys.list(params),
    queryFn: () => apiClient.listNotifications(params),
  });
}

/**
 * 标记单条已读。
 *
 * 返回值:
 *   - UseMutationResult
 */
export function useMarkNotificationReadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => apiClient.markNotificationRead(id),
    onSuccess: async () => {
      await invalidateNotificationCaches(queryClient);
    },
  });
}

/**
 * 批量标记已读。
 *
 * 返回值:
 *   - UseMutationResult
 */
export function useMarkNotificationsReadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: number[]) => apiClient.markNotificationsRead(ids),
    onSuccess: async () => {
      await invalidateNotificationCaches(queryClient);
    },
  });
}

/**
 * 全部标记已读。
 *
 * 返回值:
 *   - UseMutationResult
 */
export function useMarkAllNotificationsReadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.markAllNotificationsRead(),
    onSuccess: async () => {
      await invalidateNotificationCaches(queryClient);
    },
  });
}

/**
 * 删除单条通知。
 *
 * 返回值:
 *   - UseMutationResult
 */
export function useDeleteNotificationMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => apiClient.deleteNotification(id),
    onSuccess: async () => {
      await invalidateNotificationCaches(queryClient);
    },
  });
}

/**
 * 清空已读通知。
 *
 * 返回值:
 *   - UseMutationResult
 */
export function useClearReadNotificationsMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.clearReadNotifications(),
    onSuccess: async () => {
      await invalidateNotificationCaches(queryClient);
    },
  });
}
