import { Plus, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import type { Task, TaskPayload } from "../../lib/api/types";
import { RunHistoryTable } from "../runs/RunHistoryTable";
import { TaskForm } from "./TaskForm";
import { TaskTable } from "./TaskTable";
import {
  useCancelTaskMutation,
  useCreateTaskMutation,
  useRunHistoryQuery,
  useSetTaskEnabledMutation,
  useTasksQuery,
  useTriggerTaskMutation,
  useUpdateTaskMutation,
} from "./tasks.queries";

export function TaskDashboard() {
  const tasksQuery = useTasksQuery();
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const tasks = tasksQuery.data ?? [];
  const selectedTask = useMemo(
    () => tasks.find((task) => task.id === selectedTaskId) ?? tasks[0] ?? null,
    [selectedTaskId, tasks],
  );
  const runsQuery = useRunHistoryQuery(selectedTask?.id ?? null);
  const createTask = useCreateTaskMutation();
  const updateTask = useUpdateTaskMutation();
  const setEnabled = useSetTaskEnabledMutation();
  const triggerTask = useTriggerTaskMutation();
  const cancelTask = useCancelTaskMutation();
  const busyTaskId =
    setEnabled.variables?.taskId ??
    triggerTask.variables ??
    cancelTask.variables ??
    null;

  async function saveTask(payload: TaskPayload) {
    if (editingTask) {
      await updateTask.mutateAsync({ taskId: editingTask.id, payload });
      setEditingTask(null);
      setSelectedTaskId(editingTask.id);
      return;
    }
    const task = await createTask.mutateAsync(payload);
    setSelectedTaskId(task.id);
  }

  return (
    <div className="dashboard-grid">
      <section className="panel wide" aria-labelledby="tasks-title">
        <div className="panel-header">
          <div>
            <h2 id="tasks-title">任务列表</h2>
            <p>管理调度、启停任务，并从这里发起一次手动执行。</p>
          </div>
          <button
            className="button subtle"
            type="button"
            onClick={() => tasksQuery.refetch()}
          >
            <RefreshCw aria-hidden="true" size={16} />
            刷新
          </button>
        </div>
        {tasksQuery.isError ? (
          <div className="empty-state error">
            任务列表加载失败，请检查 API 服务或登录状态。
          </div>
        ) : (
          <TaskTable
            tasks={tasks}
            selectedTaskId={selectedTask?.id ?? null}
            busyTaskId={busyTaskId}
            onSelect={(task) => setSelectedTaskId(task.id)}
            onEdit={(task) => setEditingTask(task)}
            onToggleEnabled={(task) =>
              setEnabled.mutate({ taskId: task.id, enabled: !task.enabled })
            }
            onTrigger={(task) => triggerTask.mutate(task.id)}
            onCancel={(task) => cancelTask.mutate(task.id)}
          />
        )}
      </section>

      <section className="panel" aria-labelledby="form-title">
        <div className="panel-header">
          <div>
            <h2 id="form-title">{editingTask ? "编辑任务" : "创建任务"}</h2>
            <p>runner 使用结构化字段，避免保存不可审计的裸命令配置。</p>
          </div>
          {editingTask ? (
            <button
              className="icon-button"
              type="button"
              aria-label="新建任务"
              title="新建任务"
              onClick={() => setEditingTask(null)}
            >
              <Plus aria-hidden="true" size={16} />
            </button>
          ) : null}
        </div>
        <TaskForm
          task={editingTask}
          isSubmitting={createTask.isPending || updateTask.isPending}
          onSubmit={saveTask}
        />
        {createTask.isError || updateTask.isError ? (
          <p className="form-error">保存失败，请检查字段或后端校验结果。</p>
        ) : null}
      </section>

      <section className="panel wide" aria-labelledby="runs-title">
        <div className="panel-header">
          <div>
            <h2 id="runs-title">执行历史</h2>
            <p>
              {selectedTask
                ? selectedTask.name
                : "选择一个任务后查看历史记录。"}
            </p>
          </div>
        </div>
        <RunHistoryTable
          runs={runsQuery.data ?? []}
          isLoading={runsQuery.isLoading}
        />
      </section>
    </div>
  );
}
