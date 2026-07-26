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
  type CapabilityState,
  DesktopCapability,
  type DesktopCapabilityName,
  type EnvironmentInfo,
  type InvokeOutcome,
  mapUnavailableReason,
  type UnavailableReason,
} from "./capabilities";
export { DesktopEnvironmentPanel } from "./DesktopEnvironmentPanel";
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
