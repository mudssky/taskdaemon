/**
 * AgentRuntime 接口定稿（C-3）。
 *
 * 定稿依据：G0 `runtime-abstraction-feasibility.md`。
 * 采纳建议：followUp / getMessages / setModel / disposeSession + secret sanitize 不变量。
 * 否决：把 OMP task/hub/subagent、Pi WebUI 文件浏览器纳入 MVP 接口。
 *
 * 中立性：无 Pi/OMP 专有概念；差异走 RuntimeCapabilities。
 */

import type { RuntimeCapabilities } from "./capabilities.js";
import type {
  HistoryPage,
  MessagePage,
  ModelInfo,
  ModelRef,
  PromptInput,
  RuntimeEvent,
  RuntimeState,
  SessionHandle,
  SessionOpts,
} from "./runtime-types.js";

/**
 * 多 adapter 统一运行时接口。
 *
 * 必选方法：所有 adapter 必须实现。
 * 可选方法（?）：缺失时 capabilities 对应字段必须反映不可用，前端/gateway 按声明降级。
 *
 * 不变量：
 * 1. getState / listModels / 事件流出口不得包含密钥或 provider Authorization。
 * 2. 换掉任一 adapter 不得迫使前端协议变更（仅 capabilities 不同）。
 * 3. runtime 已提供的 loop/LLM/tool 执行一律透传，本接口不做二次封装。
 */
export interface AgentRuntime {
  /** 稳定 runtime id，如 `pi`、`omp`。 */
  readonly id: string;

  /**
   * 返回当前实现的能力声明。
   *
   * 参数: 无。
   * 返回值: RuntimeCapabilities。
   */
  capabilities(): RuntimeCapabilities;

  /**
   * 创建会话（CLI 通常 spawn 进程；SDK 创建内存会话）。
   *
   * 参数:
   *   - opts: 会话选项（profile / cwd / tools 等）。
   * 返回值:
   *   - SessionHandle。
   */
  createSession(opts: SessionOpts): Promise<SessionHandle>;

  /**
   * 发起一轮 prompt，流式产出归一化 RuntimeEvent。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - input: 用户输入。
   * 返回值:
   *   - AsyncIterable<RuntimeEvent>。
   */
  prompt(sessionId: string, input: PromptInput): AsyncIterable<RuntimeEvent>;

  /**
   * 运行中转向（可选）。capabilities.steering=false 时不得调用。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - input: 转向输入。
   * 返回值:
   *   - Promise<void>。
   */
  steer?(sessionId: string, input: PromptInput): Promise<void>;

  /**
   * 追加 follow-up（与 steer 不同：通常入队下一轮，非打断当前生成）。
   * capabilities.followUp=false 时不得调用。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - input: follow-up 输入。
   * 返回值:
   *   - Promise<void>。
   */
  followUp?(sessionId: string, input: PromptInput): Promise<void>;

  /**
   * 中止当前生成。G0：两 runtime 真停后会话可继续。
   *
   * 参数:
   *   - sessionId: 会话 id。
   * 返回值:
   *   - Promise<void>。
   */
  abort(sessionId: string): Promise<void>;

  /**
   * 读取会话状态（必须经 secret sanitize）。
   *
   * 参数:
   *   - sessionId: 会话 id。
   * 返回值:
   *   - RuntimeState。
   */
  getState(sessionId: string): Promise<RuntimeState>;

  /**
   * 读取消息历史。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - page: 可选分页。
   * 返回值:
   *   - MessagePage。
   */
  getMessages(sessionId: string, page?: HistoryPage): Promise<MessagePage>;

  /**
   * 会话级切换模型（可选）。capabilities.modelSwitch==='none' 时不得调用。
   *
   * 参数:
   *   - sessionId: 会话 id。
   *   - model: 目标模型。
   * 返回值:
   *   - Promise<void>。
   */
  setModel?(sessionId: string, model: ModelRef): Promise<void>;

  /**
   * 列出可用模型（可选）。
   *
   * 参数:
   *   - sessionId: 会话 id。
   * 返回值:
   *   - ModelInfo[]（已消毒）。
   */
  listModels?(sessionId: string): Promise<ModelInfo[]>;

  /**
   * 释放会话（杀 CLI 进程 / 释放 SDK 资源）。
   *
   * 参数:
   *   - sessionId: 会话 id。
   * 返回值:
   *   - Promise<void>。
   */
  disposeSession(sessionId: string): Promise<void>;
}

/**
 * 类型守卫：运行时是否声明某可选能力。
 *
 * 参数:
 *   - runtime: AgentRuntime。
 *   - method: 可选方法名。
 *
 * 返回值:
 *   - 是否实现该方法。
 */
export function hasOptionalRuntimeMethod(
  runtime: AgentRuntime,
  method: "steer" | "followUp" | "setModel" | "listModels",
): boolean {
  return typeof runtime[method] === "function";
}
