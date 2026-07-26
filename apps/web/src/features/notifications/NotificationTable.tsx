import { Link } from "@tanstack/react-router";
import { Check, Trash2 } from "lucide-react";
import type { Notification } from "../../lib/api/types";
import {
  formatNotificationTime,
  isNotificationRead,
  resolveSubjectLink,
  severityBadgeClass,
  severityLabel,
} from "./notification-format";

type NotificationTableProps = {
  notifications: Notification[];
  existingTaskIds: ReadonlySet<number>;
  selectedIds: ReadonlySet<number>;
  busyId: number | null;
  onToggleSelect: (id: number, checked: boolean) => void;
  onToggleSelectAll: (checked: boolean) => void;
  onMarkRead: (id: number) => void;
  onDelete: (id: number) => void;
};

/**
 * 通知列表表格。
 *
 * 参数:
 *   - props: 数据与行级操作回调。
 *
 * 返回值:
 *   - JSX.Element
 */
export function NotificationTable({
  notifications,
  existingTaskIds,
  selectedIds,
  busyId,
  onToggleSelect,
  onToggleSelectAll,
  onMarkRead,
  onDelete,
}: NotificationTableProps) {
  const allSelected =
    notifications.length > 0 &&
    notifications.every((item) => selectedIds.has(item.id));

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th scope="col">
              <input
                aria-label="全选当前页"
                checked={allSelected}
                onChange={(event) => onToggleSelectAll(event.target.checked)}
                type="checkbox"
              />
            </th>
            <th scope="col">级别</th>
            <th scope="col">标题 / 正文</th>
            <th scope="col">时间</th>
            <th scope="col">关联</th>
            <th scope="col">状态</th>
            <th scope="col">操作</th>
          </tr>
        </thead>
        <tbody>
          {notifications.map((item) => {
            const subject = resolveSubjectLink(item, existingTaskIds);
            const read = isNotificationRead(item);
            const busy = busyId === item.id;
            return (
              <tr
                data-selected={selectedIds.has(item.id) ? "true" : "false"}
                key={item.id}
              >
                <td>
                  <input
                    aria-label={`选择通知 ${item.id}`}
                    checked={selectedIds.has(item.id)}
                    onChange={(event) =>
                      onToggleSelect(item.id, event.target.checked)
                    }
                    type="checkbox"
                  />
                </td>
                <td>
                  <span className={severityBadgeClass(item.severity)}>
                    {severityLabel(item.severity)}
                  </span>
                </td>
                <td>
                  <strong className="block">{item.title}</strong>
                  {item.body ? (
                    <span className="muted block">{item.body}</span>
                  ) : null}
                </td>
                <td className="mono">
                  <time dateTime={item.occurredAt}>
                    {formatNotificationTime(item.occurredAt)}
                  </time>
                </td>
                <td>
                  {subject?.available && subject.target.type === "task" ? (
                    <Link
                      className="link-button"
                      params={{ taskId: String(subject.target.taskId) }}
                      to="/tasks/$taskId/edit"
                    >
                      {subject.label}
                    </Link>
                  ) : null}
                  {subject?.available && subject.target.type === "runs" ? (
                    <Link className="link-button" to="/runs">
                      {subject.label}
                    </Link>
                  ) : null}
                  {subject && !subject.available ? (
                    <span
                      aria-disabled="true"
                      className="muted"
                      title={subject.reason}
                    >
                      {subject.label}（{subject.reason}）
                    </span>
                  ) : null}
                  {!subject ? <span className="muted">—</span> : null}
                </td>
                <td>
                  <span className={read ? "badge" : "badge running"}>
                    {read ? "已读" : "未读"}
                  </span>
                </td>
                <td>
                  <div className="row-actions">
                    {!read ? (
                      <button
                        aria-label={`标记通知 ${item.id} 已读`}
                        className="icon-button"
                        disabled={busy}
                        onClick={() => onMarkRead(item.id)}
                        title="标为已读"
                        type="button"
                      >
                        <Check aria-hidden="true" size={16} />
                      </button>
                    ) : (
                      <span className="icon-placeholder" />
                    )}
                    <button
                      aria-label={`删除通知 ${item.id}`}
                      className="icon-button"
                      disabled={busy}
                      onClick={() => onDelete(item.id)}
                      title="删除"
                      type="button"
                    >
                      <Trash2 aria-hidden="true" size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
