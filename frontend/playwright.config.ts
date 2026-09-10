import { defineConfig, devices } from '@playwright/test';
import pkg from './package.json' with { type: 'json' };

const MOCK_URL = 'http://127.0.0.1:5173';
const REAL_URL = 'http://127.0.0.1:8080';
const REAL_BACKEND = process.env.TETIVA_REAL_BACKEND === '1';

// Seed the seen-version and the welcome flag so neither first-launch modal
// blocks specs; the first-launch specs opt out with a clean storageState.
function seededState(origin: string) {
  return {
    cookies: [],
    origins: [{
      origin,
      localStorage: [{
        name: 'gophercourier.settings',
        value: JSON.stringify({
          lastSeenWhatsNewVersion: pkg.version,
          onboardingCompletedAt: '2026-01-01T00:00:00.000Z',
        }),
      }],
    }],
  };
}

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: MOCK_URL,
    trace: 'on-first-retry',
    colorScheme: 'dark',
    storageState: seededState(MOCK_URL),
  },
  projects: [
    {
      name: 'chromium',
      testIgnore: /-real-backend\.spec\.ts$/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      // Drives the Go backend of a server-tagged binary started by hand (see
      // tests/ws-real-backend.spec.ts) — never the Vite mock server.
      name: 'real-backend',
      testMatch: /-real-backend\.spec\.ts$/,
      use: {
        ...devices['Desktop Chrome'],
        baseURL: REAL_URL,
        storageState: seededState(REAL_URL),
      },
    },
  ],
  webServer: REAL_BACKEND ? undefined : {
    command: 'npm run dev',
    url: MOCK_URL,
    reuseExistingServer: !process.env.CI,
  },
});
