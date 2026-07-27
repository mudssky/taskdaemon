/**
 * 会话列表过滤 / 排序 / fork 截断（纯函数，G5）。
 */

import type {
  AgentMessage,
  ListThreadsQuery,
  Thread,
  ThreadStatus,
} from "@taskdaemon/agent-protocol";

export type ThreadFilterInput = {
  q?: string;
  status?: ThreadStatus;
  runtimeId?: string;
  profile?: ListThreadsQuery["profile"];
  includeArchived?: boolean;
  sort?: ListThreadsQuery["sort"];
};

export type ThreadWithPreview = {
  thread: Thread;
  /** 可选：用于内容搜索的拼接文本。 */
  searchableText?: string;
};

/**
 * 过滤并排序会话列表。
 *
 * 参数:
 *   - items: 带可选可搜索正文的会话。
 *   - filter: 查询条件。
 *
 * 返回值:
 *   - 过滤后的 Thread 数组。
 */
export function filterAndSortThreads(
  items: ThreadWithPreview[],
  filter: ThreadFilterInput = {},
): Thread[] {
  const q = filter.q?.trim().toLowerCase() ?? "";
  let rows = [...items];

  if (!filter.includeArchived) {
    rows = rows.filter((row) => row.thread.status !== "archived");
  }
  if (filter.status) {
    rows = rows.filter((row) => row.thread.status === filter.status);
  }
  if (filter.runtimeId) {
    rows = rows.filter((row) => row.thread.runtimeId === filter.runtimeId);
  }
  if (filter.profile) {
    rows = rows.filter((row) => row.thread.profile === filter.profile);
  }
  if (q) {
    rows = rows.filter((row) => {
      const title = String(row.thread.metadata?.title ?? "").toLowerCase();
      const body = (row.searchableText ?? "").toLowerCase();
      return title.includes(q) || body.includes(q);
    });
  }

  const field = filter.sort?.field ?? "updated_at";
  const direction = filter.sort?.direction ?? "desc";
  const mul = direction === "asc" ? 1 : -1;
  rows.sort((a, b) => {
    const av =
      field === "created_at"
        ? new Date(a.thread.createdAt).getTime()
        : new Date(a.thread.updatedAt).getTime();
    const bv =
      field === "created_at"
        ? new Date(b.thread.createdAt).getTime()
        : new Date(b.thread.updatedAt).getTime();
    return (av - bv) * mul;
  });

  return rows.map((row) => row.thread);
}

/**
 * 按 fromMessageId 截断消息（含该消息）；找不到则复制全部。
 *
 * 参数:
 *   - messages: 源会话消息。
 *   - fromMessageId: 分叉点。
 *
 * 返回值:
 *   - 新数组（浅拷贝 content）。
 */
export function sliceMessagesForFork(
  messages: AgentMessage[],
  fromMessageId?: string,
): AgentMessage[] {
  if (!fromMessageId) {
    return messages.map(cloneMessage);
  }
  const idx = messages.findIndex((m) => m.id === fromMessageId);
  if (idx < 0) {
    return messages.map(cloneMessage);
  }
  return messages.slice(0, idx + 1).map(cloneMessage);
}

/**
 * 从源会话构造 fork 后的 Thread 元数据（不改源）。
 *
 * 参数:
 *   - source: 源 thread。
 *   - newThreadId: 新 id。
 *   - title: 可选标题。
 *   - nowIso: 时间戳。
 *
 * 返回值:
 *   - 新 Thread。
 */
export function buildForkedThread(
  source: Thread,
  newThreadId: string,
  title: string | undefined,
  nowIso: string,
): Thread {
  return {
    ...source,
    threadId: newThreadId,
    status: "idle",
    createdAt: nowIso,
    updatedAt: nowIso,
    metadata: {
      ...source.metadata,
      title:
        title ??
        `Fork · ${String(source.metadata?.title ?? source.threadId.slice(-6))}`,
      forkedFrom: source.threadId,
    },
  };
}

function cloneMessage(message: AgentMessage): AgentMessage {
  return {
    ...message,
    content: message.content.map((block) => ({ ...block })),
    metadata: message.metadata ? { ...message.metadata } : undefined,
  };
}
