import { test, expect, type Page } from '@playwright/test';

async function createCollectionWithRequests(page: Page, requestNames: string[]) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Shortcuts Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Shortcuts Test');
  await col.click();

  for (const name of requestNames) {
    await col.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    const reqDialog = page.getByRole('dialog');
    await reqDialog.getByPlaceholder('Request name').fill(name);
    await reqDialog.getByPlaceholder('Request name').press('Enter');
  }

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

test.describe('Keyboard Shortcuts', () => {
  test('should send request with Cmd+Enter', async ({ page }) => {
    await createCollectionWithRequests(page, ['Send Test']);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');

    await page.keyboard.press('Meta+Enter');

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
  });

  test('should switch tabs with Cmd+1, Cmd+2, Cmd+3', async ({ page }) => {
    await createCollectionWithRequests(page, ['Tab One', 'Tab Two', 'Tab Three']);

    const tabBar = page.locator('.flex.items-center.h-9.border-b');

    // last created tab is the active one
    const tab3Container = tabBar.getByText('Tab Three').locator('..');
    await expect(tab3Container).toHaveClass(/border-b-primary/);

    await page.keyboard.press('Meta+1');
    const tab1Container = tabBar.getByText('Tab One').locator('..');
    await expect(tab1Container).toHaveClass(/border-b-primary/);

    await page.keyboard.press('Meta+2');
    const tab2Container = tabBar.getByText('Tab Two').locator('..');
    await expect(tab2Container).toHaveClass(/border-b-primary/);

    await page.keyboard.press('Meta+3');
    await expect(tab3Container).toHaveClass(/border-b-primary/);
  });

  test('should save request with Cmd+S (dirty indicator clears)', async ({ page }) => {
    await createCollectionWithRequests(page, ['Save Test']);

    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/modified');

    const dirtyDot = page.locator('span[title="Unsaved changes"]');
    await expect(dirtyDot.first()).toBeVisible();

    await page.keyboard.press('Meta+s');

    // Shorter than AUTOSAVE_DELAY_MS: the dot must clear from the shortcut, not the timer.
    await expect(dirtyDot).toHaveCount(0, { timeout: 1000 });
  });

  test('Cmd+S saves on a Cyrillic layout (key ы, code KeyS)', async ({ page }) => {
    await createCollectionWithRequests(page, ['Layout Test']);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/ru');
    const dirtyDot = page.locator('span[title="Unsaved changes"]');
    await expect(dirtyDot.first()).toBeVisible();
    await page.evaluate(() => window.dispatchEvent(new KeyboardEvent('keydown',
      { key: 'ы', code: 'KeyS', metaKey: true, bubbles: true, cancelable: true })));
    // Shorter than AUTOSAVE_DELAY_MS: the dot must clear from the shortcut, not the timer.
    await expect(dirtyDot).toHaveCount(0, { timeout: 1000 });
  });

  test('Cmd+Enter inside a dialog does not send the underlying request', async ({ page }) => {
    await createCollectionWithRequests(page, ['Guard Test']);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Shortcuts Test').click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    const reqDialog = page.getByRole('dialog');
    const nameInput = reqDialog.getByPlaceholder('Request name');
    await nameInput.focus();

    // Dialog's own Enter handler is a no-op here: the name field is empty
    await nameInput.press('Meta+Enter');
    await page.waitForTimeout(1000);
    await expect(page.getByText('200')).toHaveCount(0);
  });

  test('should handle Cmd+Enter for slow request (shows loading)', async ({ page }) => {
    await createCollectionWithRequests(page, ['Slow Test']);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://slow.example.com');

    await page.keyboard.press('Meta+Enter');

    const cancelBtn = page.locator('button', { hasText: 'Cancel' }).first();
    await expect(cancelBtn).toBeVisible({ timeout: 2000 });
  });
});
