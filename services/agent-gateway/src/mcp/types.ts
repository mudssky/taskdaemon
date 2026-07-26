/**
 * MCP 目录类型（G6）。
 * gateway 只做配置与策略；工具实际执行由 runtime 承担。
 */

/** MCP 连接方式。 */
export type McpTransport = "stdio" | "http" | "stub";

/** 单个 MCP server 配置。 */
export type McpServerConfig = {
  /** 稳定标识，用于 allowlist `server/tool`。 */
  id: string;
  /** 是否启用；false 时对所有会话不可见。 */
  enabled: boolean;
  /** 连接方式。 */
  transport: McpTransport;
  /** stdio 命令。 */
  command?: string;
  /** stdio 参数。 */
  args?: string[];
  /** http 端点。 */
  url?: string;
  /**
   * 可用租户集合；空/省略 = 全部租户。
   * 按会话租户过滤 server 集合。
   */
  tenants?: string[];
  /**
   * stub 模式声明的工具清单（测试与无真实连接时）。
   * 真实 stdio/http 连接失败时降级为空列表，不拖垮会话。
   */
  tools?: Array<{ name: string; description?: string }>;
  /**
   * 是否可用；默认 true。
   * false 或探测失败时：该 server 工具从清单中省略（会话降级，不整体失败）。
   */
  available?: boolean;
};

/** 会话可见的工具条目。 */
export type McpToolDescriptor = {
  /** `serverId/toolName` 规范名。 */
  qualifiedName: string;
  serverId: string;
  toolName: string;
  description?: string;
  /** server 当前是否可用。 */
  serverAvailable: boolean;
};

/** MCP 目录配置块。 */
export type McpCatalogConfig = {
  /**
   * 配置生效方式：restart = 进程启动时从环境/配置加载，运行中变更需重启。
   * 文档化见 CONFIG.md。
   */
  reloadMode: "restart";
  servers: McpServerConfig[];
};
