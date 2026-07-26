/**
 * MCP server 目录：配置驱动、按租户过滤、不可用时降级。
 */

import type {
  McpCatalogConfig,
  McpServerConfig,
  McpToolDescriptor,
} from "./types.js";

/**
 * 内存 MCP 目录（启动时加载；reloadMode=restart）。
 */
export class McpCatalog {
  private readonly servers: McpServerConfig[];
  readonly reloadMode: "restart";

  /**
   * 参数:
   *   - config: MCP 目录配置。
   */
  constructor(config: McpCatalogConfig) {
    this.reloadMode = config.reloadMode;
    this.servers = config.servers.map((s) => ({ ...s }));
  }

  /**
   * 列出配置中的 server（含禁用），供运维查看。
   *
   * 返回值:
   *   - server 配置快照（不含密钥；当前模型无密钥字段）。
   */
  listServers(): McpServerConfig[] {
    return this.servers.map((s) => ({
      ...s,
      args: s.args ? [...s.args] : undefined,
    }));
  }

  /**
   * 解析租户可用的已启用 server。
   *
   * 参数:
   *   - tenantId: 租户 id。
   *
   * 返回值:
   *   - 启用且租户匹配的 server 列表。
   */
  serversForTenant(tenantId: string): McpServerConfig[] {
    return this.servers.filter((server) => {
      if (!server.enabled) return false;
      if (!server.tenants || server.tenants.length === 0) return true;
      return server.tenants.includes(tenantId);
    });
  }

  /**
   * 解析会话可用工具清单。
   * server 不可用时省略其工具，不抛错（会话降级）。
   *
   * 参数:
   *   - tenantId: 租户 id。
   *
   * 返回值:
   *   - 工具描述列表；仅包含可用 server 的工具。
   */
  listToolsForTenant(tenantId: string): McpToolDescriptor[] {
    const tools: McpToolDescriptor[] = [];
    for (const server of this.serversForTenant(tenantId)) {
      const available = server.available !== false;
      if (!available) {
        // 降级：跳过不可用 server，不让整个会话失败
        continue;
      }
      for (const tool of server.tools ?? []) {
        tools.push({
          qualifiedName: `${server.id}/${tool.name}`,
          serverId: server.id,
          toolName: tool.name,
          description: tool.description,
          serverAvailable: true,
        });
      }
    }
    return tools;
  }

  /**
   * 判断工具名是否为目录内 MCP 工具（含 `server/tool` 形式）。
   *
   * 参数:
   *   - toolName: 工具名。
   *   - tenantId: 租户。
   *
   * 返回值:
   *   - true 表示应按 MCP 策略处理。
   */
  isMcpTool(toolName: string, tenantId: string): boolean {
    if (toolName.includes("/")) {
      const [serverId, name] = splitQualified(toolName);
      if (!serverId || !name) return false;
      return this.serversForTenant(tenantId).some((s) => s.id === serverId);
    }
    return this.listToolsForTenant(tenantId).some(
      (t) => t.toolName === toolName,
    );
  }

  /**
   * 将工具名规范为 `server/tool`；无法解析时返回原名。
   *
   * 参数:
   *   - toolName: 原始工具名。
   *   - tenantId: 租户。
   *
   * 返回值:
   *   - 规范名。
   */
  qualifyToolName(toolName: string, tenantId: string): string {
    if (toolName.includes("/")) return toolName;
    const matches = this.listToolsForTenant(tenantId).filter(
      (t) => t.toolName === toolName,
    );
    const only = matches.length === 1 ? matches[0] : undefined;
    if (only) return only.qualifiedName;
    return toolName;
  }
}

/**
 * 拆分 `server/tool`。
 *
 * 参数:
 *   - qualified: 规范名。
 *
 * 返回值:
 *   - [serverId, toolName]。
 */
export function splitQualified(qualified: string): [string, string] {
  const idx = qualified.indexOf("/");
  if (idx <= 0) return ["", qualified];
  return [qualified.slice(0, idx), qualified.slice(idx + 1)];
}

/**
 * 从空配置构造目录。
 *
 * 返回值:
 *   - 空 McpCatalog。
 */
export function emptyMcpCatalog(): McpCatalog {
  return new McpCatalog({ reloadMode: "restart", servers: [] });
}
