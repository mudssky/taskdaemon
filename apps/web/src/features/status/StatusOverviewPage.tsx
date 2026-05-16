import { Activity, CheckCircle2, Clock3, ListChecks } from "lucide-react";
import { useMemo } from "react";
import type { Task, TaskRun } from "../../lib/api/types";
import {
  formatDuration,
  formatRunTime,
  runStatusLabel,
} from "../runs/run-format";
import { runnerTypeLabel } from "../tasks/task-format";
import { useRunHistoryQuery, useTasksQuery } from "../tasks/tasks.queries";

type StatusMetric = {
  label: string;
  value: number;
  hint: string;
  icon: React.ReactNode;
};

export function StatusOverviewPage() {
  const tasksQuery = useTasksQuery();
  const tasks = tasksQuery.data ?? [];
  const focusTask = useMemo(() => selectFocusTask(tasks), [tasks]);
  const runsQuery = useRunHistoryQuery(focusTask?.id ?? null);
  const latestRun = runsQuery.data?.[0] ?? null;
  const metrics = buildStatusMetrics(tasks);

  if (tasksQuery.isLoading) {
    return <div className="empty-state">正在加载运行状态...</div>;
  }

  if (tasksQuery.isError) {
    return (
      <div className="empty-state error">
        运行状态加载失败，请确认 API 服务或登录状态。
      </div>
    );
  }

  return (
    <div className="overview-grid">
      <section className="overview-metrics" aria-label="运行指标">
        {metrics.map((metric) => (
          <article className="metric-card" key={metric.label}>
            <span className="metric-icon">{metric.icon}</span>
            <div>
              <p className="eyebrow">{metric.label}</p>
              <strong>{metric.value}</strong>
              <span>{metric.hint}</span>
            </div>
          </article>
        ))}
      </section>

      <section className="panel wide" aria-labelledby="overview-focus-title">
        <div className="panel-header">
          <div>
            <h2 id="overview-focus-title">当前关注</h2>
            <p>优先展示运行中的任务；没有运行中任务时展示最近可用任务。</p>
          </div>
        </div>
        {focusTask ? (
          <div className="status-summary">
            <div>
              <p className="eyebrow">任务</p>
              <h3>{focusTask.name}</h3>
              <p>{focusTask.description || "暂无描述"}</p>
            </div>
            <dl>
              <div>
                <dt>状态</dt>
                <dd>
                  <span
                    className={`badge ${focusTask.running ? "running" : focusTask.enabled ? "success" : "muted"}`}
                  >
                    {focusTask.running
                      ? "运行中"
                      : focusTask.enabled
                        ? "已启用"
                        : "已停用"}
                  </span>
                </dd>
              </div>
              <div>
                <dt>cron</dt>
                <dd className="mono">{focusTask.cronExpression}</dd>
              </div>
              <div>
                <dt>runner</dt>
                <dd>{runnerTypeLabel(focusTask.runnerType)}</dd>
              </div>
            </dl>
          </div>
        ) : (
          <div className="empty-state">
            还没有任务，创建任务后这里会显示运行概览。
          </div>
        )}
      </section>

      <section className="panel" aria-labelledby="overview-run-title">
        <div className="panel-header">
          <div>
            <h2 id="overview-run-title">最近执行</h2>
            <p>{focusTask ? focusTask.name : "暂无可查询任务"}</p>
          </div>
        </div>
        {focusTask ? (
          <LatestRunSummary
            isLoading={runsQuery.isLoading}
            hasError={runsQuery.isError}
            latestRun={latestRun}
          />
        ) : (
          <div className="empty-state">暂无执行历史。</div>
        )}
      </section>
    </div>
  );
}

function LatestRunSummary({
  isLoading,
  hasError,
  latestRun,
}: {
  isLoading: boolean;
  hasError: boolean;
  latestRun: TaskRun | null;
}) {
  if (isLoading) {
    return <div className="empty-state">正在加载最近执行...</div>;
  }

  if (hasError) {
    return <div className="empty-state error">最近执行加载失败。</div>;
  }

  if (!latestRun) {
    return <div className="empty-state">暂无执行历史。</div>;
  }

  return (
    <div className="latest-run">
      <span className={`badge ${latestRun.status}`}>
        {runStatusLabel(latestRun.status)}
      </span>
      <dl>
        <div>
          <dt>开始时间</dt>
          <dd className="mono">{formatRunTime(latestRun)}</dd>
        </div>
        <div>
          <dt>来源</dt>
          <dd>{latestRun.trigger}</dd>
        </div>
        <div>
          <dt>耗时</dt>
          <dd>{formatDuration(latestRun.durationMs)}</dd>
        </div>
      </dl>
      {latestRun.errorSummary ? (
        <p className="form-error">{latestRun.errorSummary}</p>
      ) : null}
    </div>
  );
}

function buildStatusMetrics(tasks: Task[]): StatusMetric[] {
  const enabledCount = tasks.filter((task) => task.enabled).length;
  const runningCount = tasks.filter((task) => task.running).length;
  const pausedCount = tasks.length - enabledCount;

  return [
    {
      label: "任务总数",
      value: tasks.length,
      hint: "已登记的调度任务",
      icon: <ListChecks aria-hidden="true" size={18} />,
    },
    {
      label: "已启用",
      value: enabledCount,
      hint: "会按 cron 自动触发",
      icon: <CheckCircle2 aria-hidden="true" size={18} />,
    },
    {
      label: "运行中",
      value: runningCount,
      hint: "当前进程内活跃任务",
      icon: <Activity aria-hidden="true" size={18} />,
    },
    {
      label: "已停用",
      value: pausedCount,
      hint: "保留配置但不自动运行",
      icon: <Clock3 aria-hidden="true" size={18} />,
    },
  ];
}

function selectFocusTask(tasks: Task[]): Task | null {
  return (
    tasks.find((task) => task.running) ??
    tasks.find((task) => task.enabled) ??
    tasks[0] ??
    null
  );
}
