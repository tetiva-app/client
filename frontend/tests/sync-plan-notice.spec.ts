import { test, expect, type Page } from '@playwright/test';

// Mock build: `?mock=parked` boots connected with 4 quota-parked changes,
// `?mock=plan-limit` boots with sync stopped over the member limit.
test.describe('Sync modal plan notice', () => {
  async function openConnected(page: Page, scenario: string) {
    await page.addInitScript(() => {
      (window as unknown as { __opened: string[] }).__opened = [];
      window.open = ((url: string) => {
        (window as unknown as { __opened: string[] }).__opened.push(url);
        return null;
      }) as typeof window.open;
    });
    await page.goto('/?mock=' + scenario);
    await page.getByRole('button', { name: 'Sync', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    return dialog;
  }

  test('names the cloud collection limit as the reason changes stay local', async ({ page }) => {
    const dialog = await openConnected(page, 'parked');

    const notice = dialog.getByTestId('sync-plan-notice');
    await expect(notice).toContainText('4 changes not synced — cloud collection limit reached on your plan.');
    await expect(notice).toContainText('They will sync automatically after an upgrade.');

    await notice.getByRole('button', { name: 'See plans' }).click();
    const opened = await page.evaluate(() => (window as unknown as { __opened: string[] }).__opened);
    expect(opened[0]).toContain('tetiva.app/pricing');
  });

  test('points at the member limit when sync is paused for the whole team', async ({ page }) => {
    const dialog = await openConnected(page, 'plan-limit');

    const notice = dialog.getByTestId('sync-plan-notice');
    await expect(notice).toContainText("Sync paused — the team exceeds its plan's member limit.");
    await expect(notice).toContainText('Ask the owner to update the plan or remove members.');
    await expect(notice.getByRole('button', { name: 'See plans' })).toBeVisible();
  });

  test('stays quiet while everything syncs', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Sync', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await dialog.locator('#login-email').fill('test@example.com');
    await dialog.locator('#login-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Connect', exact: true }).click();

    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(dialog.getByTestId('sync-plan-notice')).toHaveCount(0);
  });
});
