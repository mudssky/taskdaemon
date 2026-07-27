/**
 * Runtime 注册表：runtimeId → AgentRuntime。
 */

import type {
  AgentRuntime,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";
import { AgentHttpError } from "../errors.js";

export type RuntimeDescriptor = {
  runtimeId: string;
  capabilities: RuntimeCapabilities;
};

/** Adapter 注册与查询。 */
export class RuntimeRegistry {
  private readonly runtimes = new Map<string, AgentRuntime>();

  /**
   * 注册 adapter（同 id 覆盖）。
   *
   * 参数:
   *   - runtime: AgentRuntime。
   */
  register(runtime: AgentRuntime): void {
    this.runtimes.set(runtime.id, runtime);
  }

  /**
   * 按 id 获取；不存在则 503。
   *
   * 参数:
   *   - runtimeId: 运行时 id。
   *
   * 返回值:
   *   - AgentRuntime。
   */
  get(runtimeId: string): AgentRuntime {
    const runtime = this.runtimes.get(runtimeId);
    if (!runtime) {
      throw new AgentHttpError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `runtime not registered: ${runtimeId}`,
        { details: { runtimeId } },
      );
    }
    return runtime;
  }

  /**
   * 是否已注册。
   *
   * 参数:
   *   - runtimeId: id。
   */
  has(runtimeId: string): boolean {
    return this.runtimes.has(runtimeId);
  }

  /**
   * 列出全部 runtime 及其 capabilities（只透传，不改写）。
   *
   * 返回值:
   *   - RuntimeDescriptor[]。
   */
  list(): RuntimeDescriptor[] {
    return [...this.runtimes.values()].map((runtime) => ({
      runtimeId: runtime.id,
      capabilities: runtime.capabilities(),
    }));
  }

  /**
   * 全部 runtime 实例。
   */
  all(): AgentRuntime[] {
    return [...this.runtimes.values()];
  }
}
