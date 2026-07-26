import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      refetchOnWindowFocus: false,
    },
  },
});

/** Desktop 能力清单 query key 前缀（路线图 §9.2 / D1）。 */
export const desktopKeys = {
  all: ["desktop"] as const,
  capabilities: ["desktop", "capabilities"] as const,
} as const;

/** 设置页 query key 前缀（路线图 §9.2 / W1）；实现见 features/settings。 */
export { settingsKeys } from "../features/settings/settings.queries";
/**
 * 模板 query key 前缀（路线图 §9.2 / W3）定义在
 * `features/tasks/templates/templates.queries.ts` 的 `templateKeys`。
 * 此处仅登记归属，避免第二套 key。
 */
