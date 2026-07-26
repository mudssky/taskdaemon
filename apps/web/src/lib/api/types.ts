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

export type LogArchiveStatus = "absent" | "archived" | "pruned";

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
  logArchiveStatus?: LogArchiveStatus;
  logSizeBytes?: number | null;
  logWriteFailed?: boolean;
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
    /** 仅配置文件字段（C-1 GET 契约）。 */
    fileOnly: string[];
  };
};

/** PUT /api/config/:section 音频部分更新请求（字段均可选）。 */
export type AudioConfigWriteRequest = {
  autoplay?: {
    enabled?: boolean;
    target?: string;
  };
  playback?: {
    queueLimit?: number;
  };
  inbound?: {
    /** 明文 Bearer；落盘为 hash，响应永不回显。 */
    token?: string;
    maxBytes?: number;
    url?: {
      allowedSchemes?: string[];
      allowPrivateNetworks?: boolean;
      allowedHosts?: string[];
      downloadTimeoutSeconds?: number;
      maxRedirects?: number;
    };
  };
  history?: {
    limit?: number;
  };
  ffmpeg?: {
    transcodeTimeoutSeconds?: number;
  };
};

/** 配置字段级错误（error.details.fields[]）。 */
export type ConfigFieldError = {
  path: string;
  reason: string;
  code: string;
};

/** PUT /api/config/:section 成功响应 data。 */
export type ConfigSectionWriteResponse = {
  config: AudioConfig;
  applied: string[];
  restartRequired: string[];
  reload: ConfigSectionReloadResponse;
};

export type ConfigSectionReloadResponse = {
  applied: string[];
  restartRequired: string[];
  subsystems: ConfigSubsystemStatus[];
};

export type ConfigSubsystemStatus = {
  name: string;
  status: string;
  error?: string;
};

/** 通知严重级别（C-2 冻结枚举）。 */
export const notificationSeverities = [
  "info",
  "warning",
  "error",
  "critical",
] as const;
export type NotificationSeverity = (typeof notificationSeverities)[number];

/** 通知关联实体类型（C-2 Subject.Kind）。 */
export const notificationSubjectKinds = ["task", "run", "scheduler"] as const;
export type NotificationSubjectKind = (typeof notificationSubjectKinds)[number];

/** 站内通知 DTO（对齐 /api/notifications）。 */
export type Notification = {
  id: number;
  eventId: string;
  name: string;
  severity: NotificationSeverity | string;
  subjectKind?: NotificationSubjectKind | string;
  subjectId?: string;
  title: string;
  body?: string;
  detail?: Record<string, unknown>;
  readAt?: string | null;
  occurredAt: string;
  createdAt: string;
};

export type NotificationListResponse = {
  notifications: Notification[];
  total: number;
  page: number;
  pageSize: number;
};

export type NotificationListParams = {
  page?: number;
  pageSize?: number;
  /** true=已读，false=未读，undefined=全部 */
  read?: boolean;
  severity?: NotificationSeverity;
};

export type NotificationUnreadCount = {
  count: number;
};

export type NotificationAffectedCount = {
  affected: number;
};

/** 备份模板参数类型（T7a 有限枚举）。 */
export const templateParamTypes = [
  "string",
  "number",
  "boolean",
  "enum",
  "path",
  "secret_ref",
] as const;
export type TemplateParamType = (typeof templateParamTypes)[number];

/** 模板参数定义（对齐 template_dto.templateParamResponse）。 */
export type TemplateParamDef = {
  name: string;
  type: TemplateParamType;
  required: boolean;
  default?: unknown;
  enumOptions?: string[];
  help?: string;
  min?: number;
  max?: number;
  sensitive?: boolean;
};

/** 模板定义（列表项 / 详情）。 */
export type TemplateDefinition = {
  id: string;
  name: string;
  description: string;
  scenario: string;
  runnerType: string;
  params: TemplateParamDef[];
};

export type TemplateListResponse = {
  templates: TemplateDefinition[];
};

/** 渲染请求体。 */
export type TemplateRenderRequest = {
  params: Record<string, unknown>;
};

/** 渲染草稿 runner（与 createTaskRequest.runner 同形）。 */
export type TemplateRunnerDraft = {
  type: string;
  inline?: string;
  scriptPath?: string;
  args?: string[];
  workDir?: string;
  env?: Record<string, string>;
  timeoutSeconds: number;
  outputLimitBytes?: number;
};

/**
 * 渲染成功草稿：与 createTaskRequest 兼容，并附 commandPreview / templateId。
 * 提交创建任务前须经 draftToTaskPayload 剥离预览字段。
 */
export type TemplateTaskDraft = {
  name: string;
  description: string;
  enabled: boolean | null;
  cronExpression: string;
  timezone: string;
  confirmCronWarnings: boolean;
  runner: TemplateRunnerDraft;
  commandPreview: string;
  templateId: string;
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
