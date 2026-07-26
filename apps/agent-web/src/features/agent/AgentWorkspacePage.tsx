import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { AgentApiError } from "../../lib/api/errors";
import { CapabilityBar } from "./CapabilityBar";
import { Composer } from "./Composer";
import { ThreadList } from "./ThreadList";
import { Timeline } from "./Timeline";
import {
  useCapabilitiesQuery,
  useCreateThreadMutation,
  useDeleteThreadMutation,
  useMessagesQuery,
  useRuntimesQuery,
  useThreadQuery,
  useThreadsQuery,
} from "./threads.queries";
import { useRunStream } from "./useRunStream";

/**
 * Agent 主工作台：会话列表 + 聊天 + 能力驱动控件。
 *
 * 参数: 无（路由参数 threadId 可选）。
 *
 * 返回值:
 *   - 页面节点。
 */
export function AgentWorkspacePage() {
  const params = useParams({ strict: false }) as { threadId?: string };
  const navigate = useNavigate();
  const selectedId = params.threadId;

  const threadsQuery = useThreadsQuery({ limit: 50 });
  const runtimesQuery = useRuntimesQuery();
  const threadQuery = useThreadQuery(selectedId);
  const messagesQuery = useMessagesQuery(selectedId);
  const capsQuery = useCapabilitiesQuery(selectedId);
  const createMutation = useCreateThreadMutation();
  const deleteMutation = useDeleteThreadMutation();

  const [model, setModel] = useState("mock-model");
  const [createRuntimeId, setCreateRuntimeId] = useState("pi");

  useEffect(() => {
    if (threadQuery.data?.model) {
      setModel(threadQuery.data.model);
    }
  }, [threadQuery.data?.model]);

  const stream = useRunStream({
    threadId: selectedId,
    capabilities: capsQuery.data?.capabilities,
    historyMessages: messagesQuery.data?.items,
  });

  const scrollRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);
  const lastItem = stream.state.items.at(-1);
  const timelineTail =
    lastItem?.kind === "message"
      ? `${lastItem.id}:${lastItem.text.length}:${lastItem.streaming}:${stream.state.status}`
      : lastItem?.kind === "tool"
        ? `${lastItem.id}:${lastItem.status}:${lastItem.result?.length ?? 0}`
        : `${stream.state.items.length}:${stream.state.status}`;

  // 新消息时：用户在底部则自动滚到底；上滑阅读则不抢焦点。
  useEffect(() => {
    // timelineTail 仅作变更信号，驱动滚动。
    if (timelineTail.length < 0) {
      return;
    }
    const el = scrollRef.current;
    if (!el || !stickToBottomRef.current) {
      return;
    }
    el.scrollTop = el.scrollHeight;
  }, [timelineTail]);

  const listError =
    threadsQuery.error instanceof Error
      ? threadsQuery.error.message
      : threadsQuery.error
        ? String(threadsQuery.error)
        : null;

  const runError = stream.state.error;
  const pageError = useMemo(() => {
    const err =
      capsQuery.error ?? messagesQuery.error ?? threadQuery.error ?? null;
    if (!err) {
      return null;
    }
    if (err instanceof AgentApiError) {
      return {
        message: err.message,
        code: err.code,
        traceId: err.traceId,
      };
    }
    return {
      message: err instanceof Error ? err.message : String(err),
      code: undefined,
      traceId: null as string | null,
    };
  }, [capsQuery.error, messagesQuery.error, threadQuery.error]);

  return (
    <div className="flex h-screen min-h-0 bg-background text-foreground">
      <ThreadList
        threads={threadsQuery.data?.items ?? []}
        selectedId={selectedId}
        loading={threadsQuery.isLoading}
        errorMessage={listError}
        createPending={createMutation.isPending}
        onSelect={(threadId) => {
          void navigate({ to: "/threads/$threadId", params: { threadId } });
        }}
        onCreate={() => {
          createMutation.mutate(
            {
              runtimeId: createRuntimeId,
              profile: createRuntimeId === "omp" ? "general" : "coding",
              workspaceBinding: createRuntimeId !== "omp",
              model: "mock-model",
              metadata: {
                title: `新会话 · ${createRuntimeId} · ${new Date().toLocaleTimeString()}`,
              },
            },
            {
              onSuccess: (thread) => {
                void navigate({
                  to: "/threads/$threadId",
                  params: { threadId: thread.threadId },
                });
              },
            },
          );
        }}
        onDelete={(threadId) => {
          deleteMutation.mutate(threadId, {
            onSuccess: () => {
              if (selectedId === threadId) {
                void navigate({ to: "/" });
              }
            },
          });
        }}
      />

      <section className="flex min-w-0 flex-1 flex-col">
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-muted/40 px-3 py-2 text-xs">
          <label className="flex items-center gap-1">
            新建时 runtime
            <select
              className="rounded border border-input bg-background px-2 py-1"
              value={createRuntimeId}
              onChange={(event) => setCreateRuntimeId(event.target.value)}
            >
              {(
                runtimesQuery.data ?? [
                  { runtimeId: "pi", label: "pi" },
                  { runtimeId: "omp", label: "omp" },
                ]
              ).map((runtime) => (
                <option key={runtime.runtimeId} value={runtime.runtimeId}>
                  {runtime.label ?? runtime.runtimeId}
                </option>
              ))}
            </select>
          </label>
          <span className="text-muted-foreground">
            模式：{import.meta.env.VITE_AGENT_API_MODE ?? "mock"}
          </span>
          <RunStatusBadge status={stream.state.status} />
          {stream.state.connection === "disconnected" ? (
            <button
              type="button"
              className="rounded border border-warning-border bg-warning px-2 py-0.5 text-warning-foreground"
              onClick={() => {
                stream.reconnectHint();
                void messagesQuery.refetch();
                void capsQuery.refetch();
              }}
            >
              连接断开 · 点此重载历史
            </button>
          ) : null}
        </div>

        <CapabilityBar
          thread={threadQuery.data}
          capabilities={capsQuery.data?.capabilities}
          model={model}
          onModelChange={setModel}
        />

        {(pageError || runError) && (
          <div className="border-b border-destructive-border bg-destructive-soft px-4 py-2 text-sm text-destructive-soft-foreground">
            <div>{pageError?.message ?? runError?.message}</div>
            <div className="text-xs opacity-80">
              code: {pageError?.code ?? runError?.code ?? "—"} · traceId:{" "}
              {pageError?.traceId ?? runError?.traceId ?? "—"}
            </div>
          </div>
        )}

        <div
          ref={scrollRef}
          className="min-h-0 flex-1 overflow-y-auto"
          onScroll={(event) => {
            const el = event.currentTarget;
            const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
            stickToBottomRef.current = distance < 80;
          }}
        >
          {!selectedId ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              选择左侧会话，或新建一个。
            </div>
          ) : messagesQuery.isLoading || capsQuery.isLoading ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              加载会话…
            </div>
          ) : (
            <Timeline
              state={stream.state}
              capabilities={capsQuery.data?.capabilities}
              onToggleTool={stream.onToggleTool}
            />
          )}
        </div>

        <Composer
          disabled={!selectedId}
          running={stream.state.status === "running"}
          onSend={(text) => {
            void stream.send(text, model);
          }}
          onCancel={() => {
            void stream.cancel();
          }}
        />
      </section>
    </div>
  );
}

function RunStatusBadge({
  status,
}: {
  status: "idle" | "running" | "error" | "cancelled";
}) {
  const label =
    status === "idle"
      ? "空闲"
      : status === "running"
        ? "运行中"
        : status === "error"
          ? "出错"
          : "已中断";
  return (
    <span className="rounded-full border border-border bg-background px-2 py-0.5">
      运行状态：{label}
    </span>
  );
}
