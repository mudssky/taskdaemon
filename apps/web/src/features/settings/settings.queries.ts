import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../lib/api/client";
import type {
  AudioConfig,
  AudioConfigWriteRequest,
  ConfigSectionWriteResponse,
} from "../../lib/api/types";

/**
 * 设置页 query key 命名空间（路线图 §9.2 / W1）。
 */
export const settingsKeys = {
  all: ["settings"] as const,
  config: (section: string) => ["settings", "config", section] as const,
};

/**
 * 读取 audio section 安全配置。
 *
 * 返回值:
 *   - useQuery 结果
 */
export function useAudioSectionConfigQuery() {
  return useQuery({
    queryKey: settingsKeys.config("audio"),
    queryFn: apiClient.audioConfig,
  });
}

/**
 * 写入配置 section（首版 audio）。
 *
 * 返回值:
 *   - useMutation 结果
 */
export function usePutConfigSectionMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      section: string;
      body: AudioConfigWriteRequest;
    }): Promise<ConfigSectionWriteResponse> =>
      apiClient.putConfigSection(input.section, input.body),
    onSuccess: (data, variables) => {
      const key = settingsKeys.config(variables.section);
      if (variables.section === "audio" && data.config) {
        queryClient.setQueryData<AudioConfig>(key, data.config);
      } else {
        void queryClient.invalidateQueries({ queryKey: key });
      }
      // 兼容旧 audioKeys.config 缓存（若仍有读者）
      void queryClient.invalidateQueries({ queryKey: ["audio", "config"] });
    },
  });
}
