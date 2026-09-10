import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Params Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Params Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

test.describe('Params Editor', () => {
  test('should show Params tab as default active tab', async ({ page }) => {
    await createAndOpenRequest(page);

    // scoped to the request editor tabs area
    const editorTabs = page.locator('.border-b.border-border.px-3');
    const paramsTab = editorTabs.locator('button', { hasText: 'Params' });
    await expect(paramsTab).toHaveClass(/border-primary/);
  });

  test('should add a parameter via key-value inputs', async ({ page }) => {
    await createAndOpenRequest(page);

    const newKeyInput = page.getByPlaceholder('Param name');
    const newValueInput = page.getByPlaceholder('Value');

    await newKeyInput.fill('page');
    await newValueInput.fill('1');
    await newValueInput.press('Enter');

    const checkbox = page.locator('input[type="checkbox"]').first();
    await expect(checkbox).toBeVisible();
    await expect(checkbox).toBeChecked();

    const urlInput = page.locator('[aria-placeholder="Enter request URL"]');
    await expect(urlInput).toContainText(/[?&]page=1/);
  });

  test('should add multiple params and update URL', async ({ page }) => {
    await createAndOpenRequest(page);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com');

    const newKeyInput = page.getByPlaceholder('Param name');
    const newValueInput = page.getByPlaceholder('Value');

    await newKeyInput.fill('page');
    await newValueInput.fill('1');
    await newValueInput.press('Enter');

    await newKeyInput.fill('limit');
    await newValueInput.fill('20');
    await newValueInput.press('Enter');

    const urlInput = page.locator('[aria-placeholder="Enter request URL"]');
    await expect(urlInput).toContainText(/page=1/);
    await expect(urlInput).toContainText(/limit=20/);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const paramsTab = editorTabs.locator('button', { hasText: 'Params' });
    await expect(paramsTab).toContainText('(2)');
  });

  test('should toggle param checkbox to disable/enable', async ({ page }) => {
    await createAndOpenRequest(page);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com');

    await page.getByPlaceholder('Param name').fill('token');
    await page.getByPlaceholder('Value').fill('abc123');
    await page.getByPlaceholder('Value').press('Enter');

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).toContainText(/token=abc123/);

    const checkbox = page.locator('input[type="checkbox"]').first();
    await checkbox.uncheck();

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).not.toContainText(/token=abc123/);

    const row = checkbox.locator('xpath=ancestor::div[contains(@class,"grid-cols")]');
    await expect(row).toHaveClass(/opacity-40/);

    await checkbox.check();
    await expect(page.locator('[aria-placeholder="Enter request URL"]')).toContainText(/token=abc123/);
  });

  test('should delete a param row', async ({ page }) => {
    await createAndOpenRequest(page);
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com');

    await page.getByPlaceholder('Param name').fill('search');
    await page.getByPlaceholder('Value').fill('hello');
    await page.getByPlaceholder('Value').press('Enter');

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).toContainText(/search=hello/);

    // delete button only appears on hover
    const row = page.locator('input[type="checkbox"]').first().locator('xpath=ancestor::div[contains(@class,"grid-cols")]');
    await row.hover();

    const deleteBtn = row.locator('button').last();
    await deleteBtn.click();

    await expect(page.locator('[aria-placeholder="Enter request URL"]')).not.toContainText(/search=hello/);
  });

  test('should parse existing URL params into rows', async ({ page }) => {
    await createAndOpenRequest(page);

    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com?foo=bar&baz=qux');

    const checkboxes = page.locator('input[type="checkbox"]');
    await expect(checkboxes).toHaveCount(2);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const paramsTab = editorTabs.locator('button', { hasText: 'Params' });
    await expect(paramsTab).toContainText('(2)');
  });

  test('should add param via Enter key in key field', async ({ page }) => {
    await createAndOpenRequest(page);

    const newKeyInput = page.getByPlaceholder('Param name');
    const newValueInput = page.getByPlaceholder('Value');

    await newKeyInput.fill('key');
    await newValueInput.fill('value');
    await newKeyInput.press('Enter');

    const checkbox = page.locator('input[type="checkbox"]').first();
    await expect(checkbox).toBeVisible();
  });

  test('should not add param with empty key', async ({ page }) => {
    await createAndOpenRequest(page);

    const kvEditor = page.locator('.grid-cols-\\[32px_1fr_1fr_32px\\]').last();
    const addBtn = kvEditor.locator('button').first();
    await expect(addBtn).toBeDisabled();

    await page.getByPlaceholder('Value').fill('some-value');
    await page.getByPlaceholder('Value').press('Enter');

    const checkboxes = page.locator('input[type="checkbox"]');
    await expect(checkboxes).toHaveCount(0);
  });
});
