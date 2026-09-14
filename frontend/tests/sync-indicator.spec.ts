import { test, expect } from '@playwright/test';

// Mock build: `?mock=parked` boots MockSyncService connected with 4 quota-parked changes.
test.describe('Sync status indicator', () => {
  test('warns in amber while the plan limit keeps changes local', async ({ page }) => {
    await page.goto('/?mock=parked');

    const indicator = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(indicator.getByTestId('sync-parked-alert')).toBeVisible();
    await expect(indicator).toContainText('4');

    await indicator.hover();
    await expect(page.getByText('4 changes not synced — plan limit')).toBeVisible();
  });

  test('names oversized items beside the plan limit in the tooltip', async ({ page }) => {
    await page.goto('/?mock=too-large');

    const indicator = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(indicator.getByTestId('sync-parked-alert')).toBeVisible();

    await indicator.hover();
    await expect(page.getByText('1 change not synced — plan limit · 2 items too large for the server')).toBeVisible();
  });

  test('stays neutral without parked changes', async ({ page }) => {
    await page.goto('/');

    const indicator = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(indicator.getByTestId('sync-parked-alert')).toHaveCount(0);
  });
});
