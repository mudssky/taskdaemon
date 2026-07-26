/**
 * Desktop capability 具名常量与统一状态类型（C-5）。
 */

export const DesktopCapability = {
  Environment: "desktop.environment",
  /** D2 占位，本任务不实现。 */
  Notification: "desktop.notification",
  /** D3 占位，本任务不实现。 */
  Tray: "desktop.tray",
  /** D3 占位，本任务不实现。 */
  Autostart: "desktop.autostart",
  /** D3 占位，本任务不实现。 */
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
