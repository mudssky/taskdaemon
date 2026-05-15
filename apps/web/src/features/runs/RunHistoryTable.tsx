import type { TaskRun } from "../../lib/api/types";
import { formatDuration, formatRunTime, runStatusLabel } from "./run-format";

type RunHistoryTableProps = {
  runs: TaskRun[];
  isLoading: boolean;
};

export function RunHistoryTable({ runs, isLoading }: RunHistoryTableProps) {
  if (isLoading) {
    return <div className="empty-state">正在加载执行历史...</div>;
  }
  if (runs.length === 0) {
    return <div className="empty-state">暂无执行历史。</div>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>开始时间</th>
            <th>状态</th>
            <th>来源</th>
            <th>退出码</th>
            <th>耗时</th>
            <th>输出</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run) => (
            <tr key={run.id}>
              <td className="mono">{formatRunTime(run)}</td>
              <td>
                <span className={`badge ${run.status}`}>
                  {runStatusLabel(run.status)}
                </span>
              </td>
              <td>{run.trigger}</td>
              <td>{run.exitCode ?? "-"}</td>
              <td>{formatDuration(run.durationMs)}</td>
              <td>
                <details>
                  <summary>
                    {run.errorSummary || run.stdout || run.stderr
                      ? "查看输出"
                      : "无输出"}
                  </summary>
                  {run.errorSummary ? <pre>{run.errorSummary}</pre> : null}
                  {run.stdout ? <pre>{run.stdout}</pre> : null}
                  {run.stderr ? <pre>{run.stderr}</pre> : null}
                </details>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
