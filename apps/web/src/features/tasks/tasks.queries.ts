import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../lib/api/client";
import type { TaskPayload } from "../../lib/api/types";

export const tasksKeys = {
  all: ["tasks"] as const,
  runs: (taskId: number) => ["tasks", taskId, "runs"] as const,
};

export function useTasksQuery() {
  return useQuery({
    queryKey: tasksKeys.all,
    queryFn: async () => (await apiClient.listTasks()).tasks,
    refetchInterval: (query) => {
      const hasRunning =
        query.state.data?.some((task) => task.running) ?? false;
      return hasRunning ? 3000 : false;
    },
  });
}

export function useRunHistoryQuery(taskId: number | null) {
  return useQuery({
    queryKey: taskId ? tasksKeys.runs(taskId) : ["tasks", "runs", "none"],
    queryFn: async () => (await apiClient.listTaskRuns(taskId ?? 0)).runs,
    enabled: taskId !== null,
    refetchInterval: (query) => {
      const hasRunning =
        query.state.data?.some((run) => run.status === "running") ?? false;
      return hasRunning ? 3000 : false;
    },
  });
}

export function useCreateTaskMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: TaskPayload) => apiClient.createTask(payload),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tasksKeys.all }),
  });
}

export function useUpdateTaskMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      taskId,
      payload,
    }: {
      taskId: number;
      payload: TaskPayload;
    }) => apiClient.updateTask(taskId, payload),
    onSuccess: (_task, variables) => {
      queryClient.invalidateQueries({ queryKey: tasksKeys.all });
      queryClient.invalidateQueries({
        queryKey: tasksKeys.runs(variables.taskId),
      });
    },
  });
}

export function useSetTaskEnabledMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ taskId, enabled }: { taskId: number; enabled: boolean }) =>
      apiClient.setTaskEnabled(taskId, enabled),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tasksKeys.all }),
  });
}

export function useTriggerTaskMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (taskId: number) => apiClient.triggerTask(taskId),
    onSuccess: (_run, taskId) => {
      queryClient.invalidateQueries({ queryKey: tasksKeys.all });
      queryClient.invalidateQueries({ queryKey: tasksKeys.runs(taskId) });
    },
  });
}

export function useCancelTaskMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (taskId: number) => apiClient.cancelTask(taskId),
    onSuccess: (_run, taskId) => {
      queryClient.invalidateQueries({ queryKey: tasksKeys.all });
      queryClient.invalidateQueries({ queryKey: tasksKeys.runs(taskId) });
    },
  });
}
