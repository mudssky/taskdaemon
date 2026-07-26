/**
 * TD2 Desktop 原生增量能力设置面板：托盘 / 自启动 / 窗口状态。
 * 经 useDesktopCapability 消费，不直接读 Wails。
 */

import { useCallback, useEffect, useState } from "react";
import {
  type AutostartStatusInfo,
  DesktopCapability,
  type TrayStatusInfo,
  type WindowStateInfo,
} from "./capabilities";
import { useDesktopCapability } from "./useDesktopCapability";

/**
 * 设置页 append 的 Desktop 原生能力面板。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - JSX.Element: 浏览器禁用态或 Desktop 开关/状态。
 */
export function DesktopNativeFeaturesPanel() {
  const tray = useDesktopCapability<TrayStatusInfo | { ok: boolean }>(
    DesktopCapability.Tray,
  );
  const autostart = useDesktopCapability<AutostartStatusInfo>(
    DesktopCapability.Autostart,
  );
  const windowState = useDesktopCapability<WindowStateInfo>(
    DesktopCapability.WindowState,
  );

  const [trayInfo, setTrayInfo] = useState<TrayStatusInfo | null>(null);
  const [autostartInfo, setAutostartInfo] =
    useState<AutostartStatusInfo | null>(null);
  const [windowInfo, setWindowInfo] = useState<WindowStateInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    setError(null);
    if (tray.status === "available") {
      const result = await tray.invoke({ action: "status" });
      if (result.ok && result.data && "serviceStatus" in result.data) {
        setTrayInfo(result.data as TrayStatusInfo);
      } else if (!result.ok) {
        setError(result.message);
      }
    }
    if (autostart.status === "available") {
      const result = await autostart.invoke({ action: "status" });
      if (result.ok && result.data) {
        setAutostartInfo(result.data);
      } else if (!result.ok) {
        setError(result.message);
      }
    }
    if (windowState.status === "available") {
      const result = await windowState.invoke({ action: "get" });
      if (result.ok && result.data) {
        setWindowInfo(result.data);
      } else if (!result.ok) {
        setError(result.message);
      }
    }
  }, [tray, autostart, windowState]);

  useEffect(() => {
    if (
      tray.status === "checking" ||
      autostart.status === "checking" ||
      windowState.status === "checking"
    ) {
      return;
    }
    void refresh();
  }, [tray.status, autostart.status, windowState.status, refresh]);

  const onToggleAutostart = async (enabled: boolean) => {
    if (autostart.status !== "available") {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await autostart.invoke({
        action: enabled ? "enable" : "disable",
      });
      if (!result.ok) {
        setError(result.message);
        return;
      }
      if (result.data) {
        setAutostartInfo(result.data);
      }
    } finally {
      setBusy(false);
    }
  };

  const onToggleMinimizeToTray = async (enabled: boolean) => {
    if (tray.status !== "available") {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await tray.invoke({
        action: "setMinimizeToTray",
        enabled,
      });
      if (!result.ok) {
        setError(result.message);
        return;
      }
      await refresh();
    } finally {
      setBusy(false);
    }
  };

  const onResetWindow = async () => {
    if (windowState.status !== "available") {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await windowState.invoke({ action: "reset" });
      if (!result.ok) {
        setError(result.message);
        return;
      }
      if (result.data) {
        setWindowInfo(result.data);
      }
    } finally {
      setBusy(false);
    }
  };

  const allUnavailable =
    tray.status === "unavailable" &&
    autostart.status === "unavailable" &&
    windowState.status === "unavailable";

  return (
    <section
      className="panel"
      aria-labelledby="desktop-native-features-title"
      data-testid="desktop-native-features-panel"
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Desktop</p>
          <h2 id="desktop-native-features-title">原生增量能力</h2>
          <p className="muted">
            托盘、桌面应用登录自启、窗口记忆。与「系统服务安装（T5）」不是同一回事。
          </p>
        </div>
      </div>

      {allUnavailable ? (
        <div
          className="empty-state"
          data-testid="desktop-native-features-disabled"
        >
          <p>
            <strong>仅 Desktop 环境可用</strong>
          </p>
          <p className="muted">
            当前为浏览器：开关显示禁用。请在 Wails 桌面壳内配置托盘与登录项。
          </p>
          <p className="muted">
            说明：Desktop 自启动 = 登录时打开图形应用；T5 = 无界面系统服务常驻。
          </p>
        </div>
      ) : null}

      {!allUnavailable ? (
        <dl
          className="settings-list"
          data-testid="desktop-native-features-body"
        >
          <div className="settings-list-item">
            <dt>托盘服务状态</dt>
            <dd>
              {tray.status === "available"
                ? (trayInfo?.label ?? "读取中…")
                : unavailableText(tray)}
            </dd>
          </div>
          <div className="settings-list-item">
            <dt>关闭窗口最小化到托盘</dt>
            <dd>
              {tray.status === "available" ? (
                <label>
                  <input
                    type="checkbox"
                    checked={trayInfo?.minimizeToTray ?? false}
                    disabled={busy}
                    onChange={(event) => {
                      void onToggleMinimizeToTray(event.target.checked);
                    }}
                  />{" "}
                  启用
                </label>
              ) : (
                unavailableText(tray)
              )}
            </dd>
          </div>
          <div className="settings-list-item">
            <dt>桌面应用登录时启动</dt>
            <dd>
              {autostart.status === "available" ? (
                <>
                  <label>
                    <input
                      type="checkbox"
                      checked={autostartInfo?.enabled ?? false}
                      disabled={busy}
                      onChange={(event) => {
                        void onToggleAutostart(event.target.checked);
                      }}
                    />{" "}
                    启用（非系统服务）
                  </label>
                  {autostartInfo?.note ? (
                    <p className="muted" style={{ marginTop: 6 }}>
                      {autostartInfo.note}
                    </p>
                  ) : null}
                </>
              ) : (
                unavailableText(autostart)
              )}
            </dd>
          </div>
          <div className="settings-list-item">
            <dt>窗口状态</dt>
            <dd>
              {windowState.status === "available" ? (
                <>
                  <span>
                    {windowInfo
                      ? `${windowInfo.width}×${windowInfo.height} @ (${windowInfo.x},${windowInfo.y})${windowInfo.maximised ? " 最大化" : ""}`
                      : "读取中…"}
                  </span>
                  <div style={{ marginTop: 8 }}>
                    <button
                      type="button"
                      className="btn secondary"
                      disabled={busy}
                      onClick={() => {
                        void onResetWindow();
                      }}
                    >
                      重置为默认布局
                    </button>
                  </div>
                </>
              ) : (
                unavailableText(windowState)
              )}
            </dd>
          </div>
        </dl>
      ) : null}

      {error != null ? (
        <p className="muted" data-testid="desktop-native-features-error">
          操作失败：{error}
        </p>
      ) : null}
    </section>
  );
}

/**
 * 将 capability unavailable 状态格式化为文案。
 *
 * 参数:
 *   - state: hook 返回状态。
 *
 * 返回值:
 *   - string: 展示文案。
 */
function unavailableText(state: {
  status: string;
  reason?: string;
  message?: string;
}): string {
  if (state.status === "checking") {
    return "检测中…";
  }
  if (state.status === "unavailable") {
    return `不可用（${state.reason ?? "unknown"}）：${state.message ?? ""}`;
  }
  return "—";
}
