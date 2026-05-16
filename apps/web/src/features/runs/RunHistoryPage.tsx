import { RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useRunHistoryQuery, useTasksQuery } from "../tasks/tasks.queries";
import { RunHistoryTable } from "./RunHistoryTable";

export function RunHistoryPage() {
  const tasksQuery = useTasksQuery();
  const tasks = tasksQuery.data ?? [];
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const effectiveTaskId = useMemo(() => {
    if (tasks.some((task) => task.id === selectedTaskId)) {
      return selectedTaskId;
    }
    return tasks[0]?.id ?? null;
  }, [selectedTaskId, tasks]);
  const selectedTask =
    tasks.find((task) => task.id === effectiveTaskId) ?? null;
  const runsQuery = useRunHistoryQuery(effectiveTaskId);

  useEffect(() => {
    if (
      tasks.length === 0 ||
      tasks.some((task) => task.id === selectedTaskId)
    ) {
      return;
    }
    setSelectedTaskId(tasks[0].id);
  }, [selectedTaskId, tasks]);

  if (tasksQuery.isLoading) {
    return <div className="empty-state">正在加载任务列表...</div>;
  }

  if (tasksQuery.isError) {
    return (
      <div className="empty-state error">
        执行历史加载失败，请确认 API 服务或登录状态。
      </div>
    );
  }

  return (
    <div className="dashboard-grid">
      <section className="panel" aria-labelledby="run-filter-title">
        <div className="panel-header">
          <div>
            <h2 id="run-filter-title">历史范围</h2>
            <p>选择任务后查看它的最近执行记录。</p>
          </div>
        </div>
        {tasks.length === 0 ? (
          <div className="empty-state">还没有任务，暂无可查询的执行历史。</div>
        ) : (
          <label className="field">
            任务
            <select
              value={effectiveTaskId ?? ""}
              onChange={(event) =>
                setSelectedTaskId(Number(event.target.value))
              }
            >
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.name}
                </option>
              ))}
            </select>
          </label>
        )}
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
          <button
            className="button subtle"
            type="button"
            disabled={!effectiveTaskId}
            onClick={() => runsQuery.refetch()}
          >
            <RefreshCw aria-hidden="true" size={16} />
            刷新
          </button>
        </div>
        {runsQuery.isError ? (
          <div className="empty-state error">执行历史加载失败。</div>
        ) : (
          <RunHistoryTable
            runs={runsQuery.data ?? []}
            isLoading={runsQuery.isLoading}
          />
        )}
      </section>
    </div>
  );
}
