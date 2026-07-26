/**
 * Runtime adapter 层公共导出（G3）。
 * gateway 路由只依赖 AgentRuntime + 本注册表，不 import Pi/OMP 专有类型。
 */

export {
  type CliAdapterConfig,
  CliRpcAgentRuntime,
  type TransportFactory,
} from "./cli-adapter.js";
export {
  AgentRuntimeError,
  capabilityUnsupportedError,
  type OptionalRuntimeMethod,
  sessionNotFoundError,
  wrapInternalError,
} from "./errors.js";
export {
  createEventMapperState,
  type EventMapperContext,
  type EventMapperState,
  mapNativeToRuntimeEvents,
} from "./event-mapper.js";
export {
  createJsonlFramer,
  type JsonlFrameHandler,
  type JsonlFramer,
  type JsonlFramerOptions,
} from "./jsonl-framer.js";
export {
  createRuntimeLogger,
  type LogLevel,
  type RuntimeLogFields,
  type RuntimeLogger,
  withLogFields,
} from "./log.js";
export { createOmpAdapter, type OmpAdapterOptions } from "./omp/index.js";
export { createPiAdapter, type PiAdapterOptions } from "./pi/index.js";
export {
  derivePoolPolicies,
  derivePoolPolicy,
  type PoolPolicyOptions,
  type RuntimePoolPolicy,
} from "./pool.js";
export {
  ProcessTransport,
  type ProcessTransportOptions,
} from "./process-transport.js";
export {
  createRuntimeRegistry,
  type RegisteredRuntime,
  type RuntimeFactory,
  RuntimeRegistry,
} from "./registry.js";
export {
  FakeTransport,
  type NativeMessage,
  RpcClient,
  type RpcClientOptions,
  type RpcTransport,
} from "./rpc-client.js";
export {
  isSensitiveKey,
  REDACTED,
  sanitizeModelInfo,
  sanitizeSecrets,
} from "./sanitize.js";

import type { AgentRuntime } from "@taskdaemon/agent-protocol";
import type { TransportFactory } from "./cli-adapter.js";
import type { RuntimeLogger } from "./log.js";
import { createOmpAdapter } from "./omp/index.js";
import { createPiAdapter } from "./pi/index.js";
import { createRuntimeRegistry, type RuntimeRegistry } from "./registry.js";

export type DefaultRuntimesOptions = {
  piTransportFactory?: TransportFactory;
  ompTransportFactory?: TransportFactory;
  logger?: RuntimeLogger;
  traceId?: string;
  /** 默认注册 pi + omp。 */
  include?: Array<"pi" | "omp">;
};

/**
 * 创建预装 Pi/OMP 的注册表（生产默认入口）。
 *
 * 参数:
 *   - options: 传输注入与 traceId。
 *
 * 返回值:
 *   - RuntimeRegistry。
 */
export function createDefaultRuntimeRegistry(
  options: DefaultRuntimesOptions = {},
): RuntimeRegistry {
  const registry = createRuntimeRegistry();
  const include = options.include ?? ["pi", "omp"];
  if (include.includes("pi")) {
    const pi = createPiAdapter({
      transportFactory: options.piTransportFactory,
      logger: options.logger,
      traceId: options.traceId,
    });
    registry.register(pi, () =>
      createPiAdapter({
        transportFactory: options.piTransportFactory,
        logger: options.logger,
        traceId: options.traceId,
      }),
    );
  }
  if (include.includes("omp")) {
    const omp = createOmpAdapter({
      transportFactory: options.ompTransportFactory,
      logger: options.logger,
      traceId: options.traceId,
    });
    registry.register(omp, () =>
      createOmpAdapter({
        transportFactory: options.ompTransportFactory,
        logger: options.logger,
        traceId: options.traceId,
      }),
    );
  }
  return registry;
}

/**
 * 仅类型再导出：保证 gateway 层只看到 AgentRuntime。
 */
export type { AgentRuntime };
