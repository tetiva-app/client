# Tetiva — frontend

The Vue 3 + TypeScript UI for Tetiva. See the [root README](../README.md)
for the project overview.

## Running

Two ways to run it:

- **In a browser, with mock data** — no Go backend needed:

  ```sh
  npx vite
  ```

  Opens http://localhost:5173. The app detects the browser environment and uses
  in-memory mock services.

- **As the full desktop app** — run `wails3 dev` from the repo root. This
  generates the Wails bindings and talks to the real SQLite backend.

The factory in `src/services/` chooses the mock or Wails implementation at module
load, depending on whether the Wails runtime is present.

## Commands

```sh
npm run dev          # Vite dev server
npm run build        # type-check (vue-tsc) + production build
npm test             # unit tests (Vitest)
npm run test:e2e     # end-to-end tests (Playwright)
```

## Stack

Pinia for state, Tailwind CSS 4 for styling, CodeMirror 6 for editors, and
reka-ui for components.
