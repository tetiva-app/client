import { test, expect } from '@playwright/test';

test.describe('Script Autocomplete', () => {
  // TODO(e2e): CM autocomplete tooltip timing
  test.skip('should show postman API autocompletion hints in scripts editor', async ({ page }) => {
    page.on('console', msg => console.log('BROWSER:', msg.text()));
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Autocomplete Tests');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Autocomplete Tests');
    await expect(collectionItem).toBeVisible();

    await collectionItem.click();
    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    
    dialog = page.getByRole('dialog');
    input = dialog.getByPlaceholder('Request name');
    await input.fill('Autocomplete Request');
    await input.press('Enter');

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

    const scriptsTab = page.locator('button', { hasText: 'Scripts' });
    await scriptsTab.click();

    // the async CodeEditor mounts lazily
    const editorContent = page.locator('.cm-content').first();
    await expect(editorContent).toBeVisible();

    await editorContent.click();
    
    // activateOnTyping fires the autocomplete as 'pm.' is typed
    await page.keyboard.type('pm.');
    await page.waitForTimeout(500);
    
    const tooltip = page.locator('.cm-tooltip-autocomplete');
    await expect(tooltip).toBeVisible();
    
    await expect(tooltip.locator('li', { hasText: 'environment' }).first()).toBeVisible();
    await expect(tooltip.locator('li', { hasText: 'request' }).first()).toBeVisible();

    await page.keyboard.type('e');
    await page.waitForTimeout(100);
    await page.keyboard.press('Enter');
    await page.waitForTimeout(200);

    await page.keyboard.type('.');
    await page.waitForTimeout(300);
    await expect(tooltip).toBeVisible();
    await expect(tooltip.locator('li', { hasText: 'get' }).first()).toBeVisible();

    await page.keyboard.press('Enter');
    await expect(tooltip).toBeHidden();

    const codeLine = page.locator('.cm-line').first();
    await expect(codeLine).toContainText('pm.environment.get');

    await editorContent.press('Meta+A');
    await page.keyboard.press('Backspace');

    const postBtn = page.locator('button', { hasText: 'Post-response' });
    await postBtn.click();
    await page.waitForTimeout(300);

    const postEditorContent = page.locator('.cm-content').last();
    await postEditorContent.click();

    await page.keyboard.type('pm.te');
    await page.waitForTimeout(300);
    await expect(tooltip).toBeVisible();
    await expect(tooltip.locator('li', { hasText: 'test' }).first()).toBeVisible();

    await page.keyboard.press('Enter');
    await page.waitForTimeout(200);

    const postText = await postEditorContent.textContent();
    expect(postText).toContain('pm.test');
    expect(postText).toContain('function() {');
  });
});
