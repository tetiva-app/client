import { test, expect } from '@playwright/test';
import pkg from '../package.json' with { type: 'json' };

const KEY = 'gophercourier.settings';

// Opt out of the global seeded storageState — these tests drive the seen-version
// themselves (fresh profile must start with an empty store).
test.use({ storageState: { cookies: [], origins: [] } });

test.describe("What's New & update badge", () => {
  test('fresh profile: the welcome takes over and both flags land on dismissal', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByTestId('onboarding-modal')).toBeVisible();
    await expect(page.getByTestId('whats-new-modal')).toHaveCount(0);

    // Quitting before the choice is made must not consume the welcome.
    await page.reload();
    await expect(page.getByTestId('onboarding-modal')).toBeVisible();

    await page.getByTestId('onboarding-choice-local').click();
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);
    const stored = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? '{}'), KEY);
    expect(stored.lastSeenWhatsNewVersion).toBe(pkg.version);
    expect(stored.onboardingCompletedAt).toBeTruthy();

    await page.reload();
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);
    await expect(page.getByTestId('whats-new-modal')).toHaveCount(0);
  });

  test('upgrade: modal shows once and records the version', async ({ page }) => {
    await page.addInitScript(([key]) => {
      // Re-runs on reload — never clobber what the app has written since.
      if (!localStorage.getItem(key)) {
        localStorage.setItem(key, JSON.stringify({ lastSeenWhatsNewVersion: '0.0.1' }));
      }
    }, [KEY]);
    await page.goto('/');
    await expect(page.getByTestId('whats-new-modal')).toBeVisible();
    // A seen version means the install is not new — the welcome stays away.
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);
    await page.getByRole('button', { name: /Got it|Понятно/ }).click();
    await expect(page.getByTestId('whats-new-modal')).toHaveCount(0);
    const stored = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? '{}'), KEY);
    expect(stored.lastSeenWhatsNewVersion).toBe(pkg.version);
    await page.reload();
    await expect(page.getByTestId('whats-new-modal')).toHaveCount(0);
  });

  test('badge renders when a newer update is stored', async ({ page }) => {
    await page.addInitScript(([key, v]) => {
      if (!localStorage.getItem(key)) {
        localStorage.setItem(key, JSON.stringify({
          lastSeenWhatsNewVersion: v,
          availableUpdate: { version: '99.0.0', url: 'https://example.com' },
        }));
      }
    }, [KEY, pkg.version]);
    await page.goto('/');
    await expect(page.getByTestId('update-badge')).toBeVisible();
  });
});
