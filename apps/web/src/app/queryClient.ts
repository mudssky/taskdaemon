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
