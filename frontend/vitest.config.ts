import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "node:url";

// Dedicated Vitest config. Scopes unit tests to `src/**/*.test.ts` so the
// Playwright e2e specs under `tests/` (which call test.describe at import time
// and would otherwise be collected and error) are excluded. Mirrors the `@`
// alias from vite.config.ts. The Vue plugin lets a component test mount an
// SFC, which needs the client build rather than the SSR one the node
// environment picks by default; Vite `define` is not needed here.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    include: ["src/**/*.test.ts"],
    environment: "node",
    setupFiles: ["./src/test-setup/locale.ts"],
    testTransformMode: { web: ["**/src/components/**/*.test.ts"] },
  },
});
