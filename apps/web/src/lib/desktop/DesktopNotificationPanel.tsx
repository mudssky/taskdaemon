/**
 * Desktop 原生通知设置面板：权限状态、请求授权、发送测试通知。
 */

import { useCallback, useEffect, useState } from "react";
import { DesktopCapability } from "./capabilities";
import {
  useDesktopCapability,
  useInvalidateDesktopCapabilities,
} from "./useDesktopCapability";

/** status 动作返回形状。 */
export type NotificationStatus = {
  authorized: boolean;
  canRequest: boolean;
  deniedOnce: boolean;
};

/**
 * 设置页 Desktop 通知面板。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - JSX.Element: 可用时提供测试入口；浏览器显示禁用说明。
 */
export function DesktopNotificationPanel() {
  const capability = useDesktopCapability<
    NotificationStatus | { ok: boolean; authorized?: boolean }
  >(DesktopCapability.Notification);
  const invalidate = useInvalidateDesktopCapabilities();
  const [status, setStatus] = useState<NotificationStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refreshStatus = useCallback(async () => {
    if (capability.status !== "available") {
      setStatus(null);
      return;
    }
    const result = await capability.invoke({ action: "status" });
    if (!result.ok) {
      setError(`${result.code}: ${result.message}`);
      return;
    }
    const data = result.data as NotificationStatus | undefined;
    if (data && typeof data.authorized === "boolean") {
      setStatus(data);
      setError(null);
    }
  }, [capability]);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  const runAction = useCallback(
    async (input: unknown, onOk: () => void): Promise<void> => {
      if (capability.status !== "available") {
        return;
      }
      setBusy(true);
      setMessage(null);
      setError(null);
      try {
        const result = await capability.invoke(input);
        if (!result.ok) {
          setError(`${result.code}: ${result.message}`);
          return;
        }
        onOk();
      } finally {
        setBusy(false);
      }
    },
    [capability],
  );

  const handleRequestPermission = useCallback(() => {
    void runAction({ action: "requestPermission" }, () => {
      setMessage("已请求系统通知权限");
      invalidate();
      void refreshStatus();
    });
  }, [invalidate, refreshStatus, runAction]);

  const handleTestNotification = useCallback(() => {
    void runAction(
      {
        action: "show",
        title: "taskdaemon 测试通知",
        body: "若你看到这条系统通知，说明 Desktop 通知 sink 工作正常。",
        id: `test-${Date.now()}`,
      },
      () => {
        setMessage("已发送测试通知");
        void refreshStatus();
      },
    );
  }, [refreshStatus, runAction]);

  return (
    <section
      className="panel"
      aria-labelledby="desktop-notification-title"
      data-testid="desktop-notification-panel"
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Desktop</p>
          <h2 id="desktop-notification-title">系统通知</h2>
        </div>
      </div>

      {capability.status === "checking" ? (
        <p className="muted">正在检测通知能力...</p>
      ) : null}

      {capability.status === "unavailable" ? (
        <div
          className="empty-state"
          data-testid="desktop-notification-disabled"
        >
          <p>
            <strong>系统通知不可用</strong>
          </p>
          <p className="muted">
            原因：{reasonLabel(capability.reason)} — {capability.message}
          </p>
          <p className="muted">
            请在 Wails 桌面壳内使用；纯浏览器无法弹出系统原生通知。
          </p>
        </div>
      ) : null}

      {capability.status === "available" ? (
        <div data-testid="desktop-notification-controls">
          <dl className="meta-list">
            <div>
              <dt>授权状态</dt>
              <dd data-testid="desktop-notification-auth">
                {status == null
                  ? "读取中..."
                  : status.authorized
                    ? "已授权"
                    : status.deniedOnce
                      ? "已拒绝（可在系统设置中重新开启）"
                      : "未授权"}
              </dd>
            </div>
          </dl>

          <div className="topbar-actions" style={{ marginTop: 12, gap: 8 }}>
            <button
              type="button"
              className="secondary-button"
              disabled={busy || status?.authorized === true}
              onClick={handleRequestPermission}
              data-testid="desktop-notification-request"
            >
              请求通知权限
            </button>
            <button
              type="button"
              className="primary-button"
              disabled={busy}
              onClick={handleTestNotification}
              data-testid="desktop-notification-test"
            >
              发送测试通知
            </button>
          </div>

          {message != null ? (
            <p className="muted" data-testid="desktop-notification-message">
              {message}
            </p>
          ) : null}
          {error != null ? (
            <p className="muted" data-testid="desktop-notification-error">
              {error}
            </p>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

/**
 * 将 UnavailableReason 映射为中文标签。
 *
 * 参数:
 *   - reason: 不可用原因。
 *
 * 返回值:
 *   - string: UI 文案。
 */
function reasonLabel(reason: string): string {
  switch (reason) {
    case "not-desktop":
      return "非 Desktop 环境";
    case "platform":
      return "当前系统不支持";
    case "permission":
      return "权限未授予";
    case "error":
      return "检测失败";
    default:
      return reason;
  }
}
