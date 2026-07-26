/**
 * RuntimeCapabilities — 能力协商模型（C-3）。
 *
 * 与 Track D C-5 Desktop capability 同构：
 * 「声明 + 前端按声明降级，不硬编码实现方能力」。
 *
 * 定稿依据：G0 runtime-abstraction-feasibility.md + 两份能力矩阵。
 */

/** Agent 能力画像（影响 UI 与工具面，与 attachMode 正交）。 */
export type AgentProfile = "coding" | "general";

/** 接入方式（影响延迟与隔离，与 profile 正交）。 */
export type AttachMode = "cli-spawn" | "sdk-inprocess";

/** 冷启动成本，供 G2 进程池策略使用。 */
export type ColdStartCost = "low" | "high";

/** 工具调用对外可见粒度。 */
export type ToolCallDetail = "full" | "name-only" | "none";

/** 模型切换粒度。 */
export type ModelSwitchMode = "session" | "request" | "none";

/** 会话持久化权威位置。 */
export type SessionPersistenceMode = "runtime" | "gateway" | "none";

/** 历史可读范围。 */
export type HistoryCapability = "messages" | "messages+entries" | "none";

/** 内存量级提示（编排并发，非精确 RSS）。 */
export type MemoryClass = "light" | "heavy";

/**
 * Adapter 能力声明。
 *
 * 不变量（不进本结构）：
 * - 所有 adapter 出口必须 sanitize 密钥；这是硬约束，不是能力位。
 */
export type RuntimeCapabilities = {
  /**
   * 支持的 profile。
   * 单值表示仅一种；数组表示可切换。
   */
  profile: AgentProfile | AgentProfile[];
  /** 当前接入方式。同一 runtime 可有多实现，各自报告自己的 attachMode。 */
  attachMode: AttachMode;
  /** CLI 冷启动均为 high（G0 实测）；sdk-inprocess 可为 low。 */
  coldStartCost: ColdStartCost;
  /** 是否支持运行中 steer。 */
  steering: boolean;
  /** 是否支持 follow-up（与 steer 语义不同，见 AgentRuntime）。 */
  followUp: boolean;
  /** 是否暴露 thinking / reasoning 内容。 */
  thinking: boolean;
  /** 工具调用细节粒度。 */
  toolCallDetail: ToolCallDetail;
  /** 模型切换模式。 */
  modelSwitch: ModelSwitchMode;
  /**
   * 是否默认绑定 workspace。
   * general 会话可通过 SessionOpts 关闭；关闭后 coding 特化事件不发送。
   */
  workspaceBinding: boolean;
  /** 会话持久化权威。G0：两 runtime 均为 runtime 文件。 */
  sessionPersistence: SessionPersistenceMode;
  /** abort 后会话是否可继续 prompt。G0：两 runtime 均为 true。 */
  abortContinuesSession: boolean;
  /** 历史 API 可靠范围。 */
  history: HistoryCapability;
  /** 内存量级，影响 maxConcurrency。 */
  memoryClass: MemoryClass;
};

/** Pi CLI adapter 参考实例（G0 实测，attachMode=cli-spawn）。 */
export const PI_CLI_CAPABILITIES: RuntimeCapabilities = {
  profile: ["coding", "general"],
  attachMode: "cli-spawn",
  coldStartCost: "high",
  steering: true,
  followUp: true,
  thinking: true,
  toolCallDetail: "full",
  modelSwitch: "session",
  workspaceBinding: true,
  sessionPersistence: "runtime",
  abortContinuesSession: true,
  history: "messages+entries",
  memoryClass: "light",
};

/**
 * Pi SDK-inprocess 参考实例（G0 确认 createAgentSession 可用）。
 * coldStartCost 降为 low；其余与 CLI 对齐。
 */
export const PI_SDK_CAPABILITIES: RuntimeCapabilities = {
  ...PI_CLI_CAPABILITIES,
  attachMode: "sdk-inprocess",
  coldStartCost: "low",
};

/**
 * OMP CLI adapter 参考实例（G0 实测）。
 * general 需额外裁剪，非开箱即干净；history 仅保证 messages。
 */
export const OMP_CLI_CAPABILITIES: RuntimeCapabilities = {
  profile: ["coding", "general"],
  attachMode: "cli-spawn",
  coldStartCost: "high",
  steering: true,
  followUp: true,
  thinking: true,
  toolCallDetail: "full",
  modelSwitch: "session",
  workspaceBinding: true,
  sessionPersistence: "runtime",
  abortContinuesSession: true,
  history: "messages",
  memoryClass: "heavy",
};

/**
 * 判断 capabilities 是否声明某 profile。
 *
 * 参数:
 *   - caps: 能力声明。
 *   - profile: 目标 profile。
 *
 * 返回值:
 *   - 是否支持。
 */
export function supportsProfile(
  caps: RuntimeCapabilities,
  profile: AgentProfile,
): boolean {
  return Array.isArray(caps.profile)
    ? caps.profile.includes(profile)
    : caps.profile === profile;
}
