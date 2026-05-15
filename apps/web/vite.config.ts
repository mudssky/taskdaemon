import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const wailsVitePort = Number.parseInt(process.env.WAILS_VITE_PORT ?? "", 10);

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts",
  },
  server: {
    host: "127.0.0.1",
    port: Number.isNaN(wailsVitePort) ? 5173 : wailsVitePort,
    strictPort: !Number.isNaN(wailsVitePort),
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
