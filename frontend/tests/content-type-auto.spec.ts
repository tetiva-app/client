import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('CT Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('CT Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

async function selectBodyType(page: Page, label: string) {
  const editorTabs = page.locator('.border-b.border-border.px-3');
  await editorTabs.locator('button', { hasText: 'Body' }).click();
  const selector = page.locator('button[role="combobox"]').last();
  await selector.click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
}

async function switchToHeaders(page: Page) {
  const editorTabs = page.locator('.border-b.border-border.px-3');
  await editorTabs.locator('button', { hasText: 'Headers' }).click();
}

function getFirstHeaderRow(page: Page) {
  const row = page.locator('input[type="checkbox"]').first()
    .locator('xpath=ancestor::div[contains(@class,"grid-cols")]');
  // Inputs without type attr are the key/value fields
  const keyInput = row.locator('input:not([type])').first();
  const valueInput = row.locator('input:not([type])').last();
  return { row, keyInput, valueInput };
}

test.describe('Content-Type auto-update on body type change', () => {
  test('should set Content-Type to application/json when switching to JSON', async ({ page }) => {
    await createAndOpenRequest(page);
    await selectBodyType(page, 'JSON');
    await switchToHeaders(page);

    await expect(page.locator('input[type="checkbox"]')).toHaveCount(1);
    const { keyInput, valueInput } = getFirstHeaderRow(page);
    await expect(keyInput).toHaveValue('Content-Type');
    await expect(valueInput).toHaveValue('application/json');
  });

  test('should set Content-Type to application/xml when switching to XML', async ({ page }) => {
    await createAndOpenRequest(page);
    await selectBodyType(page, 'XML');
    await switchToHeaders(page);

    const { keyInput, valueInput } = getFirstHeaderRow(page);
    await expect(keyInput).toHaveValue('Content-Type');
    await expect(valueInput).toHaveValue('application/xml');
  });

  test('should set Content-Type to application/x-www-form-urlencoded for Form Data', async ({ page }) => {
    await createAndOpenRequest(page);
    await selectBodyType(page, 'Form Data');
    await switchToHeaders(page);

    const { keyInput, valueInput } = getFirstHeaderRow(page);
    await expect(keyInput).toHaveValue('Content-Type');
    await expect(valueInput).toHaveValue('application/x-www-form-urlencoded');
  });

  test('should set Content-Type to application/octet-stream for Binary', async ({ page }) => {
    await createAndOpenRequest(page);
    await selectBodyType(page, 'Binary');
    await switchToHeaders(page);

    const { keyInput, valueInput } = getFirstHeaderRow(page);
    await expect(keyInput).toHaveValue('Content-Type');
    await expect(valueInput).toHaveValue('application/octet-stream');
  });

  test('should remove Content-Type when switching to None', async ({ page }) => {
    await createAndOpenRequest(page);

    await selectBodyType(page, 'JSON');

    const selector = page.locator('button[role="combobox"]').last();
    await selector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'None' }).click();

    await switchToHeaders(page);
    await expect(page.locator('input[type="checkbox"]')).toHaveCount(0);
  });

  test('should update Content-Type when switching between body types', async ({ page }) => {
    await createAndOpenRequest(page);

    await selectBodyType(page, 'JSON');
    await switchToHeaders(page);
    const { valueInput } = getFirstHeaderRow(page);
    await expect(valueInput).toHaveValue('application/json');

    await selectBodyType(page, 'XML');
    await switchToHeaders(page);
    const { valueInput: xmlValue } = getFirstHeaderRow(page);
    await expect(xmlValue).toHaveValue('application/xml');
  });

  test('should show Content-Type in Headers badge count', async ({ page }) => {
    await createAndOpenRequest(page);
    await selectBodyType(page, 'JSON');

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const headersTab = editorTabs.locator('button', { hasText: 'Headers' });
    await expect(headersTab).toContainText('(1)');
  });
});
