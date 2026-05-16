import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import type { TaskPayload } from "../../lib/api/types";
import { TaskForm } from "./TaskForm";
import { useCreateTaskMutation } from "./tasks.queries";

export function TaskCreatePage() {
  const navigate = useNavigate();
  const createTask = useCreateTaskMutation();

  async function saveTask(payload: TaskPayload) {
    await createTask.mutateAsync(payload);
    await navigate({ to: "/tasks" });
  }

  return (
    <section className="panel form-panel" aria-labelledby="form-title">
      <div className="panel-header">
        <div>
          <h2 id="form-title">创建任务</h2>
          <p>runner 使用结构化字段，避免保存不可审计的裸命令配置。</p>
        </div>
        <Link className="button subtle" to="/tasks">
          <ArrowLeft aria-hidden="true" size={16} />
          返回列表
        </Link>
      </div>
      <TaskForm
        task={null}
        isSubmitting={createTask.isPending}
        onSubmit={saveTask}
      />
      {createTask.isError ? (
        <p className="form-error">保存失败，请检查字段或后端校验结果。</p>
      ) : null}
    </section>
  );
}
