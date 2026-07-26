import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      refetchOnWindowFocus: false,
    },
  },
});

/**
 * Agent 前端 query key 命名空间（路线图 §9.2 / G4）。
 * 禁止与 apps/web 的 tasks/desktop 等前缀混用。
 */
export const agentKeys = {
  all: ["agent"] as const,
  threads: (params?: Record<string, unknown>) =>
    ["agent", "threads", params ?? {}] as const,
  thread: (threadId: string) => ["agent", "thread", threadId] as const,
  messages: (threadId: string, params?: Record<string, unknown>) =>
    ["agent", "messages", threadId, params ?? {}] as const,
  capabilities: (threadId: string) =>
    ["agent", "capabilities", threadId] as const,
  runtimes: ["agent", "runtimes"] as const,
} as const;
