import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Body Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Body Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

  const editorTabs = page.locator('.border-b.border-border.px-3');
  await editorTabs.locator('button', { hasText: 'Body' }).click();
}

test.describe('Body Editor', () => {
  test('should show None as default body type', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await expect(typeSelector).toContainText('None');

    await expect(page.getByText('This request does not have a body')).toBeVisible();
  });

  test('should switch to JSON body type and show editor', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    const listbox = page.locator('[role="listbox"]');
    await expect(listbox).toBeVisible();

    for (const label of ['None', 'JSON', 'XML', 'Raw', 'Form Data', 'Binary']) {
      await expect(listbox.locator('[role="option"]', { hasText: label })).toBeVisible();
    }

    await listbox.locator('[role="option"]', { hasText: 'JSON' }).click();
    await expect(typeSelector).toContainText('JSON');

    await expect(page.locator('.cm-editor')).toBeVisible();

    await expect(page.getByText('Format')).toBeVisible();

    await expect(page.getByPlaceholder('Search...')).toBeVisible();
  });

  test('should type JSON content in editor', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();

    const editor = page.locator('.cm-content:not([aria-placeholder*="request URL"])');
    await editor.click();
    await page.keyboard.type('{"name":"test"}');

    await expect(editor).toContainText('"name"');
  });

  test('should format JSON body', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();

    const editor = page.locator('.cm-content:not([aria-placeholder*="request URL"])');
    await editor.click();
    await page.keyboard.type('{"a":1,"b":2}');

    await page.getByText('Format').click();

    await expect(editor).toContainText('"a"');
    await expect(editor).toContainText('"b"');
  });

  test('should switch to XML body type', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'XML' }).click();

    await expect(typeSelector).toContainText('XML');
    await expect(page.locator('.cm-editor')).toBeVisible();
    await expect(page.getByText('Format')).toBeVisible();
  });

  test('should switch to Raw body type (no Format button)', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Raw' }).click();

    await expect(typeSelector).toContainText('Raw');
    await expect(page.locator('.cm-editor')).toBeVisible();

    await expect(page.getByText('Format')).not.toBeVisible();

    await expect(page.getByPlaceholder('Search...')).toBeVisible();
  });

  test('should switch to Form Data and use key-value editor', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Form Data' }).click();

    await expect(typeSelector).toContainText('Form Data');

    await expect(page.getByPlaceholder('Field name')).toBeVisible();

    await page.getByPlaceholder('Field name').fill('username');
    await page.getByPlaceholder('Value').fill('john');
    await page.getByPlaceholder('Value').press('Enter');

    const checkbox = page.locator('input[type="checkbox"]').first();
    await expect(checkbox).toBeVisible();
    await expect(checkbox).toBeChecked();
  });

  test('should show Body badge when type is not None', async ({ page }) => {
    await createAndOpenRequest(page);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const bodyTab = editorTabs.locator('button', { hasText: 'Body' });

    await expect(bodyTab).not.toContainText('(');

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();

    await expect(bodyTab).toContainText('(JSON)');
  });

  test('should preserve body draft when switching types', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();

    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();
    const editor = page.locator('.cm-content:not([aria-placeholder*="request URL"])');
    await editor.click();
    await page.keyboard.type('{"json":"data"}');

    // Switch to Raw — text-family types (JSON/XML/Raw) preserve content on switch
    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Raw' }).click();
    await expect(page.locator('.cm-content:not([aria-placeholder*="request URL"])')).toContainText('json');

    await typeSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();
    await expect(page.locator('.cm-content:not([aria-placeholder*="request URL"])')).toContainText('json');
  });

  test('should navigate body type dropdown with keyboard', async ({ page }) => {
    await createAndOpenRequest(page);

    const typeSelector = page.locator('button[role="combobox"]').last();
    await typeSelector.click();
    await expect(page.locator('[role="listbox"]')).toBeVisible();

    // ArrowDown from None to JSON
    await typeSelector.press('ArrowDown');
    await typeSelector.press('Enter');

    await expect(typeSelector).toContainText('JSON');

    await typeSelector.click();
    await typeSelector.press('Escape');
    await expect(page.locator('[role="listbox"]')).not.toBeVisible();
    await expect(typeSelector).toContainText('JSON');
  });
});
