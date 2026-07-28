import { defineConfig, devices } from '@playwright/test';
import pkg from './package.json' with { type: 'json' };

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    colorScheme: 'dark',
    // Seed the seen-version and the welcome flag so neither first-launch modal
    // blocks specs; the first-launch specs opt out with a clean storageState.
    storageState: {
      cookies: [],
      origins: [{
        origin: 'http://localhost:5173',
        localStorage: [{
          name: 'gophercourier.settings',
          value: JSON.stringify({
            lastSeenWhatsNewVersion: pkg.version,
            onboardingCompletedAt: '2026-01-01T00:00:00.000Z',
          }),
        }],
      }],
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
});
