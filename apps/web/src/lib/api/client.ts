import type {
  ApiErrorBody,
  AuthLoginResponse,
  AuthPrincipal,
  AuthStatus,
  Task,
  TaskPayload,
  TaskRun,
} from "./types";

export class ApiClientError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;

  constructor(status: number, code: string, message: string, details: unknown) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
    this.details = details;
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
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
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
};
