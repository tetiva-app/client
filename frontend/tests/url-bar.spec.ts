import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page, url = '') {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Test Collection');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const collectionItem = sidebar.getByText('Test Collection');
  await expect(collectionItem).toBeVisible();
  await collectionItem.click();

  await collectionItem.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Request');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

  if (url) {
    await page.locator('[aria-placeholder="Enter request URL"]').fill(url);
  }
}

test.describe('URL Bar', () => {
  test('should show method selector with GET as default', async ({ page }) => {
    await createAndOpenRequest(page);

    const methodBtn = page.locator('button[role="combobox"]');
    await expect(methodBtn).toContainText('GET');
  });

  test('should open method dropdown and select different methods', async ({ page }) => {
    await createAndOpenRequest(page);

    const methodBtn = page.locator('button[role="combobox"]');

    await methodBtn.click();
    const listbox = page.locator('[role="listbox"]');
    await expect(listbox).toBeVisible();

    const allMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD'];
    for (const method of allMethods) {
      await expect(listbox.locator(`[role="option"]`, { hasText: method })).toBeVisible();
    }

    await listbox.locator('[role="option"]', { hasText: 'POST' }).click();
    await expect(listbox).not.toBeVisible();
    await expect(methodBtn).toContainText('POST');

    await methodBtn.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'DELETE' }).click();
    await expect(methodBtn).toContainText('DELETE');
  });

  test('should navigate method dropdown with keyboard', async ({ page }) => {
    await createAndOpenRequest(page);

    const methodBtn = page.locator('button[role="combobox"]');
    await methodBtn.click();
    await expect(page.locator('[role="listbox"]')).toBeVisible();

    await methodBtn.press('ArrowDown'); // from GET to POST
    await methodBtn.press('Enter');

    await expect(page.locator('[role="listbox"]')).not.toBeVisible();
    await expect(methodBtn).toContainText('POST');

    await methodBtn.click();
    await expect(page.locator('[role="listbox"]')).toBeVisible();
    await methodBtn.press('Escape');
    await expect(page.locator('[role="listbox"]')).not.toBeVisible();
    await expect(methodBtn).toContainText('POST');
  });

  test('should allow typing a URL', async ({ page }) => {
    await createAndOpenRequest(page);

    const urlInput = page.locator('[aria-placeholder="Enter request URL"]');
    await urlInput.fill('https://api.example.com/users');
    await expect(urlInput).toContainText('https://api.example.com/users');
  });

  test('should send request and show response on Send button click', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');

    const sendBtn = page.locator('button', { hasText: 'Send' });
    await sendBtn.click();

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('OK').first()).toBeVisible();
    await expect(page.locator('[data-response-body]')).toContainText('Alice');
  });

  test('should show loading state and Cancel button for slow requests', async ({ page }) => {
    await createAndOpenRequest(page, 'https://slow.example.com');

    await page.locator('button', { hasText: 'Send' }).click();

    const cancelBtn = page.locator('button', { hasText: 'Cancel' }).first();
    await expect(cancelBtn).toBeVisible({ timeout: 2000 });

    await expect(page.getByText(/\d+\.\ds/)).toBeVisible();

    await cancelBtn.click();

    // idle state shows the ⌘ Enter hint
    await expect(page.getByText('⌘ Enter')).toBeVisible({ timeout: 3000 });
  });

  test('should show Copy as cURL option in send dropdown', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/test');

    const sendArea = page.locator('.relative.flex.items-center');
    // The dropdown trigger is the last small button in the send area
    const dropdownArrow = sendArea.locator('button').last();
    await dropdownArrow.click();

    await expect(page.getByText('Copy as cURL')).toBeVisible();
  });

  test('should show error state for error URLs', async ({ page }) => {
    await createAndOpenRequest(page, 'https://error.example.com');

    await page.locator('button', { hasText: 'Send' }).click();

    await expect(page.getByText('Connection refused')).toBeVisible({ timeout: 5000 });
  });

  test('should show 404 response with correct status', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/404');

    await page.locator('button', { hasText: 'Send' }).click();

    await expect(page.getByText('404').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('Not Found').first()).toBeVisible();
  });

  test('should show 500 response with correct status', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/500');

    await page.locator('button', { hasText: 'Send' }).click();

    await expect(page.getByText('500').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('Internal Server Error').first()).toBeVisible();
  });
});
