import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import type { TaskPayload } from "../../lib/api/types";
import { TaskForm } from "./TaskForm";
import { useTasksQuery, useUpdateTaskMutation } from "./tasks.queries";

export function TaskEditPage() {
  const { taskId } = useParams({ from: "/tasks/$taskId/edit" });
  const taskIdNumber = Number(taskId);
  const tasksQuery = useTasksQuery();
  const updateTask = useUpdateTaskMutation();
  const navigate = useNavigate();
  const task =
    Number.isInteger(taskIdNumber) && taskIdNumber > 0
      ? tasksQuery.data?.find((item) => item.id === taskIdNumber)
      : undefined;

  async function saveTask(payload: TaskPayload) {
    if (!task) {
      return;
    }
    await updateTask.mutateAsync({ taskId: task.id, payload });
    await navigate({ to: "/tasks" });
  }

  if (!Number.isInteger(taskIdNumber) || taskIdNumber <= 0) {
    return <div className="empty-state error">任务 ID 不合法。</div>;
  }

  if (tasksQuery.isLoading) {
    return <div className="empty-state">正在加载任务配置...</div>;
  }

  if (tasksQuery.isError) {
    return <div className="empty-state error">任务配置加载失败。</div>;
  }

  if (!task) {
    return <div className="empty-state">没有找到这个任务。</div>;
  }

  return (
    <section className="panel form-panel" aria-labelledby="form-title">
      <div className="panel-header">
        <div>
          <h2 id="form-title">编辑任务</h2>
          <p>正在编辑 {task.name}，保存后会刷新任务列表和执行历史。</p>
        </div>
        <Link className="button subtle" to="/tasks">
          <ArrowLeft aria-hidden="true" size={16} />
          返回列表
        </Link>
      </div>
      <TaskForm
        task={task}
        isSubmitting={updateTask.isPending}
        onSubmit={saveTask}
      />
      {updateTask.isError ? (
        <p className="form-error">保存失败，请检查字段或后端校验结果。</p>
      ) : null}
    </section>
  );
}
