import { useState } from "react";
import { ApiClientError, apiClient } from "../../lib/api/client";
import type { LogArchiveStatus, TaskRun } from "../../lib/api/types";
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
            <th>完整日志</th>
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
              <td>
                <RunLogDownloadCell run={run} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

type RunLogDownloadCellProps = {
  run: TaskRun;
};

/**
 * 按归档三态渲染下载入口；不做日志查看器。
 *
 * 参数:
 *   - run: 执行历史行。
 *
 * 返回值:
 *   - 下载按钮或状态说明。
 */
function RunLogDownloadCell({ run }: RunLogDownloadCellProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const status = (run.logArchiveStatus ?? "absent") as LogArchiveStatus;

  if (status === "absent") {
    return (
      <span
        className="muted"
        title={run.logWriteFailed ? "落盘失败" : undefined}
      >
        无归档（旧数据）
      </span>
    );
  }
  if (status === "pruned") {
    return <span className="muted">已清理</span>;
  }

  return (
    <div className="runlog-download">
      <button
        type="button"
        className="button ghost"
        disabled={loading}
        onClick={async () => {
          setLoading(true);
          setError(null);
          try {
            const blob = await apiClient.downloadRunLog(run.id);
            const url = URL.createObjectURL(blob);
            const anchor = document.createElement("a");
            anchor.href = url;
            anchor.download = `run-${run.id}.log`;
            anchor.click();
            URL.revokeObjectURL(url);
          } catch (err) {
            if (err instanceof ApiClientError) {
              setError(err.message);
            } else {
              setError("下载失败");
            }
          } finally {
            setLoading(false);
          }
        }}
      >
        {loading ? "下载中..." : "下载完整日志"}
      </button>
      {error ? <div className="empty-state error">{error}</div> : null}
    </div>
  );
}
