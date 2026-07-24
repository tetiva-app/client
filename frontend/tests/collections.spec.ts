import { test, expect } from '@playwright/test';

test.describe('Collection and Context Menu interactions', () => {
  test('should handle sub-collections, rename, and delete via context menu', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Root Collection');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const rootItem = sidebar.getByText('Root Collection');
    await expect(rootItem).toBeVisible();

    await rootItem.click({ button: 'right' });
    const contextMenu = page.getByRole('menu');
    await expect(contextMenu).toBeVisible();

    await contextMenu.getByText('New Sub-Collection').click();
    
    dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('New Sub-Collection');
    input = dialog.getByPlaceholder('Collection name');
    await input.fill('Child Collection');
    await input.press('Enter');

    // parent doesn't auto-expand on child creation
    await rootItem.click();
    
    const childItem = sidebar.getByText('Child Collection');
    await expect(childItem).toBeVisible();

    await childItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('Rename').click();
    
    dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('Rename Collection');
    input = dialog.locator('input');
    await expect(input).toHaveValue('Child Collection');
    await input.fill('Renamed Child');
    await input.press('Enter');

    await expect(sidebar.getByText('Renamed Child')).toBeVisible();
    await expect(sidebar.getByText('Child Collection')).not.toBeVisible();

    await rootItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('Delete').click();

    dialog = page.getByRole('alertdialog');
    await expect(dialog).toContainText('Delete "Root Collection" and all its contents?');
    
    await dialog.getByRole('button', { name: 'Delete' }).click();

    await expect(rootItem).not.toBeVisible();
    await expect(sidebar.getByText('Renamed Child')).not.toBeVisible();
    await expect(sidebar).toContainText('No collections yet. Click + to create one.');
  });
});
