/**
 * Pi CLI adapter（G0：cli-spawn，coldStartCost=high）。
 * SDK-inprocess 通过 createPiAdapter({ attachMode: "sdk-inprocess" }) 声明不同 capabilities；
 * 真 SDK 会话工厂可注入，默认测试用 FakeTransport。
 */

import {
  type AttachMode,
  PI_CLI_CAPABILITIES,
  PI_SDK_CAPABILITIES,
  type RuntimeCapabilities,
  type SessionOpts,
} from "@taskdaemon/agent-protocol";
import {
  type CliAdapterConfig,
  CliRpcAgentRuntime,
  type TransportFactory,
} from "../cli-adapter.js";
import type { RuntimeLogger } from "../log.js";

export type PiAdapterOptions = {
  /** 默认 cli-spawn（G0 MVP 实测路径）。 */
  attachMode?: AttachMode;
  command?: string;
  defaultModel?: string;
  defaultThinking?: string;
  transportFactory?: TransportFactory;
  logger?: RuntimeLogger;
  traceId?: string;
  capabilitiesOverride?: RuntimeCapabilities;
};

/**
 * 构造 Pi AgentRuntime。
 *
 * 参数:
 *   - options: 接入方式与注入点。
 *
 * 返回值:
 *   - CliRpcAgentRuntime（实现 AgentRuntime）。
 */
export function createPiAdapter(
  options: PiAdapterOptions = {},
): CliRpcAgentRuntime {
  const attachMode = options.attachMode ?? "cli-spawn";
  const capabilities =
    options.capabilitiesOverride ??
    (attachMode === "sdk-inprocess"
      ? PI_SDK_CAPABILITIES
      : PI_CLI_CAPABILITIES);
  const command = options.command ?? "pi";
  const defaultModel =
    options.defaultModel ?? process.env.AGENT_MODEL ?? "xh/grok-4.5";
  const defaultThinking =
    options.defaultThinking ?? process.env.AGENT_THINKING ?? "off";

  const config: CliAdapterConfig = {
    id: "pi",
    capabilities,
    defaultModel,
    defaultThinking,
    logger: options.logger,
    traceId: options.traceId,
    transportFactory: options.transportFactory,
    buildSpawn(opts: SessionOpts, sessionDir: string) {
      const model = opts.model ?? defaultModel;
      const thinking = opts.thinkingLevel ?? defaultThinking;
      const args = [
        "--mode",
        "rpc",
        "--model",
        model,
        "--thinking",
        thinking,
        "--session-dir",
        sessionDir,
      ];
      if (opts.sessionPersistence !== "runtime-file") {
        args.push("--no-session");
      }
      if (opts.tools === "none" || opts.profile === "general") {
        args.push("--no-tools");
      } else if (Array.isArray(opts.tools) && opts.tools.length === 0) {
        args.push("--no-tools");
      }
      // general：不强制 cwd
      const cwd =
        opts.workspaceBinding === false || opts.profile === "general"
          ? opts.cwd
          : opts.cwd;
      return { command, args, cwd };
    },
  };

  return new CliRpcAgentRuntime(config);
}

export { PI_CLI_CAPABILITIES, PI_SDK_CAPABILITIES };
