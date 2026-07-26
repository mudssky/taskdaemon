import type { Thread } from "@taskdaemon/agent-protocol";
import { formatActivityTime, threadStatusLabel } from "./message-format";

type ThreadListProps = {
  threads: Thread[];
  selectedId: string | undefined;
  loading: boolean;
  errorMessage: string | null;
  onSelect: (threadId: string) => void;
  onCreate: () => void;
  onDelete: (threadId: string) => void;
  createPending: boolean;
};

/**
 * 左侧会话列表。
 *
 * 参数:
 *   - props: 列表数据与回调。
 *
 * 返回值:
 *   - 侧栏节点。
 */
export function ThreadList({
  threads,
  selectedId,
  loading,
  errorMessage,
  onSelect,
  onCreate,
  onDelete,
  createPending,
}: ThreadListProps) {
  return (
    <aside className="flex h-full w-72 flex-col border-r border-border bg-card">
      <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-3">
        <div>
          <div className="text-sm font-bold">会话</div>
          <div className="text-xs text-muted-foreground">Agent Protocol</div>
        </div>
        <button
          type="button"
          className="rounded-md border border-primary bg-primary px-2 py-1 text-xs font-semibold text-primary-foreground disabled:opacity-50"
          onClick={onCreate}
          disabled={createPending}
        >
          新建
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-2">
        {loading ? (
          <p className="px-2 py-4 text-sm text-muted-foreground">加载中…</p>
        ) : null}
        {errorMessage ? (
          <p className="px-2 py-4 text-sm text-destructive">{errorMessage}</p>
        ) : null}
        {!loading && !errorMessage && threads.length === 0 ? (
          <p className="px-2 py-4 text-sm text-muted-foreground">暂无会话</p>
        ) : null}
        <ul className="flex flex-col gap-1">
          {threads.map((thread) => {
            const active = thread.threadId === selectedId;
            const title = String(
              thread.metadata?.title ?? thread.threadId.slice(0, 12),
            );
            return (
              <li key={thread.threadId}>
                <div
                  className={
                    active
                      ? "rounded-md border border-ring bg-accent px-2 py-2"
                      : "rounded-md border border-transparent px-2 py-2 hover:bg-muted"
                  }
                >
                  <button
                    type="button"
                    className="w-full text-left"
                    onClick={() => onSelect(thread.threadId)}
                  >
                    <div className="truncate text-sm font-semibold">
                      {title}
                    </div>
                    <div className="mt-0.5 flex flex-wrap gap-x-2 text-[11px] text-muted-foreground">
                      <span>{thread.runtimeId}</span>
                      <span>{thread.profile}</span>
                      <span>{threadStatusLabel(thread.status)}</span>
                      <span>{formatActivityTime(thread.updatedAt)}</span>
                    </div>
                  </button>
                  <button
                    type="button"
                    className="mt-1 text-[11px] text-destructive underline"
                    onClick={() => {
                      const ok = window.confirm(`确认删除会话「${title}」？`);
                      if (ok) {
                        onDelete(thread.threadId);
                      }
                    }}
                  >
                    删除
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      </div>
    </aside>
  );
}
