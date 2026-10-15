import { test, expect, type Locator, type Page } from '@playwright/test';

type Protocol = 'HTTP' | 'GraphQL';

async function createRequest(page: Page, protocol: Protocol) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Find');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collection = page.locator('aside').getByText('Find');
  await expect(collection).toBeVisible();
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: protocol, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Users');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);
}

async function send(page: Page, protocol: Protocol, address = 'https://api.example.com/users') {
  const url = protocol === 'HTTP'
    ? page.locator('[aria-placeholder="Enter request URL"]')
    : page.getByPlaceholder('https://api.example.com/graphql');
  await url.fill(address);
  const label = protocol === 'HTTP' ? 'Send' : 'Query';
  await page.getByRole('button', { name: 'More actions' }).locator('..').getByRole('button', { name: label, exact: true }).click();
}

const search = (page: Page) => page.getByPlaceholder('Search...', { exact: true });
const status = (page: Page) => page.getByTestId('find-status');

async function expectFindsAndReturns(page: Page, content: Locator, term: string) {
  await search(page).click();
  await search(page).pressSequentially(term);

  await expect(content.locator('.cm-searchMatch')).toHaveCount(2);
  await expect(status(page)).toHaveText('1/2');

  await search(page).press('Enter');
  await expect(status(page)).toHaveText('2/2');
  await search(page).press('Shift+Enter');
  await expect(status(page)).toHaveText('1/2');

  await search(page).fill('zzzz-nothing');
  await expect(status(page)).toHaveText('No matches');
  await expect(content.locator('.cm-searchMatch')).toHaveCount(0);

  await search(page).press('Escape');
  await expect(search(page)).toHaveValue('');
  await expect(status(page)).toHaveCount(0);
  await expect(content.locator('.cm-content')).toBeFocused();
}

test.describe('Find in a body', () => {
  test('HTTP response body', async ({ page }) => {
    await createRequest(page, 'HTTP');
    await send(page, 'HTTP');
    const body = page.locator('[data-response-body]');
    await expect(body).toContainText('Alice');

    await expectFindsAndReturns(page, body, 'example');
  });

  test('GraphQL response data', async ({ page }) => {
    await createRequest(page, 'GraphQL');
    await send(page, 'GraphQL');
    const data = page.locator('.cm-editor', { hasText: 'Alice' });
    await expect(data).toBeVisible();

    await expectFindsAndReturns(page, data, 'example');
  });

  test('request body', async ({ page }) => {
    await createRequest(page, 'HTTP');
    await page.locator('.border-b.border-border.px-3').locator('button', { hasText: 'Body' }).click();
    await page.locator('button[role="combobox"]').last().click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'JSON' }).click();
    const editor = page.locator('.cm-editor', { has: page.locator('.cm-content:not([aria-placeholder*="request URL"])') }).last();
    await editor.locator('.cm-content').fill('{"name":"Ann","nick":"Ann"}');

    await expectFindsAndReturns(page, editor, 'Ann');
  });
});

test.describe('Find in a long body', () => {
  for (const protocol of ['HTTP', 'GraphQL'] as const) {
    test(`keeps the current ${protocol} match on screen while typing and stepping`, async ({ page }) => {
      await createRequest(page, protocol);
      await send(page, protocol, 'https://api.example.com/users?count=60');
      await expect(page.getByText('user1@example.com')).toBeVisible();

      const current = page.locator('.cm-searchMatch-selected');
      await search(page).click();
      await search(page).pressSequentially('user40@');
      await expect(status(page)).toHaveText('1/1');
      await expect(current).toBeInViewport();

      await search(page).fill('User 1');
      for (let i = 0; i < 4; i++) await search(page).press('Enter');
      await expect(current).toBeInViewport();
      await search(page).press('Shift+Enter');
      await expect(current).toBeInViewport();
    });
  }
});
