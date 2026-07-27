/**
 * Wails 调用封装 —— 全仓库唯一允许读取 Wails 全局对象的文件。
 */

/** Wails v3 Call.ByName 使用的完全限定方法名。 */
export const WAILS_LIST_CAPABILITIES =
  "taskdaemon/internal/desktop.App.ListCapabilities";
export const WAILS_INVOKE_CAPABILITY =
  "taskdaemon/internal/desktop.App.InvokeCapability";

type WailsCall = {
  ByName?: (name: string, ...args: unknown[]) => Promise<unknown>;
};

type DesktopWindow = Window & {
  _wails?: unknown;
  wails?: {
    Call?: WailsCall;
    invoke?: unknown;
  };
  chrome?: {
    webview?: {
      postMessage?: unknown;
    };
  };
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: unknown;
      };
    };
  };
};

/**
 * 探测是否存在 Wails 原生桥或 runtime 标记。
 * 仅本文件可读全局对象；platform.ts 通过此函数判断环境。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - boolean: 检测到 Desktop runtime 时为 true。
 */
export function detectWailsRuntime(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const w = window as DesktopWindow;
  if (
    w._wails != null ||
    w.wails?.invoke != null ||
    w.wails?.Call?.ByName != null
  ) {
    return true;
  }
  if (w.chrome?.webview?.postMessage != null) {
    return true;
  }
  if (w.webkit?.messageHandlers?.external?.postMessage != null) {
    return true;
  }
  return false;
}

/**
 * 解析 Wails Call.ByName 入口。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - WailsCall | null: 可用时返回 Call 对象。
 */
function getWailsCall(): WailsCall | null {
  if (typeof window === "undefined") {
    return null;
  }
  const call = (window as DesktopWindow).wails?.Call;
  if (call?.ByName == null) {
    return null;
  }
  return call;
}

/**
 * 通过 Wails Call.ByName 调用 Go binding。
 *
 * 参数:
 *   - methodName: 完全限定方法名。
 *   - args: 方法参数。
 *
 * 返回值:
 *   - Promise<T>: Go 返回值。
 *   - 拒绝: 非 Desktop、无 Call 入口或调用失败。
 */
export async function callWailsByName<T>(
  methodName: string,
  ...args: unknown[]
): Promise<T> {
  if (!detectWailsRuntime()) {
    throw new Error("not running in desktop runtime");
  }
  const call = getWailsCall();
  if (call?.ByName == null) {
    throw new Error("wails Call.ByName is unavailable");
  }
  return (await call.ByName(methodName, ...args)) as T;
}

/**
 * 拉取 Go 侧已注册能力清单。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - Promise 能力描述数组。
 */
export async function listDesktopCapabilities(): Promise<
  DesktopCapabilityDescriptor[]
> {
  const raw = await callWailsByName<DesktopCapabilityDescriptor[]>(
    WAILS_LIST_CAPABILITIES,
  );
  return Array.isArray(raw) ? raw : [];
}

/**
 * 调用具名 Desktop capability。
 *
 * 参数:
 *   - name: capability 常量值。
 *   - payload: 可选 JSON 可序列化负载。
 *
 * 返回值:
 *   - Promise<DesktopInvokeResult<T>>: 结构化结果，不抛业务异常。
 */
export async function invokeDesktopCapability<T = unknown>(
  name: string,
  payload?: unknown,
): Promise<DesktopInvokeResult<T>> {
  try {
    const payloadJSON =
      payload === undefined ? "" : JSON.stringify(payload ?? null);
    const raw = await callWailsByName<DesktopInvokeResultWire>(
      WAILS_INVOKE_CAPABILITY,
      name,
      payloadJSON,
    );
    if (raw == null || typeof raw !== "object") {
      return {
        ok: false,
        code: "DESKTOP_INVOKE_FAILED",
        message: "empty invoke result from desktop bridge",
      };
    }
    if (raw.ok) {
      let data: T | undefined;
      if (raw.data == null) {
        data = undefined;
      } else if (typeof raw.data === "string") {
        try {
          data = JSON.parse(raw.data) as T;
        } catch {
          data = raw.data as T;
        }
      } else {
        data = raw.data as T;
      }
      return { ok: true, data };
    }
    return {
      ok: false,
      code: raw.code ?? "DESKTOP_INVOKE_FAILED",
      message: raw.message ?? "desktop capability invoke failed",
    };
  } catch (error) {
    return {
      ok: false,
      code: "DESKTOP_INVOKE_FAILED",
      message: error instanceof Error ? error.message : String(error),
    };
  }
}

/** Go CapabilityDescriptor 的前端镜像。 */
export type DesktopCapabilityDescriptor = {
  name: string;
  available: boolean;
  reason?: string;
  message?: string;
};

/** Go InvokeResult 线格式。 */
type DesktopInvokeResultWire = {
  ok: boolean;
  data?: unknown;
  code?: string;
  message?: string;
};

/** 前端消费的 invoke 结果。 */
export type DesktopInvokeResult<T = unknown> =
  | { ok: true; data?: T }
  | { ok: false; code: string; message: string };
