/// <reference types="vitest/config" />
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// base "./" is required: the extension UI is served from the image
// filesystem, not a web origin.
const daemonProxy = {
  "/api": { target: "http://127.0.0.1:8787", changeOrigin: true },
};

export default defineConfig({
  plugins: [react()],
  base: "./",
  build: { outDir: "build", sourcemap: false, target: "es2022" },
  // Outside Docker Desktop, /api rides a same-origin proxy to the daemon's
  // loopback dev listener (no CORS, no extra daemon surface).
  server: { proxy: daemonProxy },
  preview: { proxy: daemonProxy },
  test: {
    environment: "node",
    include: ["test/**/*.test.ts"],
  },
});
