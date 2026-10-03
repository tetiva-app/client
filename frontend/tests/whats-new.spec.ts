import { test, expect, type Page } from '@playwright/test';
import pkg from '../package.json' with { type: 'json' };

const KEY = 'gophercourier.settings';

// Opt out of the global seeded storageState — these tests drive the seen-version
// themselves (fresh profile must start with an empty store).
test.use({ storageState: { cookies: [], origins: [] } });

async function seedUpgrade(page: Page) {
  await page.addInitScript(([key]) => {
    // Re-runs on reload — never clobber what the app has written since.
    if (!localStorage.getItem(key)) {
      localStorage.setItem(key, JSON.stringify({ lastSeenWhatsNewVersion: '0.0.1' }));
    }
  }, [KEY]);
}

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
    await seedUpgrade(page);
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

  const viewports = [
    { width: 960, height: 640, listOverflows: true },
    { width: 1280, height: 800, listOverflows: false },
  ];
  for (const locale of ['en-US', 'ru-RU']) {
    test.describe(`upgrade modal in ${locale}`, () => {
      test.use({ locale });

      for (const size of viewports) {
        test(`fits a ${size.width}x${size.height} window with every note reachable`, async ({ page }) => {
          await page.setViewportSize(size);
          await seedUpgrade(page);
          await page.goto('/');
          const dialog = page.getByRole('dialog');
          const list = page.getByTestId('whats-new-modal');
          await expect(list).toBeVisible();
          const title = dialog.getByRole('heading', { name: /What's New|Что нового/ });
          const gotIt = dialog.getByRole('button', { name: /Got it|Понятно/ });
          const close = dialog.getByRole('button', { name: locale === 'ru-RU' ? 'Закрыть' : 'Close', exact: true });
          await expect(title).toBeInViewport({ ratio: 1 });
          await expect(gotIt).toBeInViewport({ ratio: 1 });
          await expect(close).toBeInViewport({ ratio: 1 });
          if (size.listOverflows) {
            expect(await list.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
          }

          const lastItem = list.locator('li').last();
          await lastItem.scrollIntoViewIfNeeded();
          await expect(lastItem).toBeInViewport({ ratio: 1 });
          await expect(title).toBeInViewport({ ratio: 1 });
          await expect(gotIt).toBeInViewport({ ratio: 1 });
        });
      }
    });
  }

  test('badge renders when an update is ready', async ({ page }) => {
    await page.addInitScript(([key, v]) => {
      if (!localStorage.getItem(key)) {
        localStorage.setItem(key, JSON.stringify({ lastSeenWhatsNewVersion: v }));
      }
    }, [KEY, pkg.version]);
    await page.goto('/?mock=update-ready');
    await expect(page.getByTestId('update-badge')).toBeVisible();
  });
});
