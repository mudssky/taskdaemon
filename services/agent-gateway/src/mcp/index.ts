/**
 * MCP 目录公共导出（G6）。
 */

export {
  emptyMcpCatalog,
  McpCatalog,
  splitQualified,
} from "./catalog.js";
export {
  assertGatewayToolAllowed,
  isAllowlisted,
} from "./policy.js";
export { redactToolArgs } from "./redaction.js";
export { mountMcpRoutes } from "./routes.js";
export type {
  McpCatalogConfig,
  McpServerConfig,
  McpToolDescriptor,
  McpTransport,
} from "./types.js";
