import { test, expect, type Page } from '@playwright/test';

type Protocol = 'HTTP' | 'gRPC' | 'GraphQL';

const TABS: Record<Protocol, string[]> = {
  HTTP: ['Params', 'Auth', 'Headers', 'Body', 'Scripts', 'Docs', 'Examples', 'Code'],
  GraphQL: ['Query', 'Headers', 'Auth', 'Schema', 'Scripts', 'Docs', 'Examples', 'Code'],
  gRPC: ['Body', 'Metadata', 'Schema', 'Scripts', 'Docs', 'Examples', 'Code'],
};

async function createRequest(page: Page, protocol: Protocol) {
  const collection = `${protocol} Tabs`;
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(collection);
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collectionItem = page.locator('aside').getByText(collection);
  await expect(collectionItem).toBeVisible();
  await collectionItem.click();
  await collectionItem.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: protocol, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill(`${protocol} Request`);
  await reqDialog.getByPlaceholder('Request name').press('Enter');
}

const tabStrip = (page: Page) => page.getByRole('button', { name: /^Docs\b/ }).locator('..');
const tab = (page: Page, label: string) => tabStrip(page).getByRole('button', { name: new RegExp(`^${label}\\b`) });
const generatedCode = (page: Page) => page.getByRole('textbox', { name: 'Generated code' });
const examplesEmpty = (page: Page) => page.getByText('No examples yet.');

for (const protocol of Object.keys(TABS) as Protocol[]) {
  test.describe(`${protocol} request tabs`, () => {
    test('put Examples before Code and switch between them', async ({ page }) => {
      await createRequest(page, protocol);

      await expect(tabStrip(page).getByRole('button')).toHaveText(TABS[protocol].map((l) => new RegExp(`^\\s*${l}\\b`)));

      await tab(page, 'Examples').click();
      await expect(examplesEmpty(page)).toBeVisible();

      await tab(page, 'Code').click();
      await expect(generatedCode(page)).toBeVisible();
      await expect(examplesEmpty(page)).toBeHidden();

      await tab(page, 'Examples').click();
      await expect(examplesEmpty(page)).toBeVisible();
      await expect(generatedCode(page)).toHaveCount(0);
    });

    test('Cmd+S saves the example on Examples and the request on Code', async ({ page }) => {
      await createRequest(page, protocol);

      await tab(page, 'Examples').click();
      await page.getByTestId('example-new-empty').click();
      const list = page.getByTestId('example-list');
      await expect(list).toContainText('New example');

      await page.getByRole('textbox', { name: 'Example name' }).fill('Renamed');
      await expect(list.getByTitle('Unsaved changes')).toBeVisible();
      await page.keyboard.press('Meta+s');
      await expect(list.getByTitle('Unsaved changes')).toHaveCount(0);
      await expect(list).toContainText('Renamed');

      await tab(page, 'Code').click();
      await expect(generatedCode(page)).toBeVisible();
      await tab(page, 'Docs').click();
      await page.getByRole('button', { name: 'Add description' }).click();
      await page.locator('[aria-placeholder^="Document this request"]').fill('Saved from the Code tab.');
      const dirty = page.locator('span[title="Unsaved changes"]');
      await expect(dirty.first()).toBeVisible();

      await tab(page, 'Code').click();
      await page.keyboard.press('Meta+s');
      // Well under AUTOSAVE_DELAY_MS, so only the shortcut can clear the dot.
      await expect(dirty).toHaveCount(0, { timeout: 500 });
    });

    test('Cmd+S on Examples saves the request when no example has edits', async ({ page }) => {
      await createRequest(page, protocol);

      await tab(page, 'Examples').click();
      await page.getByTestId('example-new-empty').click();
      await page.keyboard.press('Meta+s');
      await expect(tab(page, 'Examples')).toContainText('(1)');

      await tab(page, 'Docs').click();
      await page.getByRole('button', { name: 'Add description' }).click();
      await page.locator('[aria-placeholder^="Document this request"]').fill('Saved from the Examples tab.');
      const dirty = page.locator('span[title="Unsaved changes"]');
      await expect(dirty.first()).toBeVisible();

      await tab(page, 'Examples').click();
      await page.keyboard.press('Meta+s');
      await expect(dirty).toHaveCount(0, { timeout: 500 });
      await expect(tab(page, 'Examples')).toContainText('(1)');
    });
  });
}
