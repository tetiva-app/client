/// <reference types="vite/client" />

// Injected by Vite `define` from package.json — see vite.config.ts.
declare const __APP_VERSION__: string;

// Set only by Playwright's addInitScript so openExternal records instead of
// launching the system browser. Keep this file a script: an `export`/`declare
// global` here would scope __APP_VERSION__ to a module and break its callers.
interface Window {
  __TETIVA_NO_EXTERNAL_OPEN__?: boolean;
  __lastExternalUrl?: string;
}
