import { test, expect, type Page } from '@playwright/test';

async function openSchema(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Graph');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collection = page.locator('aside').getByText('Graph');
  await expect(collection).toBeVisible();
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: 'GraphQL', exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Users');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await page.getByPlaceholder('https://api.example.com/graphql').fill('https://api.example.com/graphql');
  await page.getByRole('button', { name: 'Load Schema', exact: true }).click();
  await page.getByRole('button', { name: /^Docs\b/ }).locator('..').getByRole('button', { name: /^Schema\b/ }).click();
}

const schemaSearch = (page: Page) => page.getByPlaceholder('Search...', { exact: true });

test.describe('Cmd+F in the GraphQL schema', () => {
  test('after a click into the SDL text it focuses the schema search', async ({ page }) => {
    await openSchema(page);
    await page.getByRole('button', { name: 'SDL', exact: true }).click();
    const sdl = page.locator('.cm-content', { hasText: 'type User' });

    await sdl.getByText('type User').click();
    await page.keyboard.press('ControlOrMeta+f');

    await expect(schemaSearch(page)).toBeFocused();
  });

  test('after a click on a Browse row it focuses the schema search', async ({ page }) => {
    await openSchema(page);
    await page.getByRole('button', { name: /^Q users/ }).click();
    await expect(page.getByText('OBJECT')).toBeVisible();

    await page.keyboard.press('ControlOrMeta+f');

    await expect(schemaSearch(page)).toBeFocused();
  });

  test('after a click on an operation\'s return type it focuses the schema search', async ({ page }) => {
    await openSchema(page);
    await page.getByRole('button', { name: /^Q users/ }).locator('.text-\\[\\#A78BFA\\]').click();
    await expect(page.getByText('OBJECT')).toBeVisible();

    await page.keyboard.press('ControlOrMeta+f');

    await expect(schemaSearch(page)).toBeFocused();
  });
});

test('SDL search highlights every match and counts them', async ({ page }) => {
  await openSchema(page);
  await page.getByRole('button', { name: 'SDL', exact: true }).click();
  await expect(page.locator('.cm-content', { hasText: 'type User' })).toBeVisible();

  await schemaSearch(page).click();
  await schemaSearch(page).pressSequentially('User');
  await expect(page.locator('.cm-searchMatch').first()).toBeVisible();
  await expect(page.getByTestId('schema-find-status')).toHaveText(/^\d+\/\d+$/);

  await schemaSearch(page).fill('zzz-nothing');
  await expect(page.getByTestId('schema-find-status')).toHaveText('No matches');
});
