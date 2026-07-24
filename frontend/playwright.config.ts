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
    // Seed the seen-version so the What's New modal (it doubles as onboarding on
    // fresh profiles) doesn't block specs; whats-new specs opt out with a clean storageState.
    storageState: {
      cookies: [],
      origins: [{
        origin: 'http://localhost:5173',
        localStorage: [{
          name: 'gophercourier.settings',
          value: JSON.stringify({ lastSeenWhatsNewVersion: pkg.version }),
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
