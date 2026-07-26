/**
 * Adapter 注册表：按 id 选择，新增 adapter 不改 gateway 路由代码。
 */

import type {
  AgentRuntime,
  RuntimeCapabilities,
} from "@taskdaemon/agent-protocol";
import type { OptionalRuntimeMethod } from "./errors.js";
import { AgentRuntimeError, capabilityUnsupportedError } from "./errors.js";

export type RuntimeFactory = () => AgentRuntime;

export type RegisteredRuntime = {
  id: string;
  factory: RuntimeFactory;
  /** 静态 capabilities 模板（未建 session 前供目录端点使用）。 */
  capabilitiesTemplate: RuntimeCapabilities;
};

/**
 * Runtime 注册与解析。
 */
export class RuntimeRegistry {
  private readonly byId = new Map<string, RegisteredRuntime>();

  /**
   * 注册 adapter。
   *
   * 参数:
   *   - runtime: 已构造的 runtime（取其 id 与 capabilities 模板）。
   *   - factory: 可选工厂；默认每次返回同一实例。
   *
   * 返回值: 无。
   */
  register(runtime: AgentRuntime, factory?: RuntimeFactory): void {
    const id = runtime.id;
    if (this.byId.has(id)) {
      throw new AgentRuntimeError(
        "AGENT_VALIDATION_FAILED",
        `runtime already registered: ${id}`,
        { details: { id } },
      );
    }
    this.byId.set(id, {
      id,
      factory: factory ?? (() => runtime),
      capabilitiesTemplate: runtime.capabilities(),
    });
  }

  /**
   * 按 id 解析 runtime 实例。
   *
   * 参数:
   *   - id: runtime id。
   *
   * 返回值:
   *   - AgentRuntime。
   */
  resolve(id: string): AgentRuntime {
    const entry = this.byId.get(id);
    if (!entry) {
      throw new AgentRuntimeError(
        "AGENT_RUNTIME_UNAVAILABLE",
        `unknown runtime id: ${id}`,
        { details: { id } },
      );
    }
    return entry.factory();
  }

  /**
   * 是否已注册。
   *
   * 参数:
   *   - id: runtime id。
   *
   * 返回值: boolean。
   */
  has(id: string): boolean {
    return this.byId.has(id);
  }

  /**
   * 列出已注册 id。
   *
   * 参数: 无。
   * 返回值: id 数组（稳定排序）。
   */
  listIds(): string[] {
    return [...this.byId.keys()].sort();
  }

  /**
   * 目录：各 runtime 静态 capabilities。
   *
   * 参数: 无。
   * 返回值: { id, capabilities }[]。
   */
  listCatalog(): Array<{ id: string; capabilities: RuntimeCapabilities }> {
    const out: Array<{ id: string; capabilities: RuntimeCapabilities }> = [];
    for (const [id, entry] of this.byId) {
      out.push({ id, capabilities: entry.capabilitiesTemplate });
    }
    return out.sort((a, b) => a.id.localeCompare(b.id));
  }

  /**
   * 调用可选方法前检查；未声明则抛 AGENT_CAPABILITY_UNSUPPORTED。
   *
   * 参数:
   *   - runtime: 实例。
   *   - method: 可选方法名。
   *
   * 返回值: 无（通过则静默）。
   */
  assertOptionalMethod(
    runtime: AgentRuntime,
    method: OptionalRuntimeMethod,
  ): void {
    const caps = runtime.capabilities();
    const ok =
      method === "steer"
        ? caps.steering && typeof runtime.steer === "function"
        : method === "followUp"
          ? caps.followUp && typeof runtime.followUp === "function"
          : method === "setModel"
            ? caps.modelSwitch !== "none" &&
              typeof runtime.setModel === "function"
            : typeof runtime.listModels === "function";
    if (!ok) {
      throw capabilityUnsupportedError(runtime.id, method);
    }
  }
}

/**
 * 创建空注册表。
 *
 * 参数: 无。
 * 返回值: RuntimeRegistry。
 */
export function createRuntimeRegistry(): RuntimeRegistry {
  return new RuntimeRegistry();
}
