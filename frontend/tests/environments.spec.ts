import { test, expect } from '@playwright/test';

test.describe('Environments Management', () => {
  test('should create, edit, and delete environments and variables', async ({ page }) => {
    await page.goto('/');

    const activityBar = page.locator('.w-12.shrink-0');
    // Environments is the second button
    await activityBar.locator('button').nth(1).click();

    const dialog = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    await expect(dialog).toBeVisible();

    const envInput = dialog.getByPlaceholder('New environment');
    await envInput.fill('Production');
    await envInput.press('Enter');

    const envList = dialog.locator('.border-r.border-border');
    await expect(envList).toContainText('Production');

    await envInput.fill('Staging');
    await envInput.press('Enter');
    await expect(envList).toContainText('Staging');

    const stagingBtn = envList.locator('button', { hasText: 'Staging' });
    const checkBtn = stagingBtn.locator('button[title="Set as active"]');
    await checkBtn.click();
    await expect(stagingBtn.locator('button[title="Active environment"]')).toBeVisible();

    // newly created env is selected by default
    const rightPanel = dialog.locator('.flex-1.flex.flex-col').last();
    await expect(rightPanel).toContainText('Variables: Staging');

    const varNameInput = rightPanel.getByPlaceholder('Variable name');
    const varValueInput = rightPanel.getByPlaceholder('Value');

    await varNameInput.fill('API_URL');
    await varValueInput.fill('https://staging.api.com');
    await varValueInput.press('Enter');

    const varRow = rightPanel.locator('input[value="API_URL"]');
    await expect(varRow).toBeVisible();

    await varNameInput.fill('API_KEY');
    await varValueInput.fill('super_secret_token');
    
    const addRow = rightPanel.locator('.grid').last();
    const lockBtn = addRow.locator('button[title="Not secret (click to set)"]');
    await lockBtn.click();
    await addRow.locator('button .lucide-plus').click();

    const secretRowInput = rightPanel.locator('input[type="password"]');
    await expect(secretRowInput).toBeVisible();
    await expect(secretRowInput).toHaveValue('••••••••');

    const prodBtn = envList.locator('button', { hasText: 'Production' });
    await prodBtn.click({ button: 'right' });
    const contextMenu = page.getByRole('menu');
    await contextMenu.getByText('Delete').click();

    const confirmDialog = page.getByRole('alertdialog');
    await confirmDialog.getByRole('button', { name: 'Delete' }).click();

    await expect(envList).not.toContainText('Production');
    await expect(envList).toContainText('Staging');
  });
});
