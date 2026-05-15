export const runnerTypes = [
  "shell",
  "bash",
  "pwsh",
  "python",
  "node",
  "typescript",
] as const;
export type RunnerType = (typeof runnerTypes)[number];

export const runStatuses = [
  "queued",
  "running",
  "success",
  "failed",
  "timeout",
  "cancelled",
  "skipped",
] as const;
export type RunStatus = (typeof runStatuses)[number];

export const triggerSources = ["cron", "manual", "cli", "api"] as const;
export type TriggerSource = (typeof triggerSources)[number];

export type RunnerConfig = {
  type?: RunnerType;
  inline?: string;
  scriptPath?: string;
  args?: string[];
  workDir?: string;
  env?: Record<string, string>;
  timeoutSeconds?: number;
  outputLimitBytes?: number;
  cronWarnings?: string[];
};

export type Task = {
  id: number;
  name: string;
  description: string;
  enabled: boolean;
  cronExpression: string;
  timezone: string;
  runnerType: RunnerType;
  runnerConfig: RunnerConfig;
  timeoutSeconds: number;
  overlapPolicy: "skip";
  running: boolean;
  createdAt: string;
  updatedAt: string;
};

export type TaskPayload = {
  name: string;
  description: string;
  enabled: boolean;
  cronExpression: string;
  timezone: string;
  confirmCronWarnings: boolean;
  runner: {
    type: RunnerType;
    inline?: string;
    scriptPath?: string;
    args?: string[];
    workDir?: string;
    env?: Record<string, string>;
    timeoutSeconds: number;
    outputLimitBytes?: number;
  };
};

export type TaskRun = {
  id: number;
  trigger: TriggerSource;
  status: RunStatus;
  exitCode: number | null;
  startedAt: string;
  finishedAt: string | null;
  durationMs: number;
  errorSummary: string;
  stdout: string;
  stderr: string;
};

export type AuthPrincipal = {
  adminId: number;
  username: string;
};

export type AuthLoginResponse = AuthPrincipal & {
  csrfToken: string;
};

export type AuthStatus = {
  initialized: boolean;
  authenticated: boolean;
  admin?: AuthPrincipal;
};

export type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
};
