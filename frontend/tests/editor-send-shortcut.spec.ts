import { test, expect, type Page } from '@playwright/test';

async function newRequest(page: Page, kind?: 'GraphQL') {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Send Keys');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collection = page.locator('aside').getByText('Send Keys');
  await expect(collection).toBeVisible();
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  if (kind) await reqDialog.getByRole('button', { name: kind, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);
}

test('Cmd+Enter in the body editor sends without adding a line to the body', async ({ page }) => {
  await newRequest(page);
  await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');
  await page.locator('.border-b.border-border.px-3').locator('button', { hasText: 'Body' }).click();
  await page.locator('button[role="combobox"]').last().click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();

  const body = page.locator('.cm-content:not([aria-placeholder*="request URL"])');
  await body.click();
  await page.keyboard.type('{"v": 4}');
  await page.keyboard.press('ControlOrMeta+Enter');

  await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
  await expect(body.locator('.cm-line')).toHaveCount(1);
});

test('Cmd+Enter in the GraphQL query editor sends without adding a line to the query', async ({ page }) => {
  await newRequest(page, 'GraphQL');
  await page.getByPlaceholder('https://api.example.com/graphql').fill('https://api.example.com/graphql');

  const query = page.locator('.cm-content').first();
  await query.click();
  await page.keyboard.press('ControlOrMeta+a');
  await page.keyboard.type('{ users { id } }');
  await page.keyboard.press('ControlOrMeta+Enter');

  await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
  await expect(query.locator('.cm-line')).toHaveCount(1);
});
