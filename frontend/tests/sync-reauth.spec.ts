import { test, expect } from '@playwright/test';

const NOTICE = 'Sign-in has changed with this update. Please sign in again.';

// Mock build: `?mock=reauth-required` boots with the §2.7 cleanup already done —
// no session, the flag still set.
test.describe('Forced re-login notice', () => {
  test('asks once for a new sign-in and opens the modal', async ({ page }) => {
    await page.goto('/?mock=reauth-required');

    const toast = page.getByText(NOTICE);
    await expect(toast).toHaveCount(1);

    // Sticky: it survives the 4s window a plain info toast would close in.
    await page.waitForTimeout(4500);
    await expect(toast).toHaveCount(1);

    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
  });

  test('stays quiet when the session is intact', async ({ page }) => {
    await page.goto('/');

    await page.getByRole('button', { name: 'Sync', exact: true }).waitFor();
    await expect(page.getByText(NOTICE)).toHaveCount(0);
  });
});
