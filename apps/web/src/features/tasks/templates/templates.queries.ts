import { useMutation, useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api/client";
import type { TemplateRenderRequest } from "../../../lib/api/types";

/**
 * 路线图 §9.2 分配的模板 query key 前缀（W3 / T7b）。
 */
export const templateKeys = {
  all: ["templates"] as const,
  list: () => [...templateKeys.all, "list"] as const,
  detail: (id: string) => [...templateKeys.all, "detail", id] as const,
} as const;

/**
 * 模板列表 query。
 *
 * @returns TanStack Query 结果；data 为 templates 数组。
 */
export function useTemplatesQuery() {
  return useQuery({
    queryKey: templateKeys.list(),
    queryFn: async () => {
      const data = await apiClient.listTemplates();
      return data.templates;
    },
  });
}

/**
 * 单个模板详情 query。
 *
 * @param id - 模板 id；null 时禁用。
 * @returns 模板定义。
 */
export function useTemplateDetailQuery(id: string | null) {
  return useQuery({
    queryKey: templateKeys.detail(id ?? ""),
    queryFn: () => apiClient.getTemplate(id as string),
    enabled: Boolean(id),
  });
}

/**
 * 渲染任务草稿 mutation（不落库）。
 *
 * @returns mutation；variables 为 { id, body }。
 */
export function useRenderTemplateMutation() {
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: TemplateRenderRequest }) =>
      apiClient.renderTemplate(id, body),
  });
}
