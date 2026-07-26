/**
 * Environment 样板能力的最小消费点：展示真实信息或浏览器禁用态。
 */

import { useEffect, useState } from "react";
import { DesktopCapability, type EnvironmentInfo } from "./capabilities";
import { useDesktopCapability } from "./useDesktopCapability";

/**
 * 在设置/总览等页面 append 的 Desktop 环境面板。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - JSX.Element: 可用时显示平台信息；浏览器显示禁用说明。
 */
export function DesktopEnvironmentPanel() {
  const capability = useDesktopCapability<EnvironmentInfo>(
    DesktopCapability.Environment,
  );
  const [info, setInfo] = useState<EnvironmentInfo | null>(null);
  const [invokeError, setInvokeError] = useState<string | null>(null);

  useEffect(() => {
    if (capability.status !== "available") {
      setInfo(null);
      setInvokeError(null);
      return;
    }

    let cancelled = false;
    void capability.invoke().then((result) => {
      if (cancelled) {
        return;
      }
      if (result.ok) {
        setInfo(result.data ?? null);
        setInvokeError(null);
        return;
      }
      setInfo(null);
      setInvokeError(`${result.code}: ${result.message}`);
    });

    return () => {
      cancelled = true;
    };
  }, [capability]);

  return (
    <section
      className="panel"
      aria-labelledby="desktop-environment-title"
      data-testid="desktop-environment-panel"
    >
      <div className="panel-header">
        <div>
          <p className="eyebrow">Desktop</p>
          <h2 id="desktop-environment-title">平台能力</h2>
        </div>
      </div>

      {capability.status === "checking" ? (
        <p className="muted">正在检测 Desktop 能力...</p>
      ) : null}

      {capability.status === "unavailable" ? (
        <div className="empty-state" data-testid="desktop-environment-disabled">
          <p>
            <strong>Desktop 能力不可用</strong>
          </p>
          <p className="muted">
            原因：{reasonLabel(capability.reason)} — {capability.message}
          </p>
          <p className="muted">
            在 Wails 桌面壳内运行时可查看平台信息与已注册能力清单。
          </p>
        </div>
      ) : null}

      {capability.status === "available" && invokeError != null ? (
        <p className="muted" data-testid="desktop-environment-error">
          调用失败：{invokeError}
        </p>
      ) : null}

      {capability.status === "available" && info != null ? (
        <dl className="meta-list" data-testid="desktop-environment-info">
          <div>
            <dt>平台</dt>
            <dd>
              {info.platform} / {info.arch}
            </dd>
          </div>
          <div>
            <dt>OS</dt>
            <dd>{info.osVersion}</dd>
          </div>
          <div>
            <dt>应用版本</dt>
            <dd>{info.appVersion}</dd>
          </div>
          <div>
            <dt>已注册能力</dt>
            <dd>{info.capabilities.join(", ") || "（无）"}</dd>
          </div>
        </dl>
      ) : null}

      {capability.status === "available" &&
      info == null &&
      invokeError == null ? (
        <p className="muted">正在读取环境信息...</p>
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
