import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const defaultWebPort = 9245;
const defaultApiOrigin = "http://127.0.0.1:39245";

const webPort =
  parsePort(process.env.WAILS_VITE_PORT) ??
  parsePort(process.env.TASKDAEMON_WEB_PORT) ??
  defaultWebPort;
const apiOrigin =
  process.env.TASKDAEMON_API_ORIGIN ??
  process.env.VITE_TASKDAEMON_API_ORIGIN ??
  defaultApiOrigin;

function parsePort(value: string | undefined) {
  if (value === undefined || value.trim() === "") {
    return undefined;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? undefined : parsed;
}

export default defineConfig({
  plugins: [react()],
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
      "/api": apiOrigin,
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
