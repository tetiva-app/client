import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page, url: string) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Response Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Response Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
  await page.locator('[aria-placeholder="Enter request URL"]').fill(url);
}

async function sendRequest(page: Page) {
  await page.locator('button', { hasText: 'Send' }).click();
}

test.describe('Response Viewer', () => {
  test('should show idle state before sending request', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com');

    await expect(page.getByText('to send a request')).toBeVisible();
  });

  test('should show 200 OK response with body', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');
    await sendRequest(page);

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('OK').first()).toBeVisible();

    await expect(page.getByText('142ms')).toBeVisible();

    await expect(page.locator('[data-response-body]')).toContainText('Alice');
    await expect(page.locator('[data-response-body]')).toContainText('Bob');
  });

  test('should show response headers tab', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');
    await sendRequest(page);

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });

    // Scope to response tabs area (inside success state)
    const responseTabs = page.locator('.border-b.border-border.px-3').last();
    await responseTabs.locator('button', { hasText: 'Headers' }).click();

    await expect(page.getByText('Content-Type')).toBeVisible();
    await expect(page.getByText('application/json')).toBeVisible();
    await expect(page.getByText('X-Request-Id')).toBeVisible();
    await expect(page.getByText('Cache-Control')).toBeVisible();
  });

  test('should show Tests tab with "No test results"', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');
    await sendRequest(page);

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });

    const responseTabs = page.locator('.border-b.border-border.px-3').last();
    await responseTabs.locator('button', { hasText: 'Tests' }).click();

    // mock does not return scriptResult
    await expect(page.getByText('No test results')).toBeVisible();
  });

  test('should show 404 response', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/404');
    await sendRequest(page);

    await expect(page.getByText('404').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('404 Not Found')).toBeVisible();

    await expect(page.locator('[data-response-body]')).toContainText('not found');
  });

  test('should show 500 response', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/500');
    await sendRequest(page);

    await expect(page.getByText('500').first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByText('Internal Server Error').first()).toBeVisible();

    await expect(page.locator('[data-response-body]')).toContainText('internal server error');
  });

  test('should show error state for connection errors', async ({ page }) => {
    await createAndOpenRequest(page, 'https://error.example.com');
    await sendRequest(page);

    await expect(page.getByText('Connection refused')).toBeVisible({ timeout: 5000 });
  });

  test('should show loading state with elapsed time', async ({ page }) => {
    await createAndOpenRequest(page, 'https://slow.example.com');
    await sendRequest(page);

    await expect(page.getByText(/\d+\.\ds/)).toBeVisible({ timeout: 2000 });

    const cancelBtn = page.locator('button', { hasText: 'Cancel' });
    await expect(cancelBtn.first()).toBeVisible();
  });

  test('should return to idle after cancelling slow request', async ({ page }) => {
    await createAndOpenRequest(page, 'https://slow.example.com');
    await sendRequest(page);

    const cancelBtn = page.locator('button', { hasText: 'Cancel' }).first();
    await expect(cancelBtn).toBeVisible({ timeout: 2000 });

    await cancelBtn.click();

    // idle state shows the ⌘ Enter hint
    await expect(page.getByText('⌘ Enter')).toBeVisible({ timeout: 3000 });
  });

  test('should show Body tab active by default in response', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');
    await sendRequest(page);

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });

    const responseTabs = page.locator('.border-b.border-border.px-3').last();
    const bodyTab = responseTabs.locator('button', { hasText: 'Body' });
    await expect(bodyTab).toHaveClass(/border-primary/);
  });

  test('should show headers count badge in response tabs', async ({ page }) => {
    await createAndOpenRequest(page, 'https://api.example.com/users');
    await sendRequest(page);

    await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });

    // mock returns 3 headers
    const responseTabs = page.locator('.border-b.border-border.px-3').last();
    await expect(responseTabs.locator('button', { hasText: 'Headers' })).toContainText('(3)');
  });
});
