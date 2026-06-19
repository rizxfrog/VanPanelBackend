import path from "node:path";
import { fileURLToPath } from "node:url";
import swc from "unplugin-swc";
import type { PluginOption, UserConfig } from "vite";
import { stubMissingMonorepoImports } from "./stub-missing-imports.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(here, "dist");

export default {
  base: "/",
  define: {
    AGENTOPS_CONTROL_UI_BUILD_ID: JSON.stringify("vanpanel"),
  },
  publicDir: path.resolve(here, "public"),
  resolve: {
    alias: {
      zod: path.resolve(here, "stubs/zod.ts"),
      "@agentops/net-policy/redact-sensitive-url": path.resolve(here, "stubs/net-policy/redact-sensitive-url.ts"),
    },
  },
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
  plugins: [
    swc.vite({
      jsc: {
        parser: {
          syntax: "typescript",
          decorators: true,
        },
        transform: {
          legacyDecorator: true,
          decoratorMetadata: true,
          useDefineForClassFields: false,
        },
        target: "es2022",
      },
    }),
    stubMissingMonorepoImports(here) as PluginOption,
  ],
} satisfies UserConfig;
