import { test, expect } from '@playwright/test';

test.describe('Request Scripts', () => {
  test('should allow switching between pre-request and post-response script tabs and save content', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Script Tests');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Script Tests');
    await expect(collectionItem).toBeVisible();

    await collectionItem.click();
    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    
    dialog = page.getByRole('dialog');
    input = dialog.getByPlaceholder('Request name');
    await input.fill('Script Request');
    await input.press('Enter');

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

    const scriptsTab = page.locator('button', { hasText: 'Scripts' });
    await scriptsTab.click();

    const preBtn = page.locator('button', { hasText: 'Pre-request' });
    const postBtn = page.locator('button', { hasText: 'Post-response' });

    await expect(preBtn).toBeVisible();
    await expect(postBtn).toBeVisible();

    // Pre-request is selected by default
    await expect(preBtn).toHaveClass(/bg-primary\/10/);

    const editorContainers = page.locator('.cm-content:not([aria-placeholder*="request URL"])');
    // Pre-request is the first one since it's mounted first and v-show=true
    await editorContainers.first().click();
    await page.keyboard.type('console.log("pre-request");');

    await postBtn.click();
    await expect(postBtn).toHaveClass(/bg-primary\/10/);

    await editorContainers.last().click();
    await page.keyboard.type('console.log("post-response");');

    await expect(scriptsTab).toContainText('(2)');
  });
});
