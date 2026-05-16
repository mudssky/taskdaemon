import { Link } from "@tanstack/react-router";
import { Ban, Pencil, Play, Power, Trash2, XCircle } from "lucide-react";
import type * as React from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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

/**
 * 渲染任务列表表格和行级操作。
 *
 * @param props - 任务数据、选中状态、忙碌任务 ID 和行级操作回调。
 * @returns 任务列表表格或空状态。
 */
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
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>任务</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>cron</TableHead>
          <TableHead>runner</TableHead>
          <TableHead>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tasks.map((task) => (
          <TableRow key={task.id} data-selected={task.id === selectedTaskId}>
            <TableCell>
              <Button
                variant="link"
                type="button"
                onClick={() => onSelect(task)}
              >
                {task.name}
              </Button>
              {task.description ? (
                <span className="muted block">{task.description}</span>
              ) : null}
            </TableCell>
            <TableCell>
              <Badge variant={taskStatusVariant(task)}>
                {taskStatusLabel(task)}
              </Badge>
            </TableCell>
            <TableCell className="mono">{task.cronExpression}</TableCell>
            <TableCell>{runnerTypeLabel(task.runnerType)}</TableCell>
            <TableCell>
              <div className="row-actions">
                <Button
                  asChild
                  variant="default"
                  size="icon"
                  aria-label="编辑任务"
                  title="编辑任务"
                >
                  <Link
                    to="/tasks/$taskId/edit"
                    params={{ taskId: String(task.id) }}
                  >
                    <Pencil aria-hidden="true" data-icon="inline-start" />
                  </Link>
                </Button>
                <IconButton
                  label={task.enabled ? "停用任务" : "启用任务"}
                  onClick={() => onToggleEnabled(task)}
                  disabled={busyTaskId === task.id}
                >
                  <Power aria-hidden="true" data-icon="inline-start" />
                </IconButton>
                <IconButton
                  label="手动触发"
                  onClick={() => onTrigger(task)}
                  disabled={busyTaskId === task.id}
                >
                  <Play aria-hidden="true" data-icon="inline-start" />
                </IconButton>
                {task.running ? (
                  <IconButton
                    label="取消运行"
                    onClick={() => onCancel(task)}
                    disabled={busyTaskId === task.id}
                  >
                    <XCircle aria-hidden="true" data-icon="inline-start" />
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
                  <Trash2 aria-hidden="true" data-icon="inline-start" />
                </IconButton>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function taskStatusVariant(task: Task): "running" | "success" | "muted" {
  if (task.running) {
    return "running";
  }
  return task.enabled ? "success" : "muted";
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
    <Button
      variant="default"
      size="icon"
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </Button>
  );
}
