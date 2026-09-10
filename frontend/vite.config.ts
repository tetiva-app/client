import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

// Read the app version from package.json so the About section and any
// runtime version checks share a single source of truth with npm/vite.
const require = createRequire(import.meta.url);
const pkg = require("./package.json") as { version: string };

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  plugins: [vue(), tailwindcss(), wails("./bindings")],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    // Wails' dev proxy dials tcp4; a ::1-only Vite leaves the window blank.
    host: "127.0.0.1",
  },
});
