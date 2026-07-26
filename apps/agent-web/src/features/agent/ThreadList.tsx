import type { Thread } from "@taskdaemon/agent-protocol";
import { useMemo, useState } from "react";
import { formatActivityTime, threadStatusLabel } from "./message-format";

type ThreadListProps = {
  threads: Thread[];
  selectedId: string | undefined;
  loading: boolean;
  errorMessage: string | null;
  onSelect: (threadId: string) => void;
  onCreate: () => void;
  onDelete: (threadId: string) => void;
  onRename: (threadId: string, title: string) => void;
  onArchive: (threadId: string, archived: boolean) => void;
  onFork: (threadId: string) => void;
  createPending: boolean;
  search: string;
  onSearchChange: (value: string) => void;
  includeArchived: boolean;
  onIncludeArchivedChange: (value: boolean) => void;
  sortDirection: "asc" | "desc";
  onSortDirectionChange: (value: "asc" | "desc") => void;
};

/**
 * 左侧会话列表（G5：搜索 / 归档 / 重命名 / fork）。
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
  onRename,
  onArchive,
  onFork,
  createPending,
  search,
  onSearchChange,
  includeArchived,
  onIncludeArchivedChange,
  sortDirection,
  onSortDirectionChange,
}: ThreadListProps) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState("");

  const countLabel = useMemo(
    () => `${threads.length} 个会话`,
    [threads.length],
  );

  return (
    <aside className="flex h-full w-80 flex-col border-r border-border bg-card">
      <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-3">
        <div>
          <div className="text-sm font-semibold">会话</div>
          <div className="text-xs text-muted-foreground">{countLabel}</div>
        </div>
        <button
          type="button"
          disabled={createPending}
          className="rounded-md border border-primary bg-primary px-2 py-1 text-xs font-semibold text-primary-foreground disabled:opacity-50"
          onClick={onCreate}
        >
          新建
        </button>
      </div>

      <div className="space-y-2 border-b border-border px-3 py-2">
        <input
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="搜索标题或内容…"
          className="w-full rounded border border-input bg-background px-2 py-1 text-xs"
        />
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <label className="flex items-center gap-1">
            <input
              type="checkbox"
              checked={includeArchived}
              onChange={(event) =>
                onIncludeArchivedChange(event.target.checked)
              }
            />
            含归档
          </label>
          <label className="flex items-center gap-1">
            排序
            <select
              className="rounded border border-input bg-background px-1 py-0.5"
              value={sortDirection}
              onChange={(event) =>
                onSortDirectionChange(event.target.value as "asc" | "desc")
              }
            >
              <option value="desc">最近优先</option>
              <option value="asc">最早优先</option>
            </select>
          </label>
        </div>
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
            const title = String(
              thread.metadata?.title ?? thread.threadId.slice(-8),
            );
            const selected = thread.threadId === selectedId;
            const archived = thread.status === "archived";
            return (
              <li key={thread.threadId}>
                <div
                  className={
                    selected
                      ? "rounded-md border border-primary bg-primary/10 p-2"
                      : "rounded-md border border-transparent p-2 hover:bg-muted/50"
                  }
                >
                  <button
                    type="button"
                    className="w-full text-left"
                    onClick={() => onSelect(thread.threadId)}
                  >
                    {editingId === thread.threadId ? null : (
                      <>
                        <div className="truncate text-sm font-medium">
                          {title}
                          {archived ? " · 归档" : ""}
                        </div>
                        <div className="mt-0.5 flex justify-between text-[11px] text-muted-foreground">
                          <span>{threadStatusLabel(thread.status)}</span>
                          <span>{formatActivityTime(thread.updatedAt)}</span>
                        </div>
                        <div className="mt-0.5 text-[11px] text-muted-foreground">
                          {thread.runtimeId} · {thread.profile}
                        </div>
                      </>
                    )}
                  </button>
                  {editingId === thread.threadId ? (
                    <form
                      className="flex gap-1"
                      onSubmit={(event) => {
                        event.preventDefault();
                        onRename(thread.threadId, editTitle.trim() || title);
                        setEditingId(null);
                      }}
                    >
                      <input
                        className="min-w-0 flex-1 rounded border border-input bg-background px-1 text-xs"
                        value={editTitle}
                        onChange={(event) => setEditTitle(event.target.value)}
                        // biome-ignore lint/a11y/noAutofocus: rename UX
                        autoFocus
                      />
                      <button
                        type="submit"
                        className="text-xs text-primary underline"
                      >
                        存
                      </button>
                    </form>
                  ) : (
                    <div className="mt-1 flex flex-wrap gap-1 text-[11px]">
                      <button
                        type="button"
                        className="underline"
                        onClick={() => {
                          setEditingId(thread.threadId);
                          setEditTitle(title);
                        }}
                      >
                        重命名
                      </button>
                      <button
                        type="button"
                        className="underline"
                        onClick={() => onFork(thread.threadId)}
                      >
                        分叉
                      </button>
                      <button
                        type="button"
                        className="underline"
                        onClick={() => onArchive(thread.threadId, !archived)}
                      >
                        {archived ? "恢复" : "归档"}
                      </button>
                      <button
                        type="button"
                        className="text-destructive underline"
                        onClick={() => {
                          if (
                            window.confirm(`删除会话「${title}」？不可恢复。`)
                          ) {
                            onDelete(thread.threadId);
                          }
                        }}
                      >
                        删除
                      </button>
                    </div>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </div>
    </aside>
  );
}
