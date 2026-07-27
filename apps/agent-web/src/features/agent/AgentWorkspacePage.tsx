import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { AgentApiError } from "../../lib/api/errors";
import { CapabilityBar } from "./CapabilityBar";
import { Composer } from "./Composer";
import { shouldShowWorkspaceUi, steeringControl } from "./capability-ui";
import { runStatusLabel } from "./hitl";
import { ThreadList } from "./ThreadList";
import { Timeline } from "./Timeline";
import {
  useCapabilitiesQuery,
  useCreateThreadMutation,
  useDeleteThreadMutation,
  useForkThreadMutation,
  useMessagesQuery,
  useRuntimesQuery,
  useThreadQuery,
  useThreadsQuery,
  useUpdateThreadMutation,
} from "./threads.queries";
import { useRunStream } from "./useRunStream";
import { WorkspacePanel } from "./WorkspacePanel";

/**
 * Agent 主工作台：会话管理 + 聊天 + 能力驱动控件 + G5 完整体验。
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

  const [search, setSearch] = useState("");
  const [includeArchived, setIncludeArchived] = useState(false);
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");
  const [model, setModel] = useState("mock-model");
  const [createRuntimeId, setCreateRuntimeId] = useState("pi");

  const listQuery: Parameters<typeof useThreadsQuery>[0] = {
    limit: 50,
    q: search || undefined,
    includeArchived,
    sort: { field: "updated_at", direction: sortDirection },
  };

  const threadsQuery = useThreadsQuery(listQuery);
  const runtimesQuery = useRuntimesQuery();
  const threadQuery = useThreadQuery(selectedId);
  const messagesQuery = useMessagesQuery(selectedId);
  const capsQuery = useCapabilitiesQuery(selectedId);
  const createMutation = useCreateThreadMutation();
  const deleteMutation = useDeleteThreadMutation();
  const updateMutation = useUpdateThreadMutation();
  const forkMutation = useForkThreadMutation();

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
        : lastItem?.kind === "hitl"
          ? `${lastItem.id}:${lastItem.status}`
          : `${stream.state.items.length}:${stream.state.status}:${stream.state.connection}`;

  useEffect(() => {
    void timelineTail;
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

  const steerCtl = steeringControl(capsQuery.data?.capabilities);
  const showWorkspace = shouldShowWorkspaceUi(
    threadQuery.data,
    capsQuery.data?.capabilities,
  );

  return (
    <div className="flex h-screen min-h-0 bg-background text-foreground">
      <ThreadList
        threads={threadsQuery.data?.items ?? []}
        selectedId={selectedId}
        loading={threadsQuery.isLoading}
        errorMessage={listError}
        createPending={createMutation.isPending}
        search={search}
        onSearchChange={setSearch}
        includeArchived={includeArchived}
        onIncludeArchivedChange={setIncludeArchived}
        sortDirection={sortDirection}
        onSortDirectionChange={setSortDirection}
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
        onRename={(threadId, title) => {
          updateMutation.mutate({ threadId, body: { title } });
        }}
        onArchive={(threadId, archived) => {
          updateMutation.mutate({ threadId, body: { archived } });
        }}
        onFork={(threadId) => {
          forkMutation.mutate(
            { threadId },
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
          <ConnectionBadge
            connection={stream.state.connection}
            onReload={() => {
              stream.reconnectHint();
              void messagesQuery.refetch();
              void capsQuery.refetch();
            }}
            onReconnect={() => stream.manualReconnect()}
          />
          {stream.state.usage ? (
            <span className="text-muted-foreground">
              tokens: {stream.state.usage.totalTokens ?? "—"}
            </span>
          ) : null}
        </div>

        <CapabilityBar
          thread={threadQuery.data}
          capabilities={capsQuery.data?.capabilities}
          model={model}
          onModelChange={setModel}
        />

        <WorkspacePanel
          thread={threadQuery.data}
          fileChanges={stream.state.fileChanges}
          visible={showWorkspace}
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
              onToggleThinking={stream.onToggleThinking}
              onHitl={(requestId, decision, modifiedText) => {
                void stream.respondHitl(requestId, decision, modifiedText);
              }}
            />
          )}
        </div>

        <Composer
          disabled={!selectedId}
          running={
            stream.state.status === "running" ||
            stream.state.status === "awaiting_human"
          }
          awaitingHuman={stream.state.status === "awaiting_human"}
          steeringEnabled={steerCtl.visible && steerCtl.enabled}
          onSend={(text) => {
            void stream.send(text, model);
          }}
          onSteer={(text) => {
            void stream.steer(text);
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
  status: "idle" | "running" | "error" | "cancelled" | "awaiting_human";
}) {
  const highlight = status === "awaiting_human";
  return (
    <span
      className={
        highlight
          ? "rounded-full border border-warning-border bg-warning px-2 py-0.5 font-semibold text-warning-foreground"
          : "rounded-full border border-border bg-background px-2 py-0.5"
      }
    >
      运行状态：{runStatusLabel(status)}
    </span>
  );
}

function ConnectionBadge({
  connection,
  onReload,
  onReconnect,
}: {
  connection: StreamViewStateConnection;
  onReload: () => void;
  onReconnect: () => void;
}) {
  if (connection === "connected" || connection === "connecting") {
    return (
      <span className="text-muted-foreground">
        连接：{connection === "connecting" ? "连接中" : "正常"}
      </span>
    );
  }
  if (connection === "reconnecting") {
    return (
      <span className="rounded border border-warning-border bg-warning px-2 py-0.5 text-warning-foreground">
        正在重连并续传…
      </span>
    );
  }
  if (connection === "cursor_invalid") {
    return (
      <button
        type="button"
        className="rounded border border-destructive-border bg-destructive-soft px-2 py-0.5 text-destructive-soft-foreground"
        onClick={onReload}
      >
        游标失效 · 点此刷新历史后继续
      </button>
    );
  }
  return (
    <div className="flex gap-1">
      <button
        type="button"
        className="rounded border border-warning-border bg-warning px-2 py-0.5 text-warning-foreground"
        onClick={onReconnect}
      >
        连接断开 · 尝试续传
      </button>
      <button
        type="button"
        className="rounded border border-border px-2 py-0.5"
        onClick={onReload}
      >
        重载历史
      </button>
    </div>
  );
}

type StreamViewStateConnection =
  | "connected"
  | "connecting"
  | "reconnecting"
  | "disconnected"
  | "cursor_invalid";
