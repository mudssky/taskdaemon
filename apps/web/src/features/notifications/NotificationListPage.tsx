import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { ApiClientError } from "../../lib/api/client";
import { notificationSeverities } from "../../lib/api/types";
import { DesktopCapability, useDesktopCapability } from "../../lib/desktop";
import { useTasksQuery } from "../tasks/tasks.queries";
import { NotificationTable } from "./NotificationTable";
import {
  hasNotificationFilters,
  type NotificationSearchParams,
  notificationReadFilters,
  parseNotificationSearch,
  toNotificationListParams,
} from "./notification.schema";
import {
  notificationEmptyMessage,
  resolveNotificationEmptyKind,
} from "./notification-format";
import {
  useClearReadNotificationsMutation,
  useDeleteNotificationMutation,
  useMarkAllNotificationsReadMutation,
  useMarkNotificationReadMutation,
  useMarkNotificationsReadMutation,
  useNotificationListQuery,
} from "./notifications.queries";

const notificationsRouteApi = getRouteApi("/notifications");

type ConfirmAction = "mark-all-read" | "clear-read" | null;

/**
 * 通知列表页：筛选、分页、已读管理与关联跳转。
 *
 * 返回值:
 *   - JSX.Element
 */
export function NotificationListPage() {
  const rawSearch = notificationsRouteApi.useSearch();
  const navigate = useNavigate({ from: "/notifications" });
  const search = parseNotificationSearch(rawSearch as Record<string, unknown>);
  const listParams = toNotificationListParams(search);
  const listQuery = useNotificationListQuery(listParams);
  const tasksQuery = useTasksQuery();
  const markRead = useMarkNotificationReadMutation();
  const markBatch = useMarkNotificationsReadMutation();
  const markAll = useMarkAllNotificationsReadMutation();
  const deleteOne = useDeleteNotificationMutation();
  const clearRead = useClearReadNotificationsMutation();
  const desktopNotification = useDesktopCapability(
    DesktopCapability.Notification,
  );
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmAction, setConfirmAction] = useState<ConfirmAction>(null);
  const [actionError, setActionError] = useState<ApiClientError | null>(null);

  const notifications = listQuery.data?.notifications ?? [];
  const total = listQuery.data?.total ?? 0;
  const page = listQuery.data?.page ?? search.page;
  const pageSize = listQuery.data?.pageSize ?? search.pageSize;
  const totalPages = Math.max(1, Math.ceil(total / pageSize) || 1);
  const existingTaskIds = useMemo(
    () => new Set((tasksQuery.data ?? []).map((task) => task.id)),
    [tasksQuery.data],
  );
  const emptyKind = resolveNotificationEmptyKind(
    total,
    hasNotificationFilters(search),
  );
  const busyId =
    markRead.variables ??
    deleteOne.variables ??
    (markBatch.isPending ? -1 : null);

  function patchSearch(patch: Partial<NotificationSearchParams>) {
    const next = { ...search, ...patch };
    void navigate({
      search: {
        page: next.page,
        pageSize: next.pageSize,
        read: next.read,
        severity: next.severity,
      },
      replace: true,
    });
  }

  function onToggleSelect(id: number, checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  function onToggleSelectAll(checked: boolean) {
    if (!checked) {
      setSelectedIds(new Set());
      return;
    }
    setSelectedIds(new Set(notifications.map((item) => item.id)));
  }

  function runMutation(action: () => Promise<unknown>): void {
    setActionError(null);
    void action().catch((error: unknown) => {
      if (error instanceof ApiClientError) {
        setActionError(error);
        return;
      }
      setActionError(
        new ApiClientError(500, "unknown_error", "操作失败", null, null),
      );
    });
  }

  function confirmDestructive() {
    if (confirmAction === "mark-all-read") {
      setConfirmAction(null);
      runMutation(() => markAll.mutateAsync());
      return;
    }
    if (confirmAction === "clear-read") {
      setConfirmAction(null);
      runMutation(() => clearRead.mutateAsync());
    }
  }

  const listError =
    listQuery.error instanceof ApiClientError ? listQuery.error : null;

  return (
    <AlertDialog
      open={confirmAction !== null}
      onOpenChange={(open) => {
        if (!open) {
          setConfirmAction(null);
        }
      }}
    >
      <div className="dashboard-grid">
        <section className="panel" aria-labelledby="notification-filters-title">
          <div className="panel-header">
            <div>
              <h2 id="notification-filters-title">筛选</h2>
              <p>按已读状态与严重级别过滤通知，状态写入 URL。</p>
            </div>
          </div>
          <div className="form-grid">
            <label className="field" htmlFor="notification-read-filter">
              已读状态
              <select
                id="notification-read-filter"
                onChange={(event) =>
                  patchSearch({
                    read: event.target
                      .value as NotificationSearchParams["read"],
                    page: 1,
                  })
                }
                value={search.read}
              >
                {notificationReadFilters.map((value) => (
                  <option key={value} value={value}>
                    {value === "all"
                      ? "全部"
                      : value === "unread"
                        ? "未读"
                        : "已读"}
                  </option>
                ))}
              </select>
            </label>
            <label className="field" htmlFor="notification-severity-filter">
              严重级别
              <select
                id="notification-severity-filter"
                onChange={(event) =>
                  patchSearch({
                    severity: event.target
                      .value as NotificationSearchParams["severity"],
                    page: 1,
                  })
                }
                value={search.severity}
              >
                <option value="all">全部</option>
                {notificationSeverities.map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="settings-list" style={{ marginTop: 16 }}>
            <h3>桌面原生通知</h3>
            {desktopNotification.status === "checking" ? (
              <p className="muted">正在检测 Desktop 通知能力...</p>
            ) : null}
            {desktopNotification.status === "unavailable" ? (
              <p className="muted">
                {desktopNotification.reason === "not-desktop"
                  ? "浏览器环境不可用；Desktop 客户端中可配置系统通知（D2）。"
                  : desktopNotification.message}
              </p>
            ) : null}
            {desktopNotification.status === "available" ? (
              <p className="muted">
                Desktop 原生通知能力可用，开关由 D2 任务接入。
              </p>
            ) : null}
          </div>
        </section>

        <section className="panel wide" aria-labelledby="notifications-title">
          <div className="panel-header">
            <div>
              <h2 id="notifications-title">通知列表</h2>
              <p>按时间倒序展示站内通知，支持已读管理与关联跳转。</p>
            </div>
            <div className="panel-actions">
              <Button
                disabled={listQuery.isFetching}
                onClick={() => void listQuery.refetch()}
                type="button"
                variant="subtle"
              >
                <RefreshCw aria-hidden="true" data-icon="inline-start" />
                刷新
              </Button>
              <Button
                disabled={selectedIds.size === 0 || markBatch.isPending}
                onClick={() =>
                  runMutation(() =>
                    markBatch
                      .mutateAsync([...selectedIds])
                      .then(() => setSelectedIds(new Set())),
                  )
                }
                type="button"
                variant="subtle"
              >
                批量已读
              </Button>
              <Button
                disabled={markAll.isPending}
                onClick={() => setConfirmAction("mark-all-read")}
                type="button"
                variant="subtle"
              >
                全部已读
              </Button>
              <Button
                disabled={clearRead.isPending}
                onClick={() => setConfirmAction("clear-read")}
                type="button"
                variant="subtle"
              >
                清空已读
              </Button>
            </div>
          </div>

          {listQuery.isLoading ? (
            <div className="empty-state">正在加载通知...</div>
          ) : null}

          {listQuery.isError ? (
            <div className="empty-state error">
              通知列表加载失败，请检查 API 服务或登录状态。
              {listError?.traceId ? `（traceId: ${listError.traceId}）` : ""}
            </div>
          ) : null}

          {!listQuery.isLoading && !listQuery.isError && emptyKind ? (
            <div className="empty-state">
              {notificationEmptyMessage(emptyKind)}
            </div>
          ) : null}

          {!listQuery.isLoading &&
          !listQuery.isError &&
          notifications.length > 0 ? (
            <>
              <NotificationTable
                busyId={busyId}
                existingTaskIds={existingTaskIds}
                notifications={notifications}
                onDelete={(id) =>
                  runMutation(() =>
                    deleteOne.mutateAsync(id).then(() =>
                      setSelectedIds((prev) => {
                        const next = new Set(prev);
                        next.delete(id);
                        return next;
                      }),
                    ),
                  )
                }
                onMarkRead={(id) => runMutation(() => markRead.mutateAsync(id))}
                onToggleSelect={onToggleSelect}
                onToggleSelectAll={onToggleSelectAll}
                selectedIds={selectedIds}
              />
              <div className="panel-actions" style={{ marginTop: 12 }}>
                <span className="muted">
                  第 {page} / {totalPages} 页，共 {total} 条
                </span>
                <Button
                  disabled={page <= 1}
                  onClick={() => patchSearch({ page: page - 1 })}
                  type="button"
                  variant="subtle"
                >
                  上一页
                </Button>
                <Button
                  disabled={page >= totalPages}
                  onClick={() => patchSearch({ page: page + 1 })}
                  type="button"
                  variant="subtle"
                >
                  下一页
                </Button>
              </div>
            </>
          ) : null}

          {actionError ? (
            <div className="empty-state error" style={{ marginTop: 12 }}>
              操作失败
              {actionError.traceId ? `（traceId: ${actionError.traceId}）` : ""}
              ，列表未做错误乐观更新。
            </div>
          ) : null}
        </section>
      </div>

      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {confirmAction === "clear-read" ? "清空已读" : "全部标记已读"}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {confirmAction === "clear-read"
              ? "确定清空所有已读通知？此操作不可撤销。"
              : "确定将全部通知标记为已读？"}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction onClick={confirmDestructive}>
            确认
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
