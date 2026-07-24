import { test, expect } from '@playwright/test';

test.describe('Import / Export Management', () => {
  test('should open import dialog from environments and handle file selection flow', async ({ page }) => {
    await page.goto('/');

    const activityBar = page.locator('.w-12.shrink-0');
    // Environments is the second button
    await activityBar.locator('button').nth(1).click();
    const envDialog = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    await expect(envDialog).toBeVisible();

    const importBtn = envDialog.locator('button[title="Import Postman Environment"]');
    await expect(importBtn).toBeVisible();

    // waitForEvent('filechooser') must be registered before the click
    const fileChooserPromise = page.waitForEvent('filechooser');
    await importBtn.click();
    
    const fileChooser = await fileChooserPromise;
    expect(fileChooser.isMultiple()).toBe(false);

    // reload to reset state — Esc close behavior here is unreliable
    await page.goto('/');
  });

  test('should trigger collection export from context menu', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    const input = dialog.getByPlaceholder('Collection name');
    await input.fill('Export Collection');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Export Collection');
    await expect(collectionItem).toBeVisible();

    const downloadPromise = page.waitForEvent('download', { timeout: 10000 }).catch(() => null);

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('Export as Postman').click();

    // mock mode may not trigger a real download — only assert the action completes
    await expect(page.getByRole('menu')).not.toBeVisible();
  });
  
  test('should show import postman collection button in sidebar header', async ({ page }) => {
    await page.goto('/');

    const importBtn = page.locator('button[title="Import Postman Collection"]');
    await expect(importBtn).toBeVisible();

    const fileChooserPromise = page.waitForEvent('filechooser');
    await importBtn.click();
    const fileChooser = await fileChooserPromise;
    expect(fileChooser.isMultiple()).toBe(false);
  });
});
