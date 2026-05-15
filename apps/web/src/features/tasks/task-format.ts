import type { RunnerType, Task } from "../../lib/api/types";

export function runnerTypeLabel(type: RunnerType): string {
  switch (type) {
    case "shell":
      return "Shell";
    case "bash":
      return "Bash";
    case "pwsh":
      return "PowerShell";
    case "python":
      return "Python";
    case "node":
      return "Node.js";
    case "typescript":
      return "TypeScript";
  }
}

export function taskStatusLabel(task: Task): string {
  if (task.running) {
    return "运行中";
  }
  return task.enabled ? "已启用" : "已停用";
}
