/**
 * Agent API 单例：默认 mock，G2 就绪后切 VITE_AGENT_API_MODE=http。
 */

import type { AgentApi } from "./client";
import { createHttpAgentApi } from "./client";
import { createMockAgentApi } from "./mock-gateway";

let singleton: AgentApi | null = null;

/**
 * 获取全局 AgentApi。
 *
 * 参数: 无。
 *
 * 返回值:
 *   - mock 或 http 实现。
 */
export function getAgentApi(): AgentApi {
  if (singleton) {
    return singleton;
  }
  const mode = (import.meta.env.VITE_AGENT_API_MODE ?? "mock").toLowerCase();
  singleton =
    mode === "http"
      ? createHttpAgentApi()
      : createMockAgentApi({ delayMs: 18 });
  return singleton;
}

/**
 * 测试注入 API 实现。
 *
 * 参数:
 *   - api: 替换实现；传 null 重置。
 *
 * 返回值: 无。
 */
export function __setAgentApiForTests(api: AgentApi | null): void {
  singleton = api;
}
