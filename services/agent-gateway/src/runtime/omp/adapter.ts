/**
 * OMP CLI adapter（G0：cli-spawn，coldStartCost=high，memoryClass=heavy）。
 * history 仅 messages；entries 不保证。
 * general 需 SessionOpts 强制裁剪（非开箱干净）——不做 Pi 能力模拟。
 */

import {
  OMP_CLI_CAPABILITIES,
  type RuntimeCapabilities,
  type SessionOpts,
} from "@taskdaemon/agent-protocol";
import {
  type CliAdapterConfig,
  CliRpcAgentRuntime,
  type TransportFactory,
} from "../cli-adapter.js";
import type { RuntimeLogger } from "../log.js";
import type { RpcClient } from "../rpc-client.js";

export type OmpAdapterOptions = {
  command?: string;
  defaultModel?: string;
  defaultThinking?: string;
  transportFactory?: TransportFactory;
  logger?: RuntimeLogger;
  traceId?: string;
  capabilitiesOverride?: RuntimeCapabilities;
};

/**
 * 构造 OMP AgentRuntime。
 *
 * 参数:
 *   - options: 注入与默认模型。
 *
 * 返回值:
 *   - CliRpcAgentRuntime。
 */
export function createOmpAdapter(
  options: OmpAdapterOptions = {},
): CliRpcAgentRuntime {
  const capabilities = options.capabilitiesOverride ?? OMP_CLI_CAPABILITIES;
  const command = options.command ?? "omp";
  const defaultModel =
    options.defaultModel ?? process.env.AGENT_MODEL ?? "xh/grok-4.5";
  const defaultThinking =
    options.defaultThinking ?? process.env.AGENT_THINKING ?? "off";

  const config: CliAdapterConfig = {
    id: "omp",
    capabilities,
    defaultModel,
    defaultThinking,
    logger: options.logger,
    traceId: options.traceId,
    transportFactory: options.transportFactory,
    buildSpawn(opts: SessionOpts, sessionDir: string) {
      const model = opts.model ?? defaultModel;
      const thinking = opts.thinkingLevel ?? defaultThinking;
      // OMP 要求 --cwd；general 无 workspace 时用 sessionDir 作沙箱 cwd
      const cwd =
        opts.cwd ??
        (opts.profile === "general" || opts.workspaceBinding === false
          ? sessionDir
          : process.cwd());
      const args = [
        "--mode",
        "rpc",
        "--model",
        model,
        "--thinking",
        thinking,
        "--session-dir",
        sessionDir,
        "--cwd",
        cwd,
      ];
      if (opts.sessionPersistence !== "runtime-file") {
        args.push("--no-session");
      }
      // general / tools=none：尽量收敛工具面（G0：--no-tools 仍可能有 MCP，adapter 再 afterReady 裁剪）
      if (
        opts.tools === "none" ||
        opts.profile === "general" ||
        (Array.isArray(opts.tools) && opts.tools.length === 0)
      ) {
        args.push("--no-tools");
      }
      return { command, args, cwd };
    },
    async afterReady(client: RpcClient, opts: SessionOpts) {
      // OMP general：追加 system 裁剪提示，避免默认 coding 人格/扩展噪声
      if (
        opts.profile !== "general" &&
        !opts.systemPrompt &&
        !opts.appendSystemPrompt
      ) {
        return;
      }
      const parts: string[] = [];
      if (opts.systemPrompt) parts.push(opts.systemPrompt);
      if (opts.appendSystemPrompt) parts.push(opts.appendSystemPrompt);
      if (opts.profile === "general" && parts.length === 0) {
        parts.push(
          "You are a general assistant. Do not assume a coding workspace. Avoid project-specific tooling unless asked.",
        );
      }
      if (parts.length === 0) return;
      // 尽力而为：命令名因版本可能不同；失败不假装成功
      try {
        await client.request(
          "set_system_prompt",
          { message: parts.join("\n\n") },
          5_000,
        );
      } catch {
        try {
          await client.request(
            "append_system_prompt",
            { message: parts.join("\n\n") },
            5_000,
          );
        } catch {
          // G0：OMP general 非开箱干净；无 RPC 时仅依赖 --no-tools + cwd 沙箱
        }
      }
    },
  };

  return new CliRpcAgentRuntime(config);
}

export { OMP_CLI_CAPABILITIES };
