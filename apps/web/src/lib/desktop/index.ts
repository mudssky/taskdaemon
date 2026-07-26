/**
 * Desktop 能力边界层对外唯一出口（C-5 冻结产物）。
 */

export {
  type DesktopCapabilityDescriptor,
  type DesktopInvokeResult,
  invokeDesktopCapability,
  listDesktopCapabilities,
} from "./bridge";
export {
  type AutostartStatusInfo,
  type CapabilityState,
  DesktopCapability,
  type DesktopCapabilityName,
  type EnvironmentInfo,
  type InvokeOutcome,
  mapUnavailableReason,
  type TrayStatusInfo,
  type UnavailableReason,
  type WindowStateInfo,
} from "./capabilities";
export { DesktopEnvironmentPanel } from "./DesktopEnvironmentPanel";
export { DesktopNativeFeaturesPanel } from "./DesktopNativeFeaturesPanel";
export { DesktopNotificationPanel } from "./DesktopNotificationPanel";
export {
  getPlatformInfo,
  isDesktopRuntime,
  type PlatformInfo,
} from "./platform";
export {
  desktopPermissionStaleTime,
  useDesktopCapability,
  useInvalidateDesktopCapabilities,
} from "./useDesktopCapability";
