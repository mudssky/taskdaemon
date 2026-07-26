import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const defaultWebPort = 9246;
const defaultApiOrigin = "http://127.0.0.1:39246";

const webPort =
  parsePort(process.env.TASKDAEMON_AGENT_WEB_PORT) ?? defaultWebPort;
const apiOrigin =
  process.env.TASKDAEMON_AGENT_API_ORIGIN ??
  process.env.VITE_AGENT_API_ORIGIN ??
  defaultApiOrigin;

/**
 * 解析端口环境变量。
 *
 * 参数:
 *   - value: 原始字符串。
 *
 * 返回值:
 *   - 合法端口或 undefined。
 */
function parsePort(value: string | undefined): number | undefined {
  if (value === undefined || value.trim() === "") {
    return undefined;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? undefined : parsed;
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts",
  },
  server: {
    host: "127.0.0.1",
    port: webPort,
    strictPort: true,
    proxy: {
      // G2 就绪后把 /v1 转到真实 gateway；默认开发走 mock client。
      "/v1": apiOrigin,
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
