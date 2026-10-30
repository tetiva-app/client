import { test, expect } from '@playwright/test';

const NOTICE = 'Your sync session has ended. Sign in again — changes on this device are kept.';
const MODAL_NOTICE = 'Your session has ended. Sign in again — changes on this device are kept and will sync after you sign in.';

// Mock build: `?mock=reauth-required` boots with the startup cleanup already done —
// no session, the flag still set.
test.describe('Ended session notice', () => {
  test('asks once for a new sign-in and opens the modal', async ({ page }) => {
    await page.goto('/?mock=reauth-required');

    const toast = page.getByText(NOTICE);
    await expect(toast).toHaveCount(1);

    // Sticky: it survives the 4s window a plain info toast would close in.
    await page.waitForTimeout(4500);
    await expect(toast).toHaveCount(1);

    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId('sync-reauth-notice')).toContainText(MODAL_NOTICE);
    await expect(dialog.getByTestId('signin-browser')).toBeVisible();
    await expect(dialog.getByText('Connected to')).toHaveCount(0);
  });

  test('signing out instead drops the notice and the toast', async ({ page }) => {
    await page.goto('/?mock=reauth-required');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    const dialog = page.getByRole('dialog');

    await dialog.getByTestId('sync-reauth-sign-out').click();

    await expect(dialog.getByTestId('sync-reauth-notice')).toHaveCount(0);
    await expect(dialog.getByTestId('signin-browser')).toBeVisible();
    await expect(page.getByText(NOTICE)).toHaveCount(0);
  });

  test('stays quiet when the session is intact', async ({ page }) => {
    await page.goto('/');

    await page.getByRole('button', { name: 'Sync', exact: true }).waitFor();
    await expect(page.getByText(NOTICE)).toHaveCount(0);
  });
});
