import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../lib/api/client";

export const audioKeys = {
  history: ["audio", "history"] as const,
};

export function useAudioHistoryQuery(limit = 50) {
  return useQuery({
    queryKey: [...audioKeys.history, limit] as const,
    queryFn: async () => (await apiClient.listAudioHistory(limit)).records,
    refetchInterval: (query) => {
      const hasActive =
        query.state.data?.some(
          (record) => record.status === "queued" || record.status === "playing",
        ) ?? false;
      return hasActive ? 2000 : false;
    },
  });
}

export function useReplayAudioMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (recordId: number) => apiClient.replayAudioRecord(recordId),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: audioKeys.history }),
  });
}
