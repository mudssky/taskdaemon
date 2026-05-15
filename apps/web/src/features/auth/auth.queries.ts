import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../lib/api/client";

export const authKeys = {
  status: ["auth", "status"] as const,
  me: ["auth", "me"] as const,
};

export function useAuthStatusQuery() {
  return useQuery({
    queryKey: authKeys.status,
    queryFn: apiClient.authStatus,
    retry: false,
  });
}

export function useSessionQuery() {
  return useQuery({
    queryKey: authKeys.me,
    queryFn: apiClient.me,
    retry: false,
  });
}

export function useLoginMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      username,
      password,
    }: {
      username: string;
      password: string;
    }) => apiClient.login(username, password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.status });
      queryClient.invalidateQueries({ queryKey: authKeys.me });
    },
  });
}

export function useInitializeAdminMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      username,
      password,
    }: {
      username: string;
      password: string;
    }) => apiClient.initializeAdmin(username, password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.status });
      queryClient.invalidateQueries({ queryKey: authKeys.me });
    },
  });
}
