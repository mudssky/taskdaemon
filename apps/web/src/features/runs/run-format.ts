import type { RunStatus, TaskRun } from "../../lib/api/types";
import { dayjs } from "../../lib/dayjs";

export function runStatusLabel(status: RunStatus): string {
  switch (status) {
    case "queued":
      return "排队中";
    case "running":
      return "运行中";
    case "success":
      return "成功";
    case "failed":
      return "失败";
    case "timeout":
      return "超时";
    case "cancelled":
      return "已取消";
    case "skipped":
      return "已跳过";
  }
}

export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms} ms`;
  }
  const seconds = Math.round(ms / 100) / 10;
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return `${minutes}m ${rest}s`;
}

export function formatRunTime(run: TaskRun): string {
  return dayjs(run.startedAt).format("YYYY-MM-DD HH:mm:ss");
}
