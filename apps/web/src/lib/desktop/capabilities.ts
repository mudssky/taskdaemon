/**
 * Desktop capability 具名常量与统一状态类型（C-5）。
 */

export const DesktopCapability = {
  Environment: "desktop.environment",
  /** D2 占位：原生通知 sink（本任务不实现）。 */
  Notification: "desktop.notification",
  /** TD2/D3：系统托盘。 */
  Tray: "desktop.tray",
  /** TD2/D3：Desktop 应用登录项自启动（非 T5 系统服务）。 */
  Autostart: "desktop.autostart",
  /** TD2/D3：窗口几何持久化。 */
  WindowState: "desktop.window-state",
} as const;

export type DesktopCapabilityName =
  (typeof DesktopCapability)[keyof typeof DesktopCapability];

export type UnavailableReason =
  | "not-desktop"
  | "platform"
  | "permission"
  | "error";

/**
 * capability 统一返回形状。
 * unavailable 分支没有 invoke —— 类型层强制 UI 分支处理。
 */
export type CapabilityState<TResult = void> =
  | {
      status: "available";
      invoke: (input?: unknown) => Promise<InvokeOutcome<TResult>>;
    }
  | {
      status: "unavailable";
      reason: UnavailableReason;
      message: string;
    }
  | { status: "checking" };

/** invoke 的结构化结果，永不抛异常。 */
export type InvokeOutcome<TResult = void> =
  | { ok: true; data?: TResult }
  | { ok: false; code: string; message: string };

/** Environment 样板能力返回形状。 */
export type EnvironmentInfo = {
  platform: string;
  arch: string;
  osVersion: string;
  appVersion: string;
  capabilities: string[];
};

/** Tray status action 返回形状。 */
export type TrayStatusInfo = {
  serviceStatus: "running" | "stopped" | "error" | string;
  minimizeToTray: boolean;
  label: string;
};

/** Autostart status 返回形状。 */
export type AutostartStatusInfo = {
  enabled: boolean;
  strategy?: string;
  path?: string;
  note: string;
};

/** Window-state get 返回形状。 */
export type WindowStateInfo = {
  x: number;
  y: number;
  width: number;
  height: number;
  maximised: boolean;
  path: string;
  enabled: boolean;
};

/**
 * 将 Go 侧 reason 字符串收敛为 UnavailableReason。
 *
 * 参数:
 *   - reason: Go 返回的 reason。
 *   - available: 是否可用。
 *
 * 返回值:
 *   - UnavailableReason | null: 可用时 null。
 */
export function mapUnavailableReason(
  reason: string | undefined,
  available: boolean,
): UnavailableReason | null {
  if (available) {
    return null;
  }
  switch (reason) {
    case "platform":
      return "platform";
    case "permission":
      return "permission";
    case "error":
      return "error";
    case "not-desktop":
      return "not-desktop";
    default:
      return "error";
  }
}
