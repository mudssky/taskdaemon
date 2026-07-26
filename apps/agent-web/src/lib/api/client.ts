/**
 * Agent API 抽象：HTTP 与 mock 共用同一接口。
 */

import {
  AGENT_API_PREFIX,
  type AguiEvent,
  type CancelRunRequest,
  type CancelRunResponse,
  type CreateRunRequest,
  type CreateThreadRequest,
  type ForkThreadRequest,
  type HitlRespondRequest,
  type HitlRespondResponse,
  type ListMessagesQuery,
  type ListMessagesResponse,
  type ListThreadsQuery,
  type ListThreadsResponse,
  type RuntimeCapabilities,
  type SteerRequest,
  type SteerResponse,
  type StreamReconnectQuery,
  type Thread,
  type ThreadCapabilitiesResponse,
  type UpdateThreadRequest,
} from "@taskdaemon/agent-protocol";
import { errorFromResponse } from "./errors";

export type RuntimeCatalogItem = {
  runtimeId: string;
  label: string;
  /** 静态能力模板（创建前选择用）；会话级以 thread capabilities 为准。 */
  capabilities: RuntimeCapabilities;
};

export type StreamHandlers = {
  onEvent: (event: AguiEvent) => void;
  onError: (error: Error) => void;
  onDone: () => void;
};

export type StreamHandle = {
  /** 中止本地读流（不等于 cancel run）。 */
  abort: () => void;
  runId: Promise<string>;
};

export type AgentApi = {
  listRuntimes: () => Promise<RuntimeCatalogItem[]>;
  listThreads: (query?: ListThreadsQuery) => Promise<ListThreadsResponse>;
  createThread: (body: CreateThreadRequest) => Promise<Thread>;
  getThread: (threadId: string) => Promise<Thread>;
  updateThread: (
    threadId: string,
    body: UpdateThreadRequest,
  ) => Promise<Thread>;
  forkThread: (threadId: string, body?: ForkThreadRequest) => Promise<Thread>;
  deleteThread: (threadId: string) => Promise<void>;
  listMessages: (
    threadId: string,
    query?: ListMessagesQuery,
  ) => Promise<ListMessagesResponse>;
  getCapabilities: (threadId: string) => Promise<ThreadCapabilitiesResponse>;
  streamRun: (
    threadId: string,
    body: CreateRunRequest,
    handlers: StreamHandlers,
  ) => StreamHandle;
  /** 重连已有 run 的事件流（C-3 游标语义）。 */
  reconnectRunStream: (
    threadId: string,
    runId: string,
    query: StreamReconnectQuery | undefined,
    handlers: StreamHandlers,
  ) => StreamHandle;
  cancelRun: (
    threadId: string,
    runId: string,
    body?: CancelRunRequest,
  ) => Promise<CancelRunResponse>;
  steer: (threadId: string, body: SteerRequest) => Promise<SteerResponse>;
  respondHitl: (
    threadId: string,
    body: HitlRespondRequest,
  ) => Promise<HitlRespondResponse>;
};

type DataEnvelope<T> = {
  data: T;
};

type RuntimesPayload = {
  items: RuntimeCatalogItem[];
};

/**
 * 基于 fetch 的真实 gateway 客户端。
 *
 * 参数:
 *   - baseUrl: API 前缀，默认 AGENT_API_PREFIX。
 *
 * 返回值:
 *   - AgentApi 实现。
 */
export function createHttpAgentApi(
  baseUrl: string = AGENT_API_PREFIX,
): AgentApi {
  return {
    async listRuntimes() {
      try {
        const data = await requestJSON<RuntimesPayload>(`${baseUrl}/runtimes`);
        return data.items;
      } catch {
        return [];
      }
    },
    listThreads(query) {
      const qs = toQuery(query);
      return requestJSON(`${baseUrl}/threads${qs}`);
    },
    createThread(body) {
      return requestJSON(`${baseUrl}/threads`, {
        method: "POST",
        body,
      });
    },
    getThread(threadId) {
      return requestJSON(`${baseUrl}/threads/${encodeURIComponent(threadId)}`);
    },
    updateThread(threadId, body) {
      return requestJSON(`${baseUrl}/threads/${encodeURIComponent(threadId)}`, {
        method: "PATCH",
        body,
      });
    },
    forkThread(threadId, body = {}) {
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/fork`,
        { method: "POST", body },
      );
    },
    async deleteThread(threadId) {
      await requestJSON(`${baseUrl}/threads/${encodeURIComponent(threadId)}`, {
        method: "DELETE",
      });
    },
    listMessages(threadId, query) {
      const qs = toQuery(query);
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/messages${qs}`,
      );
    },
    getCapabilities(threadId) {
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/capabilities`,
      );
    },
    streamRun(threadId, body, handlers) {
      return openSse(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/runs/stream`,
        {
          method: "POST",
          body,
        },
        handlers,
      );
    },
    reconnectRunStream(threadId, runId, query, handlers) {
      const qs = toQuery(query);
      const headers: Record<string, string> = {};
      if (query?.lastEventId || query?.cursor) {
        headers["Last-Event-ID"] = query.lastEventId ?? query.cursor ?? "";
      }
      return openSse(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/runs/${encodeURIComponent(runId)}/stream${qs}`,
        {
          method: "GET",
          headers,
          settleRunId: runId,
        },
        handlers,
      );
    },
    cancelRun(threadId, runId, body = { action: "interrupt" }) {
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/runs/${encodeURIComponent(runId)}/cancel`,
        { method: "POST", body },
      );
    },
    steer(threadId, body) {
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/steer`,
        { method: "POST", body },
      );
    },
    respondHitl(threadId, body) {
      return requestJSON(
        `${baseUrl}/threads/${encodeURIComponent(threadId)}/hitl`,
        { method: "POST", body },
      );
    },
  };
}

function openSse(
  path: string,
  options: {
    method: string;
    body?: unknown;
    headers?: Record<string, string>;
    settleRunId?: string;
  },
  handlers: StreamHandlers,
): StreamHandle {
  const controller = new AbortController();
  const { promise: runIdPromise, resolve: resolveRunId } =
    Promise.withResolvers<string>();
  let runIdResolved = false;
  const settleRunId = (id: string) => {
    if (!runIdResolved) {
      runIdResolved = true;
      resolveRunId(id);
    }
  };
  if (options.settleRunId) {
    settleRunId(options.settleRunId);
  }

  void (async () => {
    try {
      const headers: Record<string, string> = {
        ...(options.headers ?? {}),
      };
      if (options.body !== undefined) {
        headers["Content-Type"] = "application/json";
      }
      const response = await fetch(path, {
        method: options.method,
        headers,
        body:
          options.body === undefined ? undefined : JSON.stringify(options.body),
        signal: controller.signal,
      });
      if (!response.ok) {
        let parsed: unknown = null;
        try {
          parsed = await response.json();
        } catch {
          parsed = null;
        }
        throw errorFromResponse(response, parsed);
      }

      const location = response.headers.get("Content-Location");
      const runFromHeader = location?.split("/").pop();
      if (runFromHeader) {
        settleRunId(runFromHeader);
      }

      if (!response.body) {
        throw new Error("stream response has no body");
      }

      await readSseStream(response.body, {
        onEvent: (event) => {
          if (event.type === "RUN_STARTED") {
            settleRunId(event.runId);
          }
          handlers.onEvent(event);
        },
        onError: handlers.onError,
      });
      handlers.onDone();
    } catch (error) {
      if ((error as Error).name === "AbortError") {
        handlers.onDone();
        return;
      }
      handlers.onError(
        error instanceof Error ? error : new Error(String(error)),
      );
    }
  })();

  return {
    abort: () => controller.abort(),
    runId: runIdPromise,
  };
}

async function requestJSON<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  const response = await fetch(path, {
    method: options.method ?? "GET",
    headers:
      options.body === undefined
        ? undefined
        : { "Content-Type": "application/json" },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (response.status === 204) {
    return undefined as T;
  }

  let parsed: unknown = null;
  try {
    parsed = await response.json();
  } catch {
    parsed = null;
  }

  if (!response.ok) {
    throw errorFromResponse(response, parsed);
  }

  if (isDataEnvelope<T>(parsed)) {
    return parsed.data;
  }
  return parsed as T;
}

function isDataEnvelope<T>(value: unknown): value is DataEnvelope<T> {
  return (
    typeof value === "object" &&
    value !== null &&
    "data" in value &&
    value.data !== undefined
  );
}

function toQuery(query: object | undefined): string {
  if (!query) {
    return "";
  }
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) {
      continue;
    }
    if (typeof value === "object") {
      params.set(key, JSON.stringify(value));
      continue;
    }
    params.set(key, String(value));
  }
  const s = params.toString();
  return s ? `?${s}` : "";
}

async function readSseStream(
  body: ReadableStream<Uint8Array>,
  handlers: {
    onEvent: (event: AguiEvent) => void;
    onError: (error: Error) => void;
  },
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() ?? "";
    for (const part of parts) {
      let eventId: string | undefined;
      let dataJson: string | undefined;
      for (const line of part.split("\n")) {
        if (line.startsWith("id:")) {
          eventId = line.slice(3).trim();
        } else if (line.startsWith("data:")) {
          dataJson = line.slice(5).trim();
        }
      }
      if (!dataJson || dataJson === "[DONE]") {
        continue;
      }
      try {
        const event = JSON.parse(dataJson) as AguiEvent;
        if (eventId && !event.id) {
          event.id = eventId;
        }
        handlers.onEvent(event);
      } catch (error) {
        handlers.onError(
          error instanceof Error ? error : new Error(String(error)),
        );
      }
    }
  }
}
