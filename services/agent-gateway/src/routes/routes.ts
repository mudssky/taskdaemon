/**
 * HTTP 路由：C-3 Agent Protocol 子集 + 运维端点。
 */

import {
  AGENT_API_PREFIX,
  AgentRoutes,
  type CreateRunRequest,
  type CreateThreadRequest,
  type Principal,
} from "@taskdaemon/agent-protocol";
import type { Context } from "hono";
import { Hono } from "hono";
import { errorBody, toAgentHttpError } from "../errors.js";
import type { McpCatalog } from "../mcp/catalog.js";
import { mountMcpRoutes } from "../mcp/routes.js";
import type { Orchestrator } from "../runtime/orchestrator.js";
import type { PrincipalVariables } from "../trust/middleware.js";

export type AppEnv = {
  Variables: PrincipalVariables & {
    orchestrator: Orchestrator;
    mcpCatalog: McpCatalog;
  };
};

/**
 * 挂载业务路由。
 *
 * 参数:
 *   - app: Hono 实例。
 *   - getOrchestrator: 取编排器。
 *   - getMcpCatalog: 取 MCP 目录。
 */
export function mountRoutes(
  app: Hono<AppEnv>,
  getOrchestrator: () => Orchestrator,
  getMcpCatalog: () => McpCatalog,
): void {
  const v1 = new Hono<AppEnv>();

  v1.get("/runtimes", (c) => {
    return c.json({ items: getOrchestrator().listRuntimes() });
  });

  v1.post(AgentRoutes.threads, async (c) => {
    try {
      const principal = c.get("principal");
      const body = (await c.req
        .json()
        .catch(() => ({}))) as CreateThreadRequest;
      const thread = await getOrchestrator().createThread(principal, body);
      return c.json(thread, 200);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threads, (c) => {
    try {
      const principal = c.get("principal");
      const q = c.req.query();
      const result = getOrchestrator().listThreads(principal, {
        offset: q.offset ? Number(q.offset) : undefined,
        limit: q.limit ? Number(q.limit) : undefined,
        status: q.status as never,
        runtimeId: q.runtimeId,
        profile: q.profile as never,
        sortField:
          (q["sort.field"] as "created_at" | "updated_at") ?? undefined,
        sortDir: (q["sort.direction"] as "asc" | "desc") ?? undefined,
      });
      return c.json(result);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.thread, (c) => {
    try {
      const principal = c.get("principal");
      return c.json(
        getOrchestrator().getThread(principal, c.req.param("threadId")),
      );
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.delete(AgentRoutes.thread, async (c) => {
    try {
      const principal = c.get("principal");
      await getOrchestrator().deleteThread(principal, c.req.param("threadId"));
      return c.body(null, 204);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threadMessages, async (c) => {
    try {
      const principal = c.get("principal");
      const q = c.req.query();
      const result = await getOrchestrator().getMessages(
        principal,
        c.req.param("threadId"),
        {
          offset: q.offset ? Number(q.offset) : undefined,
          limit: q.limit ? Number(q.limit) : undefined,
        },
      );
      return c.json(result);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threadCapabilities, (c) => {
    try {
      const principal = c.get("principal");
      return c.json(
        getOrchestrator().getCapabilities(principal, c.req.param("threadId")),
      );
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.post(AgentRoutes.threadRuns, async (c) => {
    try {
      const principal = c.get("principal");
      const body = (await c.req.json().catch(() => ({}))) as CreateRunRequest;
      const run = await getOrchestrator().createRun(
        principal,
        c.req.param("threadId"),
        body,
      );
      return c.json(run);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threadRuns, (c) => {
    try {
      const principal = c.get("principal");
      const q = c.req.query();
      return c.json(
        getOrchestrator().listRuns(principal, c.req.param("threadId"), {
          offset: q.offset ? Number(q.offset) : undefined,
          limit: q.limit ? Number(q.limit) : undefined,
        }),
      );
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threadRun, (c) => {
    try {
      const principal = c.get("principal");
      return c.json(
        getOrchestrator().getRun(
          principal,
          c.req.param("threadId"),
          c.req.param("runId"),
        ),
      );
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.post(AgentRoutes.threadRunCancel, async (c) => {
    try {
      const principal = c.get("principal");
      const run = await getOrchestrator().cancelRun(
        principal,
        c.req.param("threadId"),
        c.req.param("runId"),
      );
      return c.json({ runId: run.runId, status: run.status });
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.post(AgentRoutes.threadRunsStream, async (c) => {
    try {
      const principal = c.get("principal");
      const threadId = c.req.param("threadId");
      const body = (await c.req.json().catch(() => ({}))) as CreateRunRequest;
      const run = await getOrchestrator().createRun(principal, threadId, body, {
        stream: true,
      });
      c.header(
        "Content-Location",
        `${AGENT_API_PREFIX}/threads/${threadId}/runs/${run.runId}`,
      );
      return openRunSse(c, getOrchestrator(), principal, threadId, run.runId);
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.get(AgentRoutes.threadRunStream, (c) => {
    try {
      const principal = c.get("principal");
      const threadId = c.req.param("threadId");
      const runId = c.req.param("runId");
      getOrchestrator().getRun(principal, threadId, runId);
      const cursor =
        c.req.query("cursor") ??
        c.req.header("Last-Event-ID") ??
        c.req.query("lastEventId");
      return openRunSse(
        c,
        getOrchestrator(),
        principal,
        threadId,
        runId,
        cursor,
      );
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.post(AgentRoutes.threadSteer, async (c) => {
    try {
      const principal = c.get("principal");
      const body = (await c.req.json().catch(() => ({}))) as {
        input?: { text?: string };
      };
      await getOrchestrator().steer(principal, c.req.param("threadId"), {
        text: body.input?.text,
      });
      return c.json({ ok: true });
    } catch (err) {
      return respondError(c, err);
    }
  });

  v1.post(AgentRoutes.threadFollowUp, async (c) => {
    try {
      const principal = c.get("principal");
      const body = (await c.req.json().catch(() => ({}))) as {
        input?: { text?: string };
      };
      await getOrchestrator().followUp(principal, c.req.param("threadId"), {
        text: body.input?.text,
      });
      return c.json({ ok: true });
    } catch (err) {
      return respondError(c, err);
    }
  });

  // G6: MCP 目录查询
  mountMcpRoutes(v1 as never, getMcpCatalog);

  app.route(AGENT_API_PREFIX, v1);
}

function respondError(c: Context<AppEnv>, err: unknown): Response {
  let traceId: string | undefined;
  try {
    traceId = c.get("principal")?.traceId;
  } catch {
    // no principal
  }
  const agentErr = toAgentHttpError(err, traceId);
  return c.json(
    errorBody(
      agentErr.code,
      agentErr.message,
      agentErr.traceId ?? traceId,
      agentErr.details,
    ),
    agentErr.status as 400,
  );
}

/**
 * 打开 run 的 SSE 响应。
 *
 * 参数:
 *   - c: Hono Context。
 *   - orch: Orchestrator。
 *   - principal: 调用主体。
 *   - threadId / runId: 资源 id。
 *   - cursor: 可选重连游标。
 *
 * 返回值:
 *   - Response（text/event-stream 或错误 JSON）。
 */
function openRunSse(
  c: Context<AppEnv>,
  orch: Orchestrator,
  principal: Principal,
  threadId: string,
  runId: string,
  cursor?: string,
): Response {
  const buffer = orch.getStreamBuffer(runId);
  const replay = buffer.replayFrom(cursor);
  if (cursor && replay === null) {
    return c.json(
      errorBody(
        "AGENT_STREAM_CURSOR_INVALID",
        "stream cursor invalid",
        principal.traceId,
      ),
      400,
    );
  }

  const encoder = new TextEncoder();
  let unsubscribe: (() => void) | undefined;
  let closed = false;
  let pollTimer: ReturnType<typeof setInterval> | undefined;

  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const push = (frame: string) => {
        if (closed) return;
        try {
          controller.enqueue(encoder.encode(frame));
        } catch {
          // closed
        }
      };

      for (const frame of replay ?? []) {
        push(frame);
      }

      unsubscribe = buffer.subscribe({
        onEvent: (frame) => push(frame),
        onClose: () => {
          finish();
        },
      });

      const finish = () => {
        if (closed) return;
        closed = true;
        clearInterval(pollTimer);
        unsubscribe?.();
        try {
          controller.close();
        } catch {
          // ignore
        }
      };

      const checkDone = () => {
        try {
          const run = orch.getRun(principal, threadId, runId);
          const terminal = new Set([
            "success",
            "error",
            "cancelled",
            "interrupted",
          ]);
          if (terminal.has(run.status)) {
            setTimeout(finish, 30);
          }
        } catch {
          // ignore
        }
      };
      pollTimer = setInterval(checkDone, 50);
      if (typeof pollTimer === "object" && "unref" in pollTimer) {
        pollTimer.unref();
      }
      setTimeout(checkDone, 10);

      c.req.raw.signal.addEventListener("abort", () => {
        void orch.onClientDisconnect(principal, threadId, runId);
        finish();
      });
    },
    cancel() {
      closed = true;
      clearInterval(pollTimer);
      unsubscribe?.();
      void orch.onClientDisconnect(principal, threadId, runId);
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "content-type": "text/event-stream; charset=utf-8",
      "cache-control": "no-cache",
      connection: "keep-alive",
      "x-trace-id": principal.traceId,
    },
  });
}
