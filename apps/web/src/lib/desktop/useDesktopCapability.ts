/**
 * Desktop capability 的 React 入口 hook。
 */

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { desktopKeys } from "../../app/queryClient";
import {
  type DesktopInvokeResult,
  invokeDesktopCapability,
  listDesktopCapabilities,
} from "./bridge";
import {
  type CapabilityState,
  type DesktopCapabilityName,
  type InvokeOutcome,
  mapUnavailableReason,
  type UnavailableReason,
} from "./capabilities";
import { isDesktopRuntime } from "./platform";

const CAPABILITY_STALE_MS = 60_000;
/** 权限类状态短 TTL，用户可能在系统设置里改授权。 */
const PERMISSION_STALE_MS = 5_000;

/**
 * 业务组件消费 Desktop 能力的唯一入口。
 *
 * 参数:
 *   - name: DesktopCapability 具名常量。
 *
 * 返回值:
 *   - CapabilityState: available | unavailable | checking。
 */
export function useDesktopCapability<TResult = void>(
  name: DesktopCapabilityName,
): CapabilityState<TResult> {
  const desktop = isDesktopRuntime();

  const catalogQuery = useQuery({
    queryKey: desktopKeys.capabilities,
    queryFn: listDesktopCapabilities,
    enabled: desktop,
    staleTime: CAPABILITY_STALE_MS,
    retry: false,
  });

  const invoke = useCallback(
    async (input?: unknown): Promise<InvokeOutcome<TResult>> => {
      const result: DesktopInvokeResult<TResult> =
        await invokeDesktopCapability<TResult>(name, input);
      if (result.ok) {
        return { ok: true, data: result.data };
      }
      return {
        ok: false,
        code: result.code,
        message: result.message,
      };
    },
    [name],
  );

  return useMemo((): CapabilityState<TResult> => {
    if (!desktop) {
      return {
        status: "unavailable",
        reason: "not-desktop",
        message: "当前运行在浏览器中，Desktop 能力不可用",
      };
    }

    if (catalogQuery.isLoading || catalogQuery.isPending) {
      return { status: "checking" };
    }

    if (catalogQuery.isError) {
      return {
        status: "unavailable",
        reason: "error",
        message:
          catalogQuery.error instanceof Error
            ? catalogQuery.error.message
            : "failed to load desktop capabilities",
      };
    }

    const entry = catalogQuery.data?.find((item) => item.name === name);
    if (entry == null) {
      return {
        status: "unavailable",
        reason: "platform",
        message: `capability ${name} is not registered in this desktop build`,
      };
    }

    if (!entry.available) {
      const reason: UnavailableReason =
        mapUnavailableReason(entry.reason, false) ?? "error";
      return {
        status: "unavailable",
        reason,
        message: entry.message || `capability ${name} is unavailable`,
      };
    }

    return {
      status: "available",
      invoke,
    };
  }, [
    catalogQuery.data,
    catalogQuery.error,
    catalogQuery.isError,
    catalogQuery.isLoading,
    catalogQuery.isPending,
    desktop,
    invoke,
    name,
  ]);
}

/**
 * 显式失效能力清单缓存（供 D2 授权后刷新）。
 *
 * 参数:
 *   - 无（hook 内读取 queryClient）。
 *
 * 返回值:
 *   - () => void: 触发 invalidate 的函数。
 */
export function useInvalidateDesktopCapabilities(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: desktopKeys.all });
  }, [queryClient]);
}

/**
 * 权限类查询的建议 staleTime（短 TTL）。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - number: 毫秒。
 */
export function desktopPermissionStaleTime(): number {
  return PERMISSION_STALE_MS;
}
