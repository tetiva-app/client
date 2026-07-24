import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Auth Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Auth Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();

  const editorTabs = page.locator('.border-b.border-border.px-3');
  await editorTabs.locator('button', { hasText: 'Auth' }).click();
}

test.describe('Auth Editor', () => {
  test('should show None as default auth type', async ({ page }) => {
    await createAndOpenRequest(page);

    await expect(page.getByText('Authorization Type')).toBeVisible();
    const authSelector = page.locator('button[role="combobox"]').last();
    await expect(authSelector).toContainText('Inherit');

    await expect(page.getByText('inherits authentication')).toBeVisible();
  });

  test('should open auth type dropdown with all options', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    const listbox = page.locator('[role="listbox"]');
    await expect(listbox).toBeVisible();

    for (const label of ['None', 'Basic Auth', 'Bearer Token', 'API Key']) {
      await expect(listbox.locator('[role="option"]', { hasText: label })).toBeVisible();
    }
  });

  test('should switch to Basic Auth and show username/password fields', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Basic Auth' }).click();

    await expect(authSelector).toContainText('Basic Auth');

    await expect(page.locator('[aria-placeholder="Username"]')).toBeVisible();
    await expect(page.locator('[aria-placeholder="Password"]')).toBeVisible();

    const editorTabs = page.locator('.border-b.border-border.px-3');
    await expect(editorTabs.locator('button', { hasText: 'Auth' })).toContainText('(Basic)');
  });

  test('should fill Basic Auth username and password', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Basic Auth' }).click();

    await page.locator('[aria-placeholder="Username"]').fill('admin');
    await page.locator('[aria-placeholder="Password"]').fill('secret123');

    await expect(page.locator('[aria-placeholder="Username"]')).toContainText('admin');
    // masked via CSS text-security, not type=password
    const passwordInput = page.locator('[aria-placeholder="Password"]');
    await expect(passwordInput).toHaveAttribute('style', /text-security/);
  });

  test('should toggle password visibility in Basic Auth', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Basic Auth' }).click();

    await page.locator('[aria-placeholder="Password"]').fill('mysecret');

    const passwordInput = page.locator('[aria-placeholder="Password"]');
    await expect(passwordInput).toHaveAttribute('style', /text-security/);

    const eyeToggle = page.locator('button').filter({ has: page.locator('.lucide-eye, .lucide-eye-off') });
    await eyeToggle.click();

    await expect(passwordInput).not.toHaveAttribute('style', /text-security/);

    await eyeToggle.click();
    await expect(passwordInput).toHaveAttribute('style', /text-security/);
  });

  test('should switch to Bearer Token and show prefix/token fields', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Bearer Token' }).click();

    await expect(authSelector).toContainText('Bearer Token');

    const prefixInput = page.locator('[aria-placeholder="Bearer"]');
    await expect(prefixInput).toBeVisible();

    const tokenInput = page.locator('[aria-placeholder="Paste your token here"]');
    await expect(tokenInput).toBeVisible();

    const editorTabs = page.locator('.border-b.border-border.px-3');
    await expect(editorTabs.locator('button', { hasText: 'Auth' })).toContainText('(Bearer)');
  });

  test('should fill Bearer Token fields', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Bearer Token' }).click();

    const prefixInput = page.locator('[aria-placeholder="Bearer"]');
    await expect(prefixInput).toContainText('Bearer');

    const tokenInput = page.locator('[aria-placeholder="Paste your token here"]');
    await tokenInput.fill('eyJhbGciOiJIUzI1NiJ9.test');
    await expect(tokenInput).toContainText('eyJhbGciOiJIUzI1NiJ9.test');
  });

  test('should switch to API Key and show key/value/addTo fields', async ({ page }) => {
    await createAndOpenRequest(page);

    // Auth type selector — scoped under "Authorization Type" label
    const authArea = page.locator('label', { hasText: 'Authorization Type' }).locator('..');
    const authSelector = authArea.locator('button[role="combobox"]');
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'API Key' }).click();

    await expect(authSelector).toContainText('API Key');

    await expect(page.locator('[aria-placeholder="X-API-Key"]')).toBeVisible();
    await expect(page.locator('[aria-placeholder="Your API key value"]')).toBeVisible();

    await expect(page.getByText('Add to')).toBeVisible();
    const addToSelector = page.locator('button[role="combobox"]').filter({ hasText: 'Header' });
    await expect(addToSelector).toBeVisible();

    const editorTabs = page.locator('.border-b.border-border.px-3');
    await expect(editorTabs.locator('button', { hasText: 'Auth' })).toContainText('(API Key)');
  });

  test('should change API Key "Add to" to Query Params', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'API Key' }).click();

    await page.locator('[aria-placeholder="X-API-Key"]').fill('api-key');
    await page.locator('[aria-placeholder="Your API key value"]').fill('sk-12345');

    const addToSelector = page.locator('button[role="combobox"]').filter({ hasText: 'Header' });
    await addToSelector.click();
    const addToListbox = page.locator('[role="listbox"]').last();
    await addToListbox.locator('[role="option"]', { hasText: 'Query Params' }).click();

    await expect(page.locator('button[role="combobox"]').filter({ hasText: 'Query Params' })).toBeVisible();
  });

  test('should navigate auth type dropdown with keyboard', async ({ page }) => {
    await createAndOpenRequest(page);

    const authSelector = page.locator('button[role="combobox"]').last();
    await authSelector.click();
    await expect(page.locator('[role="listbox"]')).toBeVisible();

    // ArrowDown twice from Inherit (default) → None → Basic Auth
    await authSelector.press('ArrowDown');
    await authSelector.press('ArrowDown');
    await authSelector.press('Enter');

    await expect(authSelector).toContainText('Basic Auth');
    await expect(page.locator('[aria-placeholder="Username"]')).toBeVisible();
  });

  test('should show no badge when auth type is None', async ({ page }) => {
    await createAndOpenRequest(page);

    const editorTabs = page.locator('.border-b.border-border.px-3');
    const authTab = editorTabs.locator('button', { hasText: 'Auth' });

    await expect(authTab).not.toContainText('(');
  });
});
