/**
 * G6 MCP 目录与策略测试（不依赖真实 MCP server）。
 */

import { describe, expect, it } from "vitest";
import { AgentHttpError } from "../src/errors.js";
import { McpCatalog } from "../src/mcp/catalog.js";
import { assertGatewayToolAllowed } from "../src/mcp/policy.js";
import { redactToolArgs } from "../src/mcp/redaction.js";
import { createTestContext, jsonRequest } from "./helpers.js";

const sampleServers = [
  {
    id: "fs",
    enabled: true,
    transport: "stub" as const,
    tools: [
      { name: "read_file", description: "read" },
      { name: "write_file", description: "write" },
    ],
  },
  {
    id: "down",
    enabled: true,
    transport: "stub" as const,
    available: false,
    tools: [{ name: "broken_tool" }],
  },
  {
    id: "tenant-a-only",
    enabled: true,
    transport: "stub" as const,
    tenants: ["tenant-a"],
    tools: [{ name: "secret_tool" }],
  },
];

describe("MCP catalog", () => {
  it("按租户过滤 server，不可用 server 降级省略工具", () => {
    const catalog = new McpCatalog({
      reloadMode: "restart",
      servers: sampleServers,
    });
    const toolsDev = catalog.listToolsForTenant("dev");
    expect(toolsDev.map((t) => t.qualifiedName)).toEqual([
      "fs/read_file",
      "fs/write_file",
    ]);
    expect(toolsDev.some((t) => t.serverId === "down")).toBe(false);

    const toolsA = catalog.listToolsForTenant("tenant-a");
    expect(
      toolsA.some((t) => t.qualifiedName === "tenant-a-only/secret_tool"),
    ).toBe(true);
  });

  it("默认 allowlist 空 = 拒绝 MCP tool", () => {
    const catalog = new McpCatalog({
      reloadMode: "restart",
      servers: sampleServers,
    });
    expect(() =>
      assertGatewayToolAllowed("fs/read_file", [], undefined, catalog, "dev"),
    ).toThrow(AgentHttpError);
    try {
      assertGatewayToolAllowed("fs/read_file", [], undefined, catalog, "dev");
    } catch (err) {
      expect(err).toBeInstanceOf(AgentHttpError);
      expect((err as AgentHttpError).code).toBe("MCP_TOOL_NOT_ALLOWED");
    }
  });

  it("server/* 与 server/tool 粒度放行", () => {
    const catalog = new McpCatalog({
      reloadMode: "restart",
      servers: sampleServers,
    });
    expect(() =>
      assertGatewayToolAllowed(
        "fs/read_file",
        ["fs/*"],
        undefined,
        catalog,
        "dev",
      ),
    ).not.toThrow();
    expect(() =>
      assertGatewayToolAllowed(
        "fs/write_file",
        ["fs/read_file"],
        undefined,
        catalog,
        "dev",
      ),
    ).toThrow(/MCP_TOOL_NOT_ALLOWED|denied/);
  });

  it("脱敏敏感入参", () => {
    const summary = redactToolArgs({
      path: "/tmp/a",
      token: "super-secret",
      nested: { password: "p", ok: 1 },
    });
    expect(summary).toEqual({
      path: "/tmp/a",
      token: "[REDACTED]",
      nested: { password: "[REDACTED]", ok: 1 },
    });
  });

  it("GET /v1/mcp/tools 可查询会话工具", async () => {
    const { app } = createTestContext({
      mcp: { reloadMode: "restart", servers: sampleServers },
      toolAllowlist: ["fs/*"],
    });
    const res = await jsonRequest(app, "GET", "/v1/mcp/tools");
    expect(res.status).toBe(200);
    const body = res.json as {
      reloadMode: string;
      items: Array<{ qualifiedName: string }>;
    };
    expect(body.reloadMode).toBe("restart");
    expect(body.items.map((i) => i.qualifiedName)).toContain("fs/read_file");
    expect(body.items.some((i) => i.qualifiedName.includes("broken"))).toBe(
      false,
    );
  });

  it("创建 thread 时 MCP tool 越界返回 MCP_TOOL_NOT_ALLOWED", async () => {
    const { app } = createTestContext({
      mcp: { reloadMode: "restart", servers: sampleServers },
      toolAllowlist: ["fs/read_file"],
    });
    const res = await jsonRequest(app, "POST", "/v1/threads", {
      tools: ["fs/write_file"],
    });
    expect(res.status).toBe(403);
    expect((res.json as { error: { code: string } }).error.code).toBe(
      "MCP_TOOL_NOT_ALLOWED",
    );
  });
});
