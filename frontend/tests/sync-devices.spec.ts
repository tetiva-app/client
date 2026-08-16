import { test, expect, type Page } from '@playwright/test';

// Mock build: MockSyncService returns three fixed devices, one of them current.
test.describe('Sync devices', () => {
  async function connect(page: Page, scenario = '') {
    await page.goto('/' + scenario);
    await page.getByRole('button', { name: 'Sync', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await dialog.locator('#login-email').fill('test@example.com');
    await dialog.locator('#login-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Connect', exact: true }).click();
    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    return dialog;
  }

  test('lists the signed-in devices and marks the current one', async ({ page }) => {
    const dialog = await connect(page);

    await expect(dialog.getByTestId('sync-device')).toHaveCount(3);
    await expect(dialog.getByText('This device')).toHaveCount(1);
    await expect(dialog.getByRole('button', { name: 'Sign out', exact: true })).toHaveCount(2);
  });

  test('signing out one device removes its row', async ({ page }) => {
    const dialog = await connect(page);

    await dialog.getByRole('button', { name: 'Sign out', exact: true }).first().click();

    await expect(dialog.getByTestId('sync-device')).toHaveCount(2);
    await expect(dialog.getByText('This device')).toHaveCount(1);
  });

  test('labels a device by host, platform and version', async ({ page }) => {
    const dialog = await connect(page);

    await expect(dialog.getByTestId('sync-device').first())
      .toContainText('mbp.local · macOS · Tetiva 0.17.0');
  });

  test('keeps the label readable at the default modal width', async ({ page }) => {
    const dialog = await connect(page);

    const label = dialog.getByTestId('sync-device-label').first();
    await expect(label).toHaveAttribute('title', 'mbp.local · macOS · Tetiva 0.17.0');

    const clipped = await label.evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(clipped).toBeLessThanOrEqual(0);
  });

  test('keeps the connected content inside the dialog padding', async ({ page }) => {
    const dialog = await connect(page);

    const content = await dialog.evaluate((el) => ({
      overflow: el.scrollWidth - el.clientWidth,
      right: el.getBoundingClientRect().right - parseFloat(getComputedStyle(el).paddingRight),
    }));
    expect(content.overflow).toBeLessThanOrEqual(0);

    const rowRight = await dialog
      .getByTestId('sync-device')
      .last()
      .evaluate((el) => el.getBoundingClientRect().right);
    const buttonRight = await dialog
      .getByRole('button', { name: 'Disconnect' })
      .evaluate((el) => el.getBoundingClientRect().right);

    expect(rowRight).toBeLessThanOrEqual(content.right);
    expect(buttonRight).toBeLessThanOrEqual(content.right);
  });

  test('omits last active for a device that never made a call', async ({ page }) => {
    const dialog = await connect(page);

    const rows = dialog.getByTestId('sync-device');
    await expect(rows.nth(1)).toContainText('last active');
    await expect(rows.nth(2)).not.toContainText('last active');
  });

  test('a failed revoke keeps the rows and the sign-out-everywhere control', async ({ page }) => {
    const dialog = await connect(page, '?mock=revoke-error');

    await dialog.getByRole('button', { name: 'Sign out', exact: true }).first().click();

    await expect(dialog.getByText("Couldn't sign that device out")).toBeVisible();
    await expect(dialog.getByTestId('sync-device')).toHaveCount(3);
    await expect(dialog.getByRole('button', { name: 'Sign out everywhere' })).toBeVisible();
  });

  test('signing out everywhere keeps only this device', async ({ page }) => {
    const dialog = await connect(page);

    await dialog.getByRole('button', { name: 'Sign out everywhere' }).click();
    await dialog.getByRole('button', { name: 'Confirm' }).click();

    await expect(dialog.getByTestId('sync-device')).toHaveCount(1);
    await expect(dialog.getByText('This device')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Sign out everywhere' })).toHaveCount(0);
    await expect(dialog.getByText('No other devices')).toBeVisible();
  });
});
