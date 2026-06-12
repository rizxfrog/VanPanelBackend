import path from "node:path";
import { fileURLToPath } from "node:url";
import type { UserConfig } from "vite";

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(here, "dist");

export default {
  base: "/",
  define: {
    OPENCLAW_CONTROL_UI_BUILD_ID: JSON.stringify("vanpanel"),
  },
  publicDir: path.resolve(here, "public"),
  optimizeDeps: {
    include: [
      "ipaddr.js",
      "lit/directives/repeat.js",
      "markdown-it-task-lists",
    ],
  },
  build: {
    outDir,
    emptyOutDir: true,
    sourcemap: false,
    chunkSizeWarningLimit: 1024,
  },
  server: {
    host: true,
    port: 5173,
    proxy: {
      "/api": "http://localhost:8889",
    },
  },
} satisfies UserConfig;
