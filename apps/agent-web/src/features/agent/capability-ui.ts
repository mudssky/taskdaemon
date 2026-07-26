/**
 * 能力驱动 UI 分支（纯函数）。
 * 前端不得按 runtimeId 硬编码能力；一律读 RuntimeCapabilities。
 */

import type {
  RuntimeCapabilities,
  Thread,
  ToolCallDetail,
} from "@taskdaemon/agent-protocol";

/** 控件可见/可用描述。 */
export type CapabilityControl = {
  /** 是否渲染该控件区域。 */
  visible: boolean;
  /** 可见时是否可交互。 */
  enabled: boolean;
  /** 禁用或不可用原因（展示给用户，不伪造能力）。 */
  reason?: string;
};

/**
 * steering 入口渲染策略。
 *
 * 参数:
 *   - caps: 会话能力声明。
 *
 * 返回值:
 *   - visible/enabled/reason。
 */
export function steeringControl(
  caps: RuntimeCapabilities | undefined,
): CapabilityControl {
  if (!caps) {
    return {
      visible: true,
      enabled: false,
      reason: "能力尚未加载",
    };
  }
  if (!caps.steering) {
    // R7：关闭时不显示入口；测试侧通过 visible=false 断言。
    return {
      visible: false,
      enabled: false,
      reason: "当前 runtime 未声明 steering",
    };
  }
  return { visible: true, enabled: true };
}

/**
 * 模型选择器渲染策略。
 *
 * 参数:
 *   - caps: 会话能力声明。
 *
 * 返回值:
 *   - visible/enabled/reason。
 */
export function modelSwitchControl(
  caps: RuntimeCapabilities | undefined,
): CapabilityControl {
  if (!caps) {
    return {
      visible: true,
      enabled: false,
      reason: "能力尚未加载",
    };
  }
  if (caps.modelSwitch === "none") {
    return {
      visible: false,
      enabled: false,
      reason: "当前 runtime 不支持模型切换",
    };
  }
  return {
    visible: true,
    enabled: true,
    reason:
      caps.modelSwitch === "request" ? "按请求切换模型" : "按会话切换模型",
  };
}

/**
 * 推理区是否渲染。
 *
 * 参数:
 *   - caps: 会话能力声明。
 *
 * 返回值:
 *   - true 时才展示 REASONING 内容。
 */
export function shouldShowThinking(
  caps: RuntimeCapabilities | undefined,
): boolean {
  return caps?.thinking === true;
}

/**
 * 工具调用展示粒度。
 *
 * 参数:
 *   - caps: 会话能力声明。
 *
 * 返回值:
 *   - full | name-only | none。
 */
export function toolCallDetailMode(
  caps: RuntimeCapabilities | undefined,
): ToolCallDetail {
  return caps?.toolCallDetail ?? "none";
}

/**
 * 工作区相关 UI 是否展示。
 * thread.workspaceBound 与 capabilities.workspaceBinding 任一为 false 则隐藏。
 *
 * 参数:
 *   - thread: 当前会话；可为 undefined。
 *   - caps: 能力声明。
 *
 * 返回值:
 *   - 是否展示工作区 UI。
 */
export function shouldShowWorkspaceUi(
  thread: Thread | undefined,
  caps: RuntimeCapabilities | undefined,
): boolean {
  if (thread && !thread.workspaceBound) {
    return false;
  }
  if (caps && !caps.workspaceBinding) {
    return false;
  }
  // 无 caps 时保守隐藏，避免伪造 coding workspace 体验。
  if (!caps) {
    return false;
  }
  return true;
}

/**
 * follow-up 入口。
 *
 * 参数:
 *   - caps: 会话能力声明。
 *
 * 返回值:
 *   - visible/enabled/reason。
 */
export function followUpControl(
  caps: RuntimeCapabilities | undefined,
): CapabilityControl {
  if (!caps) {
    return { visible: false, enabled: false, reason: "能力尚未加载" };
  }
  if (!caps.followUp) {
    return {
      visible: false,
      enabled: false,
      reason: "当前 runtime 未声明 follow-up",
    };
  }
  return { visible: true, enabled: true };
}

/**
 * 将 capabilities 摘要成调试/状态条文案（非硬编码 runtime 名能力）。
 *
 * 参数:
 *   - caps: 能力声明。
 *   - runtimeId: 中立 runtime 标识（仅展示，不用于分支）。
 *
 * 返回值:
 *   - 人类可读摘要。
 */
export function summarizeCapabilities(
  caps: RuntimeCapabilities,
  runtimeId: string,
): string {
  const profiles = Array.isArray(caps.profile)
    ? caps.profile.join("/")
    : caps.profile;
  return [
    `runtime=${runtimeId}`,
    `profile=${profiles}`,
    `tools=${caps.toolCallDetail}`,
    `thinking=${caps.thinking ? "on" : "off"}`,
    `model=${caps.modelSwitch}`,
    `steer=${caps.steering ? "on" : "off"}`,
    `workspace=${caps.workspaceBinding ? "on" : "off"}`,
  ].join(" · ");
}
