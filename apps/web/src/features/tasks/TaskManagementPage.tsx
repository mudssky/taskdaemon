import { Link } from "@tanstack/react-router";
import { Plus, RefreshCw } from "lucide-react";
import { useState } from "react";
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
import type { Task } from "../../lib/api/types";
import { TaskTable } from "./TaskTable";
import {
  useCancelTaskMutation,
  useDeleteTaskMutation,
  useSetTaskEnabledMutation,
  useTasksQuery,
  useTriggerTaskMutation,
} from "./tasks.queries";

export function TaskManagementPage() {
  const tasksQuery = useTasksQuery();
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Task | null>(null);
  const setEnabled = useSetTaskEnabledMutation();
  const triggerTask = useTriggerTaskMutation();
  const cancelTask = useCancelTaskMutation();
  const deleteTask = useDeleteTaskMutation();
  const tasks = tasksQuery.data ?? [];
  const busyTaskId =
    setEnabled.variables?.taskId ??
    triggerTask.variables ??
    cancelTask.variables ??
    deleteTask.variables ??
    null;

  function selectTask(task: Task) {
    setSelectedTaskId(task.id);
  }

  function confirmDeleteTask() {
    if (!deleteCandidate) {
      return;
    }
    deleteTask.mutate(deleteCandidate.id);
    setDeleteCandidate(null);
  }

  return (
    <AlertDialog
      open={deleteCandidate !== null}
      onOpenChange={(open) => {
        if (!open) {
          setDeleteCandidate(null);
        }
      }}
    >
      <section className="panel wide" aria-labelledby="tasks-title">
        <div className="panel-header">
          <div>
            <h2 id="tasks-title">任务列表</h2>
            <p>管理调度、启停任务，并从这里发起一次手动执行。</p>
          </div>
          <div className="panel-actions">
            <Button
              variant="subtle"
              type="button"
              onClick={() => tasksQuery.refetch()}
            >
              <RefreshCw aria-hidden="true" data-icon="inline-start" />
              刷新
            </Button>
            <Button asChild variant="primary">
              <Link to="/tasks/new">
                <Plus aria-hidden="true" data-icon="inline-start" />
                新建任务
              </Link>
            </Button>
          </div>
        </div>
        {tasksQuery.isError ? (
          <div className="empty-state error">
            任务列表加载失败，请检查 API 服务或登录状态。
          </div>
        ) : (
          <TaskTable
            tasks={tasks}
            selectedTaskId={selectedTaskId}
            busyTaskId={busyTaskId}
            onSelect={selectTask}
            onToggleEnabled={(task) =>
              setEnabled.mutate({ taskId: task.id, enabled: !task.enabled })
            }
            onTrigger={(task) => triggerTask.mutate(task.id)}
            onCancel={(task) => cancelTask.mutate(task.id)}
            onDelete={(task) => setDeleteCandidate(task)}
          />
        )}
      </section>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除任务</AlertDialogTitle>
          <AlertDialogDescription>
            确定删除任务 {deleteCandidate?.name}？执行历史也会被清理。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction onClick={confirmDeleteTask}>
            删除任务
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
