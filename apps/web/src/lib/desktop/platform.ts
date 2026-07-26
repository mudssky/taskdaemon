/**
 * Desktop 环境判断的唯一来源。
 * 业务代码禁止自行探测原生桥；底层全局对象读取委托 bridge.ts。
 */

import { detectWailsRuntime } from "./bridge";

export type PlatformInfo = {
  runtime: "desktop" | "web";
  /** 是否检测到原生桥。 */
  hasNativeBridge: boolean;
};

/**
 * 判断当前是否运行在 Wails Desktop 壳内。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - boolean: true 表示 Desktop runtime。
 */
export function isDesktopRuntime(): boolean {
  return detectWailsRuntime();
}

/**
 * 返回精简平台信息；非 Desktop 时 runtime 为 web。
 *
 * 参数:
 *   - 无。
 *
 * 返回值:
 *   - PlatformInfo: runtime 与原生桥探测结果。
 */
export function getPlatformInfo(): PlatformInfo {
  const desktop = detectWailsRuntime();
  return {
    runtime: desktop ? "desktop" : "web",
    hasNativeBridge: desktop,
  };
}
