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

export const audioSourceKinds = ["url", "upload"] as const;
export type AudioSourceKind = (typeof audioSourceKinds)[number];

export const audioRecordStatuses = [
  "received",
  "queued",
  "playing",
  "played",
  "failed",
  "skipped",
  "queued_skipped",
] as const;
export type AudioRecordStatus = (typeof audioRecordStatuses)[number];

export type AudioRecord = {
  id: number;
  sourceKind: AudioSourceKind;
  source: string;
  originalFilename: string;
  storedPath: string;
  mimeType: string;
  sizeBytes: number;
  sha256: string;
  status: AudioRecordStatus;
  errorSummary: string;
  playedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type AudioConfig = {
  autoplay: {
    enabled: boolean;
    target: "backend" | "frontend" | string;
  };
  playback: {
    queueLimit: number;
  };
  inbound: {
    tokenConfigured: boolean;
    maxBytes: number;
    url: {
      allowedSchemes: string[];
      allowPrivateNetworks: boolean;
      allowedHosts: string[];
      downloadTimeoutSeconds: number;
      maxRedirects: number;
    };
  };
  history: {
    limit: number;
  };
  ffmpeg: {
    pathConfigured: boolean;
    probePathConfigured: boolean;
    transcodeTimeoutSeconds: number;
  };
  configuration: {
    runtimeEditable: string[];
    restartRequired: string[];
  };
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
  code?: number;
  msg?: string;
  data?: null;
  traceId?: string;
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
};

export type ApiEnvelope<T> = {
  code: number;
  msg: string;
  data: T;
  traceId?: string;
  error?: ApiErrorBody["error"];
};
