/**
 * Thread / capabilities / messages 的 TanStack Query 封装。
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  CreateThreadRequest,
  ForkThreadRequest,
  ListThreadsQuery,
  UpdateThreadRequest,
} from "@taskdaemon/agent-protocol";
import { agentKeys } from "../../app/queryClient";
import { getAgentApi } from "../../lib/api/agent-api";

/**
 * 可用 runtime 目录（创建会话时选择）。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - useQuery 结果。
 */
export function useRuntimesQuery() {
  return useQuery({
    queryKey: agentKeys.runtimes,
    queryFn: () => getAgentApi().listRuntimes(),
  });
}

/**
 * 会话列表。
 *
 * 参数:
 *   - query: 分页/过滤。
 *
 * 返回值:
 *   - useQuery 结果。
 */
export function useThreadsQuery(query: ListThreadsQuery = {}) {
  return useQuery({
    queryKey: agentKeys.threads(query as Record<string, unknown>),
    queryFn: () => getAgentApi().listThreads(query),
  });
}

/**
 * 单会话元数据。
 *
 * 参数:
 *   - threadId: 会话 id；空则 disabled。
 *
 * 返回值:
 *   - useQuery 结果。
 */
export function useThreadQuery(threadId: string | undefined) {
  return useQuery({
    queryKey: agentKeys.thread(threadId ?? ""),
    queryFn: () => getAgentApi().getThread(threadId as string),
    enabled: Boolean(threadId),
  });
}

/**
 * 会话消息历史。
 *
 * 参数:
 *   - threadId: 会话 id。
 *
 * 返回值:
 *   - useQuery 结果。
 */
export function useMessagesQuery(threadId: string | undefined) {
  return useQuery({
    queryKey: agentKeys.messages(threadId ?? ""),
    queryFn: () =>
      getAgentApi().listMessages(threadId as string, { limit: 200 }),
    enabled: Boolean(threadId),
  });
}

/**
 * 会话能力协商。
 *
 * 参数:
 *   - threadId: 会话 id。
 *
 * 返回值:
 *   - useQuery 结果。
 */
export function useCapabilitiesQuery(threadId: string | undefined) {
  return useQuery({
    queryKey: agentKeys.capabilities(threadId ?? ""),
    queryFn: () => getAgentApi().getCapabilities(threadId as string),
    enabled: Boolean(threadId),
  });
}

/**
 * 创建会话 mutation。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - useMutation。
 */
export function useCreateThreadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateThreadRequest) => getAgentApi().createThread(body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: agentKeys.all });
    },
  });
}

/**
 * 删除会话 mutation。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - useMutation。
 */
export function useDeleteThreadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => getAgentApi().deleteThread(threadId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: agentKeys.all });
    },
  });
}

/**
 * 更新会话（重命名 / 归档）。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - useMutation。
 */
export function useUpdateThreadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: { threadId: string; body: UpdateThreadRequest }) =>
      getAgentApi().updateThread(args.threadId, args.body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: agentKeys.all });
    },
  });
}

/**
 * 分叉会话。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - useMutation。
 */
export function useForkThreadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: { threadId: string; body?: ForkThreadRequest }) =>
      getAgentApi().forkThread(args.threadId, args.body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: agentKeys.all });
    },
  });
}
