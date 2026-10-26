import { test, expect, type Page } from '@playwright/test';

async function openRequest(page: Page, protocol: 'HTTP' | 'WebSocket' = 'HTTP') {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Users API');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const col = page.locator('aside').getByText('Users API');
  await col.click();
  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: protocol, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Get user');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);
}

const urlBar = (page: Page) => page.locator('[aria-placeholder="Enter request URL"]');
const pathTable = (page: Page) => page.getByTestId('path-variables');
const pathKeys = (page: Page) => pathTable(page).locator('input:not([placeholder])');
const pathValue = (page: Page, i: number) => pathTable(page).getByPlaceholder('Value').nth(i);
const paramsTab = (page: Page) => page.locator('.border-b.border-border.px-3').locator('button', { hasText: 'Params' });

test.describe('Path variables', () => {
  test('a :name segment gets a row, and the row follows the URL', async ({ page }) => {
    await openRequest(page);
    await expect(page.getByText('To add a path variable, write :name in the URL path')).toBeVisible();

    await urlBar(page).fill('https://api.example.com/users/:id?fields=:name');
    await expect(page.getByTestId('path-variables-title')).toHaveText('Path Variables');
    await expect(pathKeys(page)).toHaveCount(1);
    await expect(pathKeys(page).first()).toHaveValue('id');
    await expect(paramsTab(page)).toContainText('(2)');

    await pathValue(page, 0).fill('42');
    await urlBar(page).fill('https://api.example.com/users/:id/posts/:postId');
    await expect(pathKeys(page)).toHaveCount(2);
    await expect(pathValue(page, 0)).toHaveValue('42');
    await expect(pathValue(page, 1)).toHaveValue('');

    await urlBar(page).fill('https://api.example.com/users/:i/posts/:postId');
    await expect(pathKeys(page).first()).toHaveValue('i');
    await expect(pathValue(page, 0)).toHaveValue('');
    await urlBar(page).fill('https://api.example.com/users/:id/posts/:postId');
    await expect(pathValue(page, 0)).toHaveValue('42');

    await urlBar(page).fill('http://localhost:8080/users');
    await expect(pathTable(page)).toHaveCount(0);
  });

  test('renaming a key in the table renames the segment in the URL', async ({ page }) => {
    await openRequest(page);
    await urlBar(page).fill('https://api.example.com/users/:id/friends/:id');
    await pathValue(page, 0).fill('7');

    await pathKeys(page).first().fill('userId');
    await pathKeys(page).first().press('Enter');

    await expect(urlBar(page)).toHaveText('https://api.example.com/users/:userId/friends/:userId');
    await expect(pathValue(page, 0)).toHaveValue('7');

    await pathKeys(page).first().fill('a/b');
    await pathKeys(page).first().press('Enter');
    await expect(pathKeys(page).first()).toHaveValue('userId');
  });

  test('the value goes into the URL that gets copied, {{variables}} included', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await openRequest(page);
    await urlBar(page).fill('https://api.example.com/users/:id/:tab');
    await pathValue(page, 0).fill('{{userId}}');

    await page.getByRole('button', { name: 'More actions' }).click();
    await page.getByRole('menuitem', { name: 'Copy as cURL', exact: true }).click();

    await expect(page.getByText('Copied as cURL', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('https://api.example.com/users/{{userId}}/:tab');
  });

  test('the values survive a save and a reopen', async ({ page }) => {
    await openRequest(page);
    await urlBar(page).fill('https://api.example.com/users/:id');
    await pathValue(page, 0).fill('42');
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+s' : 'Control+s');
    await expect(page.locator('[title="Unsaved changes"]')).toHaveCount(0);

    await page.locator('button[title="Close tab"]').click();
    await page.locator('aside').getByText('Get user').click();
    await expect(pathValue(page, 0)).toHaveValue('42');
  });

  test('a WebSocket request has the table too', async ({ page }) => {
    await openRequest(page, 'WebSocket');
    await page.getByPlaceholder('ws://… or wss://…').fill('wss://chat.example.com/rooms/:room');
    await page.locator('button', { hasText: 'Params' }).click();

    await expect(pathKeys(page).first()).toHaveValue('room');
  });
});
