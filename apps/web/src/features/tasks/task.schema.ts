import { z } from "zod";
import { runnerTypes, type Task, type TaskPayload } from "../../lib/api/types";
import { validateCronExpression } from "../scheduler/cron";

const runnerTypeSchema = z.enum(runnerTypes);

export const taskFormSchema = z
  .object({
    name: z.string().trim().min(1, "任务名称不能为空"),
    description: z.string(),
    enabled: z.boolean(),
    cronExpression: z.string().trim().min(1, "cron 表达式不能为空"),
    timezone: z.string().trim().min(1, "timezone 不能为空"),
    runnerType: runnerTypeSchema,
    commandMode: z.enum(["inline", "script"]),
    inline: z.string(),
    scriptPath: z.string(),
    argsText: z.string(),
    workDir: z.string(),
    envText: z.string(),
    timeoutSeconds: z.number().int().positive("timeout 必须大于 0"),
    outputLimitBytes: z.number().int().nonnegative("输出截断大小不能为负数"),
    confirmCronWarnings: z.boolean(),
  })
  .superRefine((values, ctx) => {
    const cron = validateCronExpression(values.cronExpression, values.timezone);
    if (!cron.ok) {
      ctx.addIssue({
        code: "custom",
        path: ["cronExpression"],
        message: cron.error,
      });
    } else if (cron.warnings.length > 0 && !values.confirmCronWarnings) {
      ctx.addIssue({
        code: "custom",
        path: ["confirmCronWarnings"],
        message: "高频或秒级 cron 需要显式确认",
      });
    }

    if (values.commandMode === "inline" && values.inline.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["inline"],
        message: "命令片段不能为空",
      });
    }
    if (values.commandMode === "script" && values.scriptPath.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["scriptPath"],
        message: "脚本路径不能为空",
      });
    }
  });

export type TaskFormValues = z.infer<typeof taskFormSchema>;

export function defaultTaskFormValues(task?: Task | null): TaskFormValues {
  const runnerConfig = task?.runnerConfig ?? {};
  const hasScriptPath = Boolean(runnerConfig.scriptPath);
  return {
    name: task?.name ?? "",
    description: task?.description ?? "",
    enabled: task?.enabled ?? true,
    cronExpression: task?.cronExpression ?? "30 9 * * *",
    timezone: task?.timezone ?? "Local",
    runnerType: task?.runnerType ?? "shell",
    commandMode: hasScriptPath ? "script" : "inline",
    inline: runnerConfig.inline ?? "",
    scriptPath: runnerConfig.scriptPath ?? "",
    argsText: (runnerConfig.args ?? []).join(" "),
    workDir: runnerConfig.workDir ?? "",
    envText: envToText(runnerConfig.env ?? {}),
    timeoutSeconds: task?.timeoutSeconds ?? runnerConfig.timeoutSeconds ?? 3600,
    outputLimitBytes: runnerConfig.outputLimitBytes ?? 65536,
    confirmCronWarnings: false,
  };
}

export function toTaskPayload(values: TaskFormValues): TaskPayload {
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    enabled: values.enabled,
    cronExpression: values.cronExpression.trim(),
    timezone: values.timezone.trim(),
    confirmCronWarnings: values.confirmCronWarnings,
    runner: {
      type: values.runnerType,
      inline: values.commandMode === "inline" ? values.inline.trim() : "",
      scriptPath:
        values.commandMode === "script" ? values.scriptPath.trim() : "",
      args: splitArgs(values.argsText),
      workDir: values.workDir.trim(),
      env: parseEnvText(values.envText),
      timeoutSeconds: values.timeoutSeconds,
      outputLimitBytes: values.outputLimitBytes,
    },
  };
}

export function parseEnvText(value: string): Record<string, string> {
  const env: Record<string, string> = {};
  for (const line of value.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    const [key, ...rest] = trimmed.split("=");
    if (!key || rest.length === 0) {
      continue;
    }
    env[key.trim()] = rest.join("=").trim();
  }
  return env;
}

function envToText(env: Record<string, string>): string {
  return Object.entries(env)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}

function splitArgs(value: string): string[] {
  return value
    .split(/\s+/)
    .map((item) => item.trim())
    .filter(Boolean);
}
