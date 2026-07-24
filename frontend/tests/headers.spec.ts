import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Headers Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Headers Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

  const editorTabs = page.locator('.border-b.border-border.px-3');
  await editorTabs.locator('button', { hasText: 'Headers' }).click();
}

test.describe('Headers Editor', () => {
  test('should switch to Headers tab', async ({ page }) => {
    await createAndOpenRequest(page);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const headersTab = editorTabs.locator('button', { hasText: 'Headers' });
    await expect(headersTab).toHaveClass(/border-primary/);

    await expect(page.getByPlaceholder('Header name')).toBeVisible();
  });

  test('should add a header via key-value inputs', async ({ page }) => {
    await createAndOpenRequest(page);

    await page.getByPlaceholder('Header name').fill('Content-Type');
    await page.getByPlaceholder('Value').fill('application/json');
    await page.getByPlaceholder('Value').press('Enter');

    const checkbox = page.locator('input[type="checkbox"]').first();
    await expect(checkbox).toBeVisible();
    await expect(checkbox).toBeChecked();

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const headersTab = editorTabs.locator('button', { hasText: 'Headers' });
    await expect(headersTab).toContainText('(1)');
  });

  test('should add multiple headers', async ({ page }) => {
    await createAndOpenRequest(page);

    const keyInput = page.getByPlaceholder('Header name');
    const valueInput = page.getByPlaceholder('Value');

    await keyInput.fill('Authorization');
    await valueInput.fill('Bearer token123');
    await valueInput.press('Enter');

    await keyInput.fill('Accept');
    await valueInput.fill('application/json');
    await valueInput.press('Enter');

    await expect(page.locator('input[type="checkbox"]')).toHaveCount(2);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    await expect(editorTabs.locator('button', { hasText: 'Headers' })).toContainText('(2)');
  });

  test('should toggle header enabled/disabled', async ({ page }) => {
    await createAndOpenRequest(page);

    await page.getByPlaceholder('Header name').fill('X-Custom');
    await page.getByPlaceholder('Value').fill('test');
    await page.getByPlaceholder('Value').press('Enter');

    const checkbox = page.locator('input[type="checkbox"]').first();
    await checkbox.uncheck();

    const row = checkbox.locator('xpath=ancestor::div[contains(@class,"grid-cols")]');
    await expect(row).toHaveClass(/opacity-40/);

    await checkbox.check();
    await expect(row).not.toHaveClass(/opacity-40/);
  });

  test('should delete a header row', async ({ page }) => {
    await createAndOpenRequest(page);

    await page.getByPlaceholder('Header name').fill('X-Delete-Me');
    await page.getByPlaceholder('Value').fill('temp');
    await page.getByPlaceholder('Value').press('Enter');

    await expect(page.locator('input[type="checkbox"]')).toHaveCount(1);

    // Hover to reveal delete button
    const row = page.locator('input[type="checkbox"]').first().locator('xpath=ancestor::div[contains(@class,"grid-cols")]');
    await row.hover();
    await row.locator('button').last().click();

    await expect(page.locator('input[type="checkbox"]')).toHaveCount(0);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    await expect(editorTabs.locator('button', { hasText: 'Headers' })).not.toContainText('(');
  });

  test('should not add header with empty key', async ({ page }) => {
    await createAndOpenRequest(page);

    await page.getByPlaceholder('Value').fill('some-value');
    await page.getByPlaceholder('Value').press('Enter');

    await expect(page.locator('input[type="checkbox"]')).toHaveCount(0);
  });
});
