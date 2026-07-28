/**
 * Wails / Desktop 调用封装 —— 全仓库唯一允许读取原生全局对象的文件。
 *
 * 业务 API（含登录）必须走同域 /api，禁止经 wails.localhost AssetServer 转发 POST
 * （WebView2 会丢 body，表现为登录 400 / 0ms）。
 *
 * Desktop 能力优先 Wails Call.ByName；不可用时回退同进程 HTTP：
 *   GET  /api/desktop/capabilities
 *   POST /api/desktop/invoke
 */

/** Wails v3 Call.ByName 使用的完全限定方法名（保留兼容）。 */
export const WAILS_LIST_CAPABILITIES =
  "taskdaemon/internal/desktop.App.ListCapabilities";
export const WAILS_INVOKE_CAPABILITY =
  "taskdaemon/internal/desktop.App.InvokeCapability";

/** HTTP 能力清单路径。 */
export const DESKTOP_HTTP_CAPABILITIES = "/api/desktop/capabilities";
/** HTTP 能力调用路径。 */
export const DESKTOP_HTTP_INVOKE = "/api/desktop/invoke";

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
 * 解析已注入的 window.wails.Call。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - WailsCall | null。
 */
function getInjectedWailsCall(): WailsCall | null {
  if (typeof window === "undefined") {
    return null;
  }
  const call = (window as DesktopWindow).wails?.Call;
  if (call?.ByName == null) {
    return null;
  }
  return call;
}

type ApiEnvelope<T> = {
  data?: T;
};

/**
 * 解析标准 API 成功信封。
 *
 * 参数:
 *   - response: fetch Response。
 *
 * 返回值:
 *   - Promise<T>: data 字段。
 */
async function readAPIData<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = (await response.json()) as {
        error?: { message?: string };
        message?: string;
      };
      message = body.error?.message ?? body.message ?? message;
    } catch {
      // ignore
    }
    throw new Error(message || `desktop http failed (${response.status})`);
  }
  const body = (await response.json()) as ApiEnvelope<T>;
  return body.data as T;
}

/**
 * 通过同域 HTTP 拉取能力清单。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - Promise 能力描述数组。
 */
async function listDesktopCapabilitiesHTTP(): Promise<
  DesktopCapabilityDescriptor[]
> {
  const response = await fetch(DESKTOP_HTTP_CAPABILITIES, {
    method: "GET",
    credentials: "include",
  });
  const raw = await readAPIData<DesktopCapabilityDescriptor[]>(response);
  return Array.isArray(raw) ? raw : [];
}

/**
 * 通过同域 HTTP 调用 capability。
 *
 * 参数:
 *   - name: capability 名。
 *   - payloadJSON: JSON 字符串负载。
 *
 * 返回值:
 *   - Promise 线格式结果。
 */
async function invokeDesktopCapabilityHTTP(
  name: string,
  payloadJSON: string,
): Promise<DesktopInvokeResultWire> {
  const response = await fetch(DESKTOP_HTTP_INVOKE, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, payloadJSON }),
  });
  return readAPIData<DesktopInvokeResultWire>(response);
}

/**
 * 通过 Wails Call.ByName 或 HTTP 桥调用 Go binding / capability 入口。
 *
 * 参数:
 *   - methodName: 完全限定方法名（仅 Wails 路径使用）。
 *   - args: 方法参数。
 *
 * 返回值:
 *   - Promise<T>: 返回值。
 */
export async function callWailsByName<T>(
  methodName: string,
  ...args: unknown[]
): Promise<T> {
  if (!detectWailsRuntime()) {
    throw new Error("not running in desktop runtime");
  }

  const injected = getInjectedWailsCall();
  if (injected?.ByName != null) {
    return (await injected.ByName(methodName, ...args)) as T;
  }

  if (methodName === WAILS_LIST_CAPABILITIES) {
    return (await listDesktopCapabilitiesHTTP()) as T;
  }
  if (methodName === WAILS_INVOKE_CAPABILITY) {
    const name = typeof args[0] === "string" ? args[0] : "";
    const payloadJSON = typeof args[1] === "string" ? args[1] : "";
    return (await invokeDesktopCapabilityHTTP(name, payloadJSON)) as T;
  }

  throw new Error(
    `wails Call.ByName is unavailable and no HTTP fallback for ${methodName}`,
  );
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
