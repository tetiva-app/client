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
