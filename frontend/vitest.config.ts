import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

// Dedicated Vitest config. Scopes unit tests to `src/**/*.test.ts` so the
// Playwright e2e specs under `tests/` (which call test.describe at import time
// and would otherwise be collected and error) are excluded. Mirrors the `@`
// alias from vite.config.ts; tests import only .ts modules, so the Vue plugin
// and Vite `define` are not needed here.
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    include: ["src/**/*.test.ts"],
    environment: "node",
  },
});
