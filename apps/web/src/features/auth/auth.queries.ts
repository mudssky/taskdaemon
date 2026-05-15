import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../lib/api/client";

export const authKeys = {
  me: ["auth", "me"] as const,
};

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
    onSuccess: () => queryClient.invalidateQueries({ queryKey: authKeys.me }),
  });
}
