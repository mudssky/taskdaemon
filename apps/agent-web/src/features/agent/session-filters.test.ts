import type { AgentMessage, Thread } from "@taskdaemon/agent-protocol";
import { describe, expect, it } from "vitest";
import {
  buildForkedThread,
  filterAndSortThreads,
  sliceMessagesForFork,
} from "./session-filters";

const baseThread = (id: string, overrides: Partial<Thread> = {}): Thread => ({
  threadId: id,
  status: "idle",
  runtimeId: "pi",
  profile: "coding",
  workspaceBound: true,
  createdAt: "2026-07-27T00:00:00.000Z",
  updatedAt: "2026-07-27T01:00:00.000Z",
  tenantId: "dev",
  subject: "dev",
  metadata: { title: `标题-${id}` },
  ...overrides,
});

describe("filterAndSortThreads", () => {
  it("默认隐藏归档并用 q 搜标题与内容", () => {
    const rows = [
      {
        thread: baseThread("a", {
          updatedAt: "2026-07-27T02:00:00.000Z",
          metadata: { title: "Alpha plan" },
        }),
        searchableText: "hello world",
      },
      {
        thread: baseThread("b", {
          status: "archived",
          updatedAt: "2026-07-27T03:00:00.000Z",
          metadata: { title: "Beta" },
        }),
        searchableText: "secret",
      },
      {
        thread: baseThread("c", {
          updatedAt: "2026-07-27T00:30:00.000Z",
          metadata: { title: "Gamma" },
        }),
        searchableText: "plan details",
      },
    ];

    const filtered = filterAndSortThreads(rows, { q: "plan" });
    expect(filtered.map((t) => t.threadId)).toEqual(["a", "c"]);

    const withArchived = filterAndSortThreads(rows, {
      includeArchived: true,
      q: "beta",
    });
    expect(withArchived.map((t) => t.threadId)).toEqual(["b"]);
  });

  it("按 updated_at 升序排序", () => {
    const rows = [
      {
        thread: baseThread("new", {
          updatedAt: "2026-07-27T03:00:00.000Z",
        }),
      },
      {
        thread: baseThread("old", {
          updatedAt: "2026-07-27T01:00:00.000Z",
        }),
      },
    ];
    const sorted = filterAndSortThreads(rows, {
      sort: { field: "updated_at", direction: "asc" },
    });
    expect(sorted.map((t) => t.threadId)).toEqual(["old", "new"]);
  });
});

describe("fork helpers", () => {
  it("sliceMessagesForFork 含分叉点且不改源数组", () => {
    const messages: AgentMessage[] = [
      {
        id: "m1",
        role: "user",
        content: [{ type: "text", text: "1" }],
        createdAt: "t",
      },
      {
        id: "m2",
        role: "assistant",
        content: [{ type: "text", text: "2" }],
        createdAt: "t",
      },
      {
        id: "m3",
        role: "user",
        content: [{ type: "text", text: "3" }],
        createdAt: "t",
      },
    ];
    const sliced = sliceMessagesForFork(messages, "m2");
    expect(sliced.map((m) => m.id)).toEqual(["m1", "m2"]);
    expect(messages).toHaveLength(3);
    sliced[0]!.content[0] = { type: "text", text: "mutated" };
    expect(
      messages[0]!.content[0] && messages[0]!.content[0].type === "text"
        ? messages[0]!.content[0].text
        : "",
    ).toBe("1");
  });

  it("buildForkedThread 生成新 id 与 forkedFrom，源不变", () => {
    const source = baseThread("src");
    const forked = buildForkedThread(
      source,
      "dst",
      "Fork title",
      "2026-07-27T10:00:00.000Z",
    );
    expect(forked.threadId).toBe("dst");
    expect(forked.metadata?.title).toBe("Fork title");
    expect(forked.metadata?.forkedFrom).toBe("src");
    expect(source.threadId).toBe("src");
    expect(source.metadata?.forkedFrom).toBeUndefined();
  });
});
