import type {
  ApiEnvelope,
  ApiErrorBody,
  AudioConfig,
  AudioRecord,
  AuthLoginResponse,
  AuthPrincipal,
  AuthStatus,
  Notification,
  NotificationAffectedCount,
  NotificationListParams,
  NotificationListResponse,
  NotificationUnreadCount,
  Task,
  TaskPayload,
  TaskRun,
  TemplateDefinition,
  TemplateListResponse,
  TemplateRenderRequest,
  TemplateTaskDraft,
} from "./types";

export class ApiClientError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;
  readonly traceId: string | null;

  constructor(
    status: number,
    code: string,
    message: string,
    details: unknown,
    traceId: string | null,
  ) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.traceId = traceId;
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
};

async function requestJSON<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const response = await fetch(path, {
    method: options.method ?? "GET",
    credentials: "include",
    headers:
      options.body === undefined
        ? undefined
        : { "Content-Type": "application/json" },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    let body: ApiErrorBody = {};
    try {
      body = (await response.json()) as ApiErrorBody;
    } catch {
      body = {};
    }
    throw new ApiClientError(
      response.status,
      body.error?.code ?? "unknown_error",
      body.error?.message ?? response.statusText,
      body.error?.details ?? null,
      body.traceId ?? response.headers.get("X-Trace-Id"),
    );
  }

  const body = (await response.json()) as ApiEnvelope<T>;
  return body.data;
}

export const apiClient = {
  async login(username: string, password: string) {
    return requestJSON<AuthLoginResponse>("/api/auth/login", {
      method: "POST",
      body: { username, password },
    });
  },

  async initializeAdmin(username: string, password: string) {
    return requestJSON<AuthLoginResponse>("/api/auth/init", {
      method: "POST",
      body: { username, password },
    });
  },

  async authStatus() {
    return requestJSON<AuthStatus>("/api/auth/status");
  },

  async me() {
    return requestJSON<AuthPrincipal>("/api/auth/me");
  },

  async logout() {
    await requestJSON<void>("/api/auth/logout", { method: "POST" });
  },

  async listTasks() {
    return requestJSON<{ tasks: Task[] }>("/api/tasks");
  },

  async createTask(payload: TaskPayload) {
    return requestJSON<Task>("/api/tasks", { method: "POST", body: payload });
  },

  async updateTask(taskId: number, payload: TaskPayload) {
    return requestJSON<Task>(`/api/tasks/${taskId}`, {
      method: "PUT",
      body: payload,
    });
  },

  async setTaskEnabled(taskId: number, enabled: boolean) {
    return requestJSON<Task>(`/api/tasks/${taskId}/enabled`, {
      method: "PATCH",
      body: { enabled },
    });
  },

  async deleteTask(taskId: number) {
    await requestJSON<void>(`/api/tasks/${taskId}`, { method: "DELETE" });
  },

  async triggerTask(taskId: number) {
    return requestJSON<TaskRun>(`/api/tasks/${taskId}/trigger`, {
      method: "POST",
    });
  },

  async cancelTask(taskId: number) {
    await requestJSON<void>(`/api/tasks/${taskId}/cancel`, { method: "POST" });
  },

  async listTaskRuns(taskId: number) {
    return requestJSON<{ runs: TaskRun[] }>(`/api/tasks/${taskId}/runs`);
  },

  /** T6: 下载完整 run 日志（流式 blob） */
  async downloadRunLog(runId: number): Promise<Blob> {
    const response = await fetch(`/api/runs/${runId}/log`, {
      method: "GET",
      credentials: "include",
    });
    if (!response.ok) {
      let code = "RUNLOG_READ_FAILED";
      let message = "下载完整日志失败";
      try {
        const body = (await response.json()) as ApiEnvelope<unknown>;
        code = body.error?.code ?? code;
        message = body.error?.message ?? message;
      } catch {
        // 非 JSON 错误体时保留默认文案
      }
      throw new ApiClientError(response.status, code, message, null, null);
    }
    return response.blob();
  },

  async listAudioHistory(limit?: number) {
    const query =
      limit === undefined ? "" : `?limit=${encodeURIComponent(limit)}`;
    return requestJSON<{ records: AudioRecord[] }>(
      `/api/audio/history${query}`,
    );
  },

  async replayAudioRecord(recordId: number) {
    return requestJSON<AudioRecord>(`/api/audio/history/${recordId}/replay`, {
      method: "POST",
    });
  },

  async audioConfig() {
    return requestJSON<AudioConfig>("/api/config/audio");
  },

  async listNotifications(params: NotificationListParams = {}) {
    const search = new URLSearchParams();
    if (params.page !== undefined) {
      search.set("page", String(params.page));
    }
    if (params.pageSize !== undefined) {
      search.set("pageSize", String(params.pageSize));
    }
    if (params.read !== undefined) {
      search.set("read", String(params.read));
    }
    if (params.severity !== undefined) {
      search.set("severity", params.severity);
    }
    const query = search.toString();
    return requestJSON<NotificationListResponse>(
      `/api/notifications${query ? `?${query}` : ""}`,
    );
  },

  async notificationUnreadCount() {
    return requestJSON<NotificationUnreadCount>(
      "/api/notifications/unread-count",
    );
  },

  async markNotificationRead(id: number) {
    return requestJSON<Notification>(`/api/notifications/${id}/read`, {
      method: "POST",
    });
  },

  async markNotificationsRead(ids: number[]) {
    return requestJSON<NotificationAffectedCount>("/api/notifications/read", {
      method: "POST",
      body: { ids },
    });
  },

  async markAllNotificationsRead() {
    return requestJSON<NotificationAffectedCount>(
      "/api/notifications/read-all",
      { method: "POST" },
    );
  },

  async deleteNotification(id: number) {
    await requestJSON<void>(`/api/notifications/${id}`, { method: "DELETE" });
  },

  async clearReadNotifications() {
    return requestJSON<NotificationAffectedCount>("/api/notifications/read", {
      method: "DELETE",
    });
  },

  /** T7a: 列出全部备份模板（含参数定义）。 */
  async listTemplates() {
    return requestJSON<TemplateListResponse>("/api/templates");
  },

  /**
   * T7a: 查询单个模板详情。
   *
   * @param id - 模板稳定标识。
   * @returns 模板定义。
   */
  async getTemplate(id: string) {
    return requestJSON<TemplateDefinition>(
      `/api/templates/${encodeURIComponent(id)}`,
    );
  },

  /**
   * T7a: 渲染任务草稿（不落库）。
   *
   * @param id - 模板标识。
   * @param body - 用户参数。
   * @returns 与 createTask 兼容的草稿 + commandPreview。
   */
  async renderTemplate(id: string, body: TemplateRenderRequest) {
    return requestJSON<TemplateTaskDraft>(
      `/api/templates/${encodeURIComponent(id)}/render`,
      { method: "POST", body },
    );
  },
};
