import type { Plugin } from "vite";
import path from "node:path";

/**
 * Resolves imports that target files outside the webui directory
 * (from the original monorepo) to local stub files.
 */
export function stubMissingMonorepoImports(root: string): Plugin {
  const stubsDir = path.resolve(root, "stubs");

  const redirects: Array<{ pattern: RegExp; target: string }> = [
    { pattern: /\/src\/gateway\/control-ui-contract\.(?:ts|js)$/, target: "gateway/control-ui-contract.ts" },
    { pattern: /\/src\/gateway\/events\.(?:ts|js)$/, target: "gateway/events.ts" },
    { pattern: /\/src\/gateway\/device-auth\.(?:ts|js)$/, target: "gateway/device-auth.ts" },
    { pattern: /\/src\/shared\/assistant-identity-values\.(?:ts|js)$/, target: "shared/assistant-identity-values.ts" },
    { pattern: /\/src\/shared\/operator-scope-compat\.(?:ts|js)$/, target: "shared/operator-scope-compat.ts" },
    { pattern: /\/src\/shared\/chat-envelope\.(?:ts|js)$/, target: "shared/chat-envelope.ts" },
    { pattern: /\/src\/shared\/chat-message-content\.(?:ts|js)$/, target: "shared/chat-message-content.ts" },
    { pattern: /\/src\/shared\/text\/citation-control-markers\.(?:ts|js)$/, target: "shared/text/citation-control-markers.ts" },
    { pattern: /\/src\/shared\/text\/assistant-visible-text\.(?:ts|js)$/, target: "shared/text/assistant-visible-text.ts" },
    { pattern: /\/src\/talk\/talk-session-controller\.(?:ts|js)$/, target: "talk/talk-session-controller.ts" },
    { pattern: /\/src\/talk\/agent-consult-tool\.(?:ts|js)$/, target: "talk/agent-consult-tool.ts" },
    { pattern: /\/src\/talk\/agent-run-control-shared\.(?:ts|js)$/, target: "talk/agent-run-control-shared.ts" },
    { pattern: /\/src\/talk\/talk-events\.(?:ts|js)$/, target: "talk/talk-events.ts" },
    { pattern: /\/src\/agents\/internal-runtime-context\.(?:ts|js)$/, target: "agents/internal-runtime-context.ts" },
    { pattern: /\/src\/auto-reply\/reply\/strip-inbound-meta\.(?:ts|js)$/, target: "auto-reply/strip-inbound-meta.ts" },
    { pattern: /\/src\/auto-reply\/commands-registry\.shared\.(?:ts|js)$/, target: "auto-reply/commands-registry.shared.ts" },
    { pattern: /\/src\/chat\/canvas-render\.(?:ts|js)$/, target: "chat/canvas-render.ts" },
    { pattern: /\/src\/chat\/tool-content\.(?:ts|js)$/, target: "chat/tool-content.ts" },
    { pattern: /\/src\/media\/parse\.(?:ts|js)$/, target: "media/parse.ts" },
    { pattern: /\/src\/utils\/directive-tags\.(?:ts|js)$/, target: "utils/directive-tags.ts" },
    { pattern: /\/src\/infra\/format-time\/format-duration\.(?:ts|js)$/, target: "infra/format-time/format-duration.ts" },
    { pattern: /\/src\/infra\/format-time\/format-relative\.(?:ts|js)$/, target: "infra/format-time/format-relative.ts" },
    { pattern: /\/src\/infra\/approval-display-paths\.(?:ts|js)$/, target: "infra/approval-display-paths.ts" },
    { pattern: /\/src\/config\/merge-patch\.(?:ts|js)$/, target: "config/merge-patch.ts" },
    { pattern: /\/packages\/terminal-core\/src\/ansi\.(?:ts|js)$/, target: "packages/terminal-core/ansi.ts" },
    { pattern: /\/packages\/gateway-protocol\/src\/connect-error-details\.(?:ts|js)$/, target: "../packages/gateway-protocol/src/connect-error-details.ts" },
    { pattern: /\/packages\/gateway-protocol\/src\/version\.(?:ts|js)$/, target: "../packages/gateway-protocol/src/version.ts" },
    { pattern: /\/packages\/gateway-protocol\/src\/client-info\.(?:ts|js)$/, target: "../packages/gateway-protocol/src/client-info.ts" },
    { pattern: /\/packages\/gateway-protocol\/src\/startup-unavailable\.(?:ts|js)$/, target: "../packages/gateway-protocol/src/startup-unavailable.ts" },
    { pattern: /\/packages\/gateway-protocol\/src\/index\.(?:ts|js)$/, target: "../packages/gateway-protocol/src/index.ts" },
    { pattern: /\/src\/agents\/tool-policy-shared\.(?:ts|js)$/, target: "agents/tool-policy-shared.ts" },
    { pattern: /\/src\/shared\/device-auth-store\.(?:ts|js)$/, target: "shared/device-auth-store.ts" },
    { pattern: /\/src\/shared\/device-auth\.(?:ts|js)$/, target: "shared/device-auth.ts" },
    { pattern: /\/src\/agents\/tool-display-common\.(?:ts|js)$/, target: "agents/tool-display-common.ts" },
    { pattern: /\/src\/agents\/tool-display-exec\.(?:ts|js)$/, target: "agents/tool-display-exec.ts" },
    { pattern: /\/src\/shared\/device-pairing-access\.(?:ts|js)$/, target: "shared/device-pairing-access.ts" },
    { pattern: /\/src\/shared\/usage-aggregates\.(?:ts|js)$/, target: "shared/usage-aggregates.ts" },
    { pattern: /\/src\/shared\/session-usage-timeseries-types\.(?:ts|js)$/, target: "shared/session-usage-timeseries-types.ts" },
    { pattern: /\/src\/shared\/usage-types\.(?:ts|js)$/, target: "shared/usage-types.ts" },
    { pattern: /\/src\/shared\/config-ui-hints-types\.(?:ts|js)$/, target: "shared/config-ui-hints-types.ts" },
    { pattern: /\/src\/shared\/session-types\.(?:ts|js)$/, target: "shared/session-types.ts" },
    { pattern: /\/src\/config\/sessions\/types\.(?:ts|js)$/, target: "config/sessions-types.ts" },
    { pattern: /\/src\/cron\/types-shared\.(?:ts|js)$/, target: "cron/types-shared.ts" },
    { pattern: /\/src\/config\/zod-schema\.(?:ts|js)$/, target: "config/zod-schema.ts" },
    { pattern: /\/src\/infra\/update-startup\.(?:ts|js)$/, target: "infra/update-startup.ts" },
    { pattern: /\/src\/gateway\/server-methods\/models-auth-status\.(?:ts|js)$/, target: "gateway/models-auth-status.ts" },
    { pattern: /\/apps\//, target: "shared/tool-display.json" },
  ];

  return {
    name: "stub-missing-monorepo-imports",
    enforce: "pre",
    async resolveId(source, importer, options) {
      // Only handle relative imports
      if (!source.startsWith(".")) return null;

      const resolved = await this.resolve(source, importer, { ...options, skipSelf: true });
      if (resolved) return null;

      // Try to map the source
      if (!importer) return null;
      const absPath = path.resolve(path.dirname(importer), source);

      for (const { pattern, target } of redirects) {
        if (pattern.test(absPath)) {
          return path.resolve(stubsDir, target);
        }
      }

      return null;
    },
  };
}
