import { test, expect, type Page } from '@playwright/test';

async function createCollection(page: Page, name: string) {
  await page.getByTitle('New Collection').click();
  const input = page.getByRole('dialog').getByPlaceholder('Collection name');
  await input.fill(name);
  await input.press('Enter');
  const item = page.locator('aside').getByText(name);
  await expect(item).toBeVisible();
  return item;
}

async function createRequest(page: Page, collection: ReturnType<Page['locator']>, name: string) {
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const input = page.getByRole('dialog').getByPlaceholder('Request name');
  await input.fill(name);
  await input.press('Enter');
  const url = page.locator('[aria-placeholder="Enter request URL"]');
  await expect(url).toBeVisible();
  return url;
}

async function publishFromMenu(page: Page, collection: ReturnType<Page['locator']>) {
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('Publish…').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByTestId('publish-acknowledge').check();
  await dialog.getByTestId('publish-submit').click();
  await expect(dialog).toBeHidden();
}

async function togglePublishing(page: Page) {
  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByTestId('settings-nav-publishing').click();
  await page.getByTestId('settings-publishing-switch').click();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('settings-dialog')).toHaveCount(0);
}

test.describe('Publications panel', () => {
  test('lists a published collection, counts it as outdated after an edit and opens its Publish tab', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Petstore API');
    const url = await createRequest(page, root, 'List pets');
    await url.fill('https://petstore.example/pets');
    await page.keyboard.press('Meta+s');
    await publishFromMenu(page, root);

    const rail = page.getByTestId('activity-publications');
    const badge = page.getByTestId('publications-badge');
    await rail.click();
    const row = page.getByTestId('publications-row');
    await expect(row).toHaveCount(1);
    await expect(row).toContainText('Petstore API');
    await expect(row.getByTestId('publications-row-state')).toHaveText('up to date');
    await expect(badge).toHaveCount(0);

    await url.fill('https://petstore.example/v2/pets');
    await page.keyboard.press('Meta+s');
    await expect(badge).toHaveText('1');
    await expect(row.getByTestId('publications-row-state')).toHaveText('has changes');

    await row.click();
    const tab = page.getByRole('tab', { name: /Publish/ });
    await expect(tab).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByTestId('publication-published')).toBeVisible();
  });

  test('offers the collections to publish from the empty state', async ({ page }) => {
    await page.goto('/');
    await createCollection(page, 'Billing API');

    await page.getByTestId('activity-publications').click();
    await expect(page.getByTestId('publications-empty')).toBeVisible();
    await page.getByTestId('publications-empty-publish').click();

    const picker = page.getByTestId('publish-picker');
    await expect(picker).toBeVisible();
    await picker.getByTestId('publish-picker-search').fill('bill');
    await picker.getByTestId('publish-picker-row').filter({ hasText: 'Billing API' }).click();

    await expect(page.getByRole('dialog')).toContainText('Publish “Billing API”');
  });

  test('opening a page from the panel brings its tab into view in a narrow window', async ({ page }) => {
    await page.setViewportSize({ width: 960, height: 640 });
    await page.goto('/');
    const root = await createCollection(page, 'Petstore API');
    let url = await createRequest(page, root, 'List every pet in the store');
    for (const name of ['Create a pet in the store', 'Find a pet by its id', 'Update an existing pet', 'Delete a pet from the store']) {
      url = await createRequest(page, root, name);
    }
    await url.fill('https://petstore.example/pets/1');
    await page.keyboard.press('Meta+s');
    await publishFromMenu(page, root);

    await page.getByTestId('activity-publications').click();
    await page.getByTestId('publications-row').click();

    const tab = page.locator('[data-tab-id^="collection:"]');
    await expect(tab).toContainText('Petstore API');
    await expect(tab).toBeInViewport({ ratio: 0.99 });
  });

  test('the tree keeps its folders open across rail sections', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Petstore API');
    await createRequest(page, root, 'List pets');
    const folder = page.locator('aside [data-tree-item-id]').filter({ hasText: 'Petstore API' });
    await expect(folder).toHaveAttribute('aria-expanded', 'false');
    await folder.click();
    await expect(folder).toHaveAttribute('aria-expanded', 'true');

    await page.getByTestId('activity-publications').click();
    await expect(page.getByTestId('publications-panel')).toBeVisible();
    await page.getByTestId('activity-collections').click();

    await expect(folder).toHaveAttribute('aria-expanded', 'true');
    await expect(page.locator('aside').getByText('List pets')).toBeVisible();
  });

  test('hides the rail item and falls back to Collections when publishing is turned off', async ({ page }) => {
    await page.goto('/');
    const rail = page.getByTestId('activity-publications');
    await rail.click();
    await expect(page.getByTestId('publications-panel')).toBeVisible();

    await togglePublishing(page);

    await expect(rail).toHaveCount(0);
    await expect(page.getByTestId('publications-panel')).toHaveCount(0);
    await expect(page.getByTitle('New Collection')).toBeVisible();

    await togglePublishing(page);
    await expect(rail).toBeVisible();
  });
});
