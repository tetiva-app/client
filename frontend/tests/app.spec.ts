import { test, expect } from '@playwright/test';

test.describe('Tetiva App', () => {
  test('should load the app and show empty state', async ({ page }) => {
    await page.goto('/');

    await expect(page).toHaveTitle(/Tetiva/);

    const mainContent = page.locator('main');
    await expect(mainContent).toContainText('Select a request from the sidebar to get started.');

    const sidebar = page.locator('aside');
    await expect(sidebar).toContainText('No collections yet. Click + to create one.');
  });

  test('should allow creating a new collection', async ({ page }) => {
    await page.goto('/');

    const newCollectionBtn = page.getByTitle('New Collection');
    await newCollectionBtn.click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText('New Collection');

    const input = dialog.getByPlaceholder('Collection name');
    await input.fill('My Test Collection');
    await input.press('Enter');

    await expect(dialog).not.toBeVisible();

    const sidebar = page.locator('aside');
    await expect(sidebar).toContainText('My Test Collection');
    
    await expect(sidebar).not.toContainText('No collections yet. Click + to create one.');
  });

  test('should disable create button if collection name is empty', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const createBtn = dialog.getByRole('button', { name: 'Create' });
    await expect(createBtn).toBeDisabled();

    const input = dialog.getByPlaceholder('Collection name');
    await input.fill('   ');
    await expect(createBtn).toBeDisabled();

    await input.fill('Valid Name');
    await expect(createBtn).toBeEnabled();
  });
});

test('documentation buttons use the app tooltip, not the native one', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByTestId('settings-nav-mcp').click();

  const help = page.getByRole('button', { name: 'Documentation: mcp server' });
  await expect(help).toBeVisible();
  await expect(help).not.toHaveAttribute('title', /./);

  await help.hover();
  // Reka renders the visible bubble without role=tooltip; the slot is the handle.
  await expect(page.locator('[data-slot="tooltip-content"]')).toContainText('Documentation: mcp server');
});

test('a dialog opens without its help tooltip, which still shows on keyboard focus', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Sync', exact: true }).click();

  const help = page.getByRole('dialog').getByRole('button', { name: 'Documentation: sync' });
  const tooltip = page.locator('[data-slot="tooltip-content"]', { hasText: 'Documentation: sync' });
  await expect(help).toBeFocused();
  await expect(tooltip).toHaveCount(0);

  await page.keyboard.press('Tab');
  await page.keyboard.press('Shift+Tab');
  await expect(help).toBeFocused();
  await expect(tooltip).toBeVisible();
});

test('closing a dialog with the mouse does not pop the tooltip of the rail button that opened it', async ({ page }) => {
  await page.goto('/');
  const settings = page.getByRole('button', { name: 'Settings' });
  await settings.click();
  await expect(page.getByTestId('settings-dialog')).toBeVisible();

  await page.getByTestId('settings-dialog').getByRole('button', { name: 'Close' }).click();
  await expect(page.getByTestId('settings-dialog')).toHaveCount(0);
  await expect(settings).toBeFocused();
  await expect(page.locator('[data-slot="tooltip-content"]', { hasText: 'Settings' })).toHaveCount(0);
});
