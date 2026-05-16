import { Link } from "@tanstack/react-router";
import { Ban, Pencil, Play, Power, Trash2, XCircle } from "lucide-react";
import type { Task } from "../../lib/api/types";
import { runnerTypeLabel, taskStatusLabel } from "./task-format";

type TaskTableProps = {
  tasks: Task[];
  selectedTaskId: number | null;
  busyTaskId: number | null;
  onSelect: (task: Task) => void;
  onToggleEnabled: (task: Task) => void;
  onTrigger: (task: Task) => void;
  onCancel: (task: Task) => void;
  onDelete: (task: Task) => void;
};

export function TaskTable({
  tasks,
  selectedTaskId,
  busyTaskId,
  onSelect,
  onToggleEnabled,
  onTrigger,
  onCancel,
  onDelete,
}: TaskTableProps) {
  if (tasks.length === 0) {
    return (
      <div className="empty-state">
        <p>还没有任务。创建第一个任务后，它会出现在运行状态列表里。</p>
      </div>
    );
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>任务</th>
            <th>状态</th>
            <th>cron</th>
            <th>runner</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {tasks.map((task) => (
            <tr key={task.id} data-selected={task.id === selectedTaskId}>
              <td>
                <button
                  className="link-button"
                  type="button"
                  onClick={() => onSelect(task)}
                >
                  {task.name}
                </button>
                {task.description ? (
                  <span className="muted block">{task.description}</span>
                ) : null}
              </td>
              <td>
                <span
                  className={`badge ${task.running ? "running" : task.enabled ? "success" : "muted"}`}
                >
                  {taskStatusLabel(task)}
                </span>
              </td>
              <td className="mono">{task.cronExpression}</td>
              <td>{runnerTypeLabel(task.runnerType)}</td>
              <td>
                <div className="row-actions">
                  <Link
                    className="icon-button"
                    to="/tasks/$taskId/edit"
                    params={{ taskId: String(task.id) }}
                    aria-label="编辑任务"
                    title="编辑任务"
                  >
                    <Pencil aria-hidden="true" size={16} />
                  </Link>
                  <IconButton
                    label={task.enabled ? "停用任务" : "启用任务"}
                    onClick={() => onToggleEnabled(task)}
                    disabled={busyTaskId === task.id}
                  >
                    <Power aria-hidden="true" size={16} />
                  </IconButton>
                  <IconButton
                    label="手动触发"
                    onClick={() => onTrigger(task)}
                    disabled={busyTaskId === task.id}
                  >
                    <Play aria-hidden="true" size={16} />
                  </IconButton>
                  {task.running ? (
                    <IconButton
                      label="取消运行"
                      onClick={() => onCancel(task)}
                      disabled={busyTaskId === task.id}
                    >
                      <XCircle aria-hidden="true" size={16} />
                    </IconButton>
                  ) : (
                    <span className="icon-placeholder" aria-hidden="true">
                      <Ban size={16} />
                    </span>
                  )}
                  <IconButton
                    label="删除任务"
                    onClick={() => onDelete(task)}
                    disabled={busyTaskId === task.id || task.running}
                  >
                    <Trash2 aria-hidden="true" size={16} />
                  </IconButton>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function IconButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      className="icon-button"
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </button>
  );
}
