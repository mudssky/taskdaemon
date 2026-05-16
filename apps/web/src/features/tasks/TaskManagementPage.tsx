import { Link } from "@tanstack/react-router";
import { Plus, RefreshCw } from "lucide-react";
import { useState } from "react";
import type { Task } from "../../lib/api/types";
import { TaskTable } from "./TaskTable";
import {
  useCancelTaskMutation,
  useSetTaskEnabledMutation,
  useTasksQuery,
  useTriggerTaskMutation,
} from "./tasks.queries";

export function TaskManagementPage() {
  const tasksQuery = useTasksQuery();
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const setEnabled = useSetTaskEnabledMutation();
  const triggerTask = useTriggerTaskMutation();
  const cancelTask = useCancelTaskMutation();
  const tasks = tasksQuery.data ?? [];
  const busyTaskId =
    setEnabled.variables?.taskId ??
    triggerTask.variables ??
    cancelTask.variables ??
    null;

  function selectTask(task: Task) {
    setSelectedTaskId(task.id);
  }

  return (
    <section className="panel wide" aria-labelledby="tasks-title">
      <div className="panel-header">
        <div>
          <h2 id="tasks-title">任务列表</h2>
          <p>管理调度、启停任务，并从这里发起一次手动执行。</p>
        </div>
        <div className="panel-actions">
          <button
            className="button subtle"
            type="button"
            onClick={() => tasksQuery.refetch()}
          >
            <RefreshCw aria-hidden="true" size={16} />
            刷新
          </button>
          <Link className="button primary" to="/tasks/new">
            <Plus aria-hidden="true" size={16} />
            新建任务
          </Link>
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
        />
      )}
    </section>
  );
}
