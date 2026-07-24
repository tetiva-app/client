import { test, expect, type Page } from '@playwright/test';

// Mock build: MockSyncService accepts any credentials and echoes the server URL back.
test.describe('Sync connect modal', () => {
  async function openModal(page: Page) {
    await page.goto('/');
    await page.getByRole('button', { name: 'Sync', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    return dialog;
  }

  test('no server field by default, toggle reveals empty input', async ({ page }) => {
    const dialog = await openModal(page);

    await expect(dialog.getByRole('tab', { name: 'Login' })).toBeVisible();
    await expect(dialog.locator('#server-url')).toHaveCount(0);

    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await expect(dialog.locator('#server-url')).toBeVisible();
    await expect(dialog.locator('#server-url')).toHaveValue('');
  });

  test('connect without custom server shows the cloud label', async ({ page }) => {
    const dialog = await openModal(page);

    await dialog.locator('#login-email').fill('test@example.com');
    await dialog.locator('#login-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Connect', exact: true }).click();

    await expect(dialog.getByText('Tetiva Cloud')).toBeVisible();
    await expect(dialog.getByText('test@example.com')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
  });

  test('connect to a custom server shows the normalized raw address', async ({ page }) => {
    const dialog = await openModal(page);

    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await dialog.locator('#server-url').fill('https://sync.corp.local/');
    await dialog.locator('#login-email').fill('test@example.com');
    await dialog.locator('#login-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Connect', exact: true }).click();

    await expect(dialog.getByText('sync.corp.local:443')).toBeVisible();
    await expect(dialog.getByText('Tetiva Cloud')).toHaveCount(0);
  });
});
