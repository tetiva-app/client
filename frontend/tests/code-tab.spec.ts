import { test, expect, type Page } from '@playwright/test';

const URL_INPUT = '[aria-placeholder="Enter request URL"]';
const SNIPPETS_BUNDLE = /\/snippets\/dist\/snippets\.mjs/;

async function openCodeTab(page: Page, url: string) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Code Collection');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collectionItem = page.locator('aside').getByText('Code Collection');
  await expect(collectionItem).toBeVisible();
  await collectionItem.click();
  await collectionItem.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Code Request');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await page.locator(URL_INPUT).fill(url);
  await page.getByRole('button', { name: 'Code', exact: true }).click();
}

const generatedCode = (page: Page) => page.getByRole('textbox', { name: 'Generated code' });

test.describe('Code tab', () => {
  test('shows the unsaved request as cURL and keeps Copy off until an edit is rebuilt', async ({ page }) => {
    await openCodeTab(page, 'https://api.example.com/users');
    const copy = page.getByRole('button', { name: 'Copy', exact: true });

    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/users'");
    await expect(copy).toBeEnabled();

    await page.locator(URL_INPUT).fill('https://api.example.com/orders');
    await expect(copy).toBeDisabled();

    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/orders'");
    await expect(copy).toBeEnabled();
  });

  test('switches the language and highlights the new code', async ({ page }) => {
    await openCodeTab(page, 'https://api.example.com/users');
    await expect(generatedCode(page)).toContainText('curl');

    await page.getByRole('combobox', { name: 'Language' }).click();
    await page.getByRole('listbox').getByRole('option', { name: 'Python' }).click();

    await expect(page.getByRole('combobox', { name: 'Language' })).toHaveText('Python');
    await expect(generatedCode(page)).toContainText('import requests');
    await expect(generatedCode(page).locator('span', { hasText: /^import$/ })).toBeVisible();
  });

  test('picks the language from the keyboard', async ({ page }) => {
    await openCodeTab(page, 'https://api.example.com/users');
    const language = page.getByRole('combobox', { name: 'Language' });
    await expect(generatedCode(page)).toContainText('curl');

    await language.focus();
    await page.keyboard.press('Enter');
    const options = page.getByRole('listbox').getByRole('option');
    await expect(options).toHaveText(['cURL', 'Python', 'JavaScript', 'Go', 'Java (HttpClient)', 'Java (OkHttp)', 'C#', 'PHP']);
    await expect(options.first()).toBeFocused();
    for (const next of [1, 2, 3]) {
      await page.keyboard.press('ArrowDown');
      await expect(options.nth(next)).toBeFocused();
    }
    await page.keyboard.press('Enter');

    await expect(page.getByRole('listbox')).toHaveCount(0);
    await expect(language).toHaveText('Go');
    await expect(language).toBeFocused();
    await expect(generatedCode(page)).toContainText('package main');
  });

  test('offers secret values only while variables are resolved', async ({ page }) => {
    await openCodeTab(page, 'https://api.example.com/users');
    const resolve = page.getByRole('switch', { name: 'Resolve variables' });
    const secrets = page.getByRole('switch', { name: 'Include secret values' });

    await expect(resolve).toBeChecked();
    await expect(secrets).toBeEnabled();
    await expect(secrets).not.toBeChecked();

    await secrets.click();
    await expect(secrets).toBeChecked();

    await resolve.click();
    await expect(secrets).toBeDisabled();
    await expect(secrets).not.toBeChecked();

    await resolve.click();
    await expect(secrets).toBeEnabled();
    await expect(secrets).not.toBeChecked();
  });

  test('announces warnings politely', async ({ page }) => {
    await openCodeTab(page, 'https://api.example.com/{{a b}}');
    const warnings = page.getByRole('list', { name: 'Snippet warnings' });

    await expect(warnings).toHaveAttribute('aria-live', 'polite');
    await expect(warnings).toContainText('Variable "a b" contains unsafe characters and is shown as VAR_0');
  });

  test('copies the code', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await openCodeTab(page, 'https://api.example.com/users');
    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/users'");

    await page.getByRole('button', { name: 'Copy', exact: true }).click();

    await expect(page.getByText('Copied', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("curl 'https://api.example.com/users'");
  });

  test('reports a snippet library that fails to load as an alert', async ({ page }) => {
    await page.route(SNIPPETS_BUNDLE, (route) => route.abort());
    await openCodeTab(page, 'https://api.example.com/users');

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Copy', exact: true })).toBeDisabled();
  });

  test('marks the code busy while it is being generated', async ({ page }) => {
    let release!: () => void;
    const released = new Promise<void>((resolve) => { release = resolve; });
    await page.route(SNIPPETS_BUNDLE, async (route) => {
      await released;
      await route.continue();
    });
    await openCodeTab(page, 'https://api.example.com/users');

    const status = page.getByRole('status').filter({ hasText: 'Generating code' });
    await expect(status).toBeVisible();
    await expect(page.locator('[aria-busy="true"]').getByRole('textbox', { name: 'Generated code' })).toBeVisible();

    release();

    await expect(status).toHaveCount(0);
    await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
    await expect(generatedCode(page)).toContainText('curl');
  });
});
