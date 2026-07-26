import { Link } from "@tanstack/react-router";
import { Bell } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { ApiClientError } from "../../lib/api/client";
import { DEFAULT_NOTIFICATION_SEARCH } from "./notification.schema";
import {
  formatNotificationTime,
  formatUnreadBadge,
  isNotificationRead,
  severityBadgeClass,
  severityLabel,
} from "./notification-format";
import {
  useMarkNotificationReadMutation,
  useNotificationPreviewQuery,
  useUnreadCountQuery,
} from "./notifications.queries";

/**
 * 导航栏通知铃铛：未读角标 + 最近通知快览。
 *
 * 返回值:
 *   - JSX.Element
 */
export function NotificationBell() {
  const panelId = useId();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const unreadQuery = useUnreadCountQuery();
  const previewQuery = useNotificationPreviewQuery(open);
  const markRead = useMarkNotificationReadMutation();
  const badge = formatUnreadBadge(unreadQuery.data?.count ?? 0);
  const items = previewQuery.data?.notifications ?? [];
  const actionError =
    markRead.error instanceof ApiClientError ? markRead.error : null;

  useEffect(() => {
    if (!open) {
      return;
    }
    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div className="notification-bell" ref={rootRef}>
      <button
        aria-controls={panelId}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={badge ? `通知，${badge} 条未读` : "通知"}
        className="icon-button notification-bell-trigger"
        onClick={() => setOpen((value) => !value)}
        type="button"
      >
        <Bell aria-hidden="true" size={16} />
        {badge ? (
          <span className="notification-badge" data-testid="unread-badge">
            {badge}
          </span>
        ) : null}
      </button>
      {open ? (
        <div
          aria-label="通知快览"
          className="notification-preview"
          id={panelId}
          role="dialog"
        >
          <div className="notification-preview-header">
            <strong>最近通知</strong>
            <Link
              className="link-button"
              onClick={() => setOpen(false)}
              search={DEFAULT_NOTIFICATION_SEARCH}
              to="/notifications"
            >
              查看全部
            </Link>
          </div>
          {previewQuery.isLoading ? (
            <div className="empty-state">正在加载通知...</div>
          ) : null}
          {previewQuery.isError ? (
            <div className="empty-state error">
              通知加载失败
              {previewQuery.error instanceof ApiClientError &&
              previewQuery.error.traceId
                ? `（traceId: ${previewQuery.error.traceId}）`
                : ""}
              。
            </div>
          ) : null}
          {!previewQuery.isLoading &&
          !previewQuery.isError &&
          items.length === 0 ? (
            <div className="empty-state">还没有任何站内通知。</div>
          ) : null}
          {items.length > 0 ? (
            <ul className="notification-preview-list">
              {items.map((item) => (
                <li
                  className={
                    isNotificationRead(item)
                      ? "notification-preview-item is-read"
                      : "notification-preview-item"
                  }
                  key={item.id}
                >
                  <div className="notification-preview-meta">
                    <span className={severityBadgeClass(item.severity)}>
                      {severityLabel(item.severity)}
                    </span>
                    <time dateTime={item.occurredAt}>
                      {formatNotificationTime(item.occurredAt)}
                    </time>
                  </div>
                  <p className="notification-preview-title">{item.title}</p>
                  {item.body ? (
                    <p className="notification-preview-body muted">
                      {item.body}
                    </p>
                  ) : null}
                  {!isNotificationRead(item) ? (
                    <button
                      className="link-button"
                      disabled={markRead.isPending}
                      onClick={() => markRead.mutate(item.id)}
                      type="button"
                    >
                      标为已读
                    </button>
                  ) : (
                    <span className="muted">已读</span>
                  )}
                </li>
              ))}
            </ul>
          ) : null}
          {actionError ? (
            <div className="empty-state error">
              标记已读失败
              {actionError.traceId ? `（traceId: ${actionError.traceId}）` : ""}
              。
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
