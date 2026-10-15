import { test, expect, type Locator, type Page } from '@playwright/test';

function sidebar(page: Page): Locator {
  return page.locator('aside');
}

function openTabs(page: Page): Locator {
  return page.locator('div.h-9.border-b.border-border').first();
}

async function createCollection(page: Page, name: string): Promise<Locator> {
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');
  const item = sidebar(page).getByText(name, { exact: true });
  await expect(item).toBeVisible();
  return item;
}

async function createSubCollection(page: Page, parent: Locator, name: string): Promise<Locator> {
  await parent.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Sub-Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');
  await expect(dialog).not.toBeVisible();
  const item = sidebar(page).getByText(name, { exact: true });
  if (!(await item.isVisible())) await parent.click();
  await expect(item).toBeVisible();
  return item;
}

async function createRequest(page: Page, collection: Locator, name: string) {
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Request name').fill(name);
  await dialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

async function openCollectionAuth(page: Page, collection: Locator) {
  await collection.dblclick();
  await page.getByRole('tab', { name: 'Authorization' }).click();
  await expect(page.getByTestId('collection-auth-type-selector')).toBeVisible();
}

async function pickCollectionAuth(page: Page, label: string) {
  await page.getByTestId('collection-auth-type-selector').click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
  await expect(page.getByTestId('collection-auth-type-selector')).toContainText(label);
}

async function setBearer(page: Page, token: string) {
  await pickCollectionAuth(page, 'Bearer Token');
  await page.locator('[aria-placeholder="Paste your token here"]').click();
  await page.keyboard.type(token);
}

async function setBasic(page: Page, user: string) {
  await pickCollectionAuth(page, 'Basic Auth');
  await page.locator('[aria-placeholder="Username"]').click();
  await page.keyboard.type(user);
}

async function moveTo(page: Page, item: Locator, target: string, expand?: string) {
  await item.click({ button: 'right' });
  await page.getByRole('menu').getByText('Move to…').click();
  const dialog = page.getByRole('alertdialog');
  if (expand) await dialog.locator('button', { hasText: expand }).locator('svg').first().click();
  await dialog.getByText(target, { exact: true }).click();
  await dialog.getByRole('button', { name: 'Move', exact: true }).click();
  await expect(dialog).not.toBeVisible();
}

test.describe('Inherited auth on nested collections', () => {
  test('a sub-collection shows Inherit and the collection its auth comes from', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Root Api');
    await openCollectionAuth(page, root);
    await setBearer(page, 't0ken');
    await page.keyboard.press('Meta+s');

    const child = await createSubCollection(page, root, 'Child Folder');
    await openCollectionAuth(page, child);

    await expect(page.getByTestId('collection-auth-type-selector')).toContainText('Inherit');
    await expect(page.getByTestId('inherited-auth')).toContainText('Bearer Token');
    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Root Api');

    await page.getByTestId('inherited-auth-owner').click();
    await expect(page.getByTestId('collection-auth-type-selector')).toContainText('Bearer Token');
  });

  test('switching a sub-collection back to Inherit before saving shows the parent, not its own scheme', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Unsaved Root');
    await openCollectionAuth(page, root);
    await setBearer(page, 'r00t');
    await page.keyboard.press('Meta+s');

    const child = await createSubCollection(page, root, 'Unsaved Child');
    await openCollectionAuth(page, child);
    await setBasic(page, 'own');
    await page.keyboard.press('Meta+s');

    await pickCollectionAuth(page, 'Inherit');

    await expect(page.getByTestId('inherited-auth')).toContainText('Bearer Token');
    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Unsaved Root');
  });

  test('a change on the parent shows up when the sub-collection tab comes back', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Moving Root');
    await openCollectionAuth(page, root);
    await setBearer(page, 'first');
    await page.keyboard.press('Meta+s');

    const child = await createSubCollection(page, root, 'Watching Child');
    await openCollectionAuth(page, child);
    await expect(page.getByTestId('inherited-auth')).toContainText('Bearer Token');

    await openTabs(page).getByText('Moving Root', { exact: true }).click();
    await setBasic(page, 'second');
    await openTabs(page).getByText('Watching Child', { exact: true }).click();

    await expect(page.getByTestId('inherited-auth')).toContainText('Basic Auth');
  });

  test('a top-level collection keeps No Auth', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Lonely Root');
    await openCollectionAuth(page, root);

    await expect(page.getByTestId('collection-auth-type-selector')).toContainText('No Auth');
    await expect(page.getByText('without auth headers')).toBeVisible();
  });

  test('a request moved to an identically configured sibling names the new owner', async ({ page }) => {
    await page.goto('/');
    const parent = await createCollection(page, 'Sibling Parent');
    const a = await createSubCollection(page, parent, 'Folder A');
    const b = await createSubCollection(page, parent, 'Folder B');
    for (const folder of [a, b]) {
      await openCollectionAuth(page, folder);
      await setBasic(page, 'neo');
      await page.keyboard.press('Meta+s');
    }

    await createRequest(page, a, 'Moved Req');
    await page.locator('.border-b.border-border.px-3').locator('button', { hasText: 'Auth' }).click();
    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Folder A');

    const moved = sidebar(page).getByText('Moved Req', { exact: true });
    if (!(await moved.isVisible())) await a.click();
    await moveTo(page, moved, 'Folder B', 'Sibling Parent');

    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Folder B');
  });

  test('a top-level collection moved under a parent turns No Auth into Inherit', async ({ page }) => {
    await page.goto('/');
    const parent = await createCollection(page, 'Host Parent');
    await openCollectionAuth(page, parent);
    await setBearer(page, 'host');
    await page.keyboard.press('Meta+s');

    const guest = await createCollection(page, 'Guest Col');
    await openCollectionAuth(page, guest);
    await expect(page.getByTestId('collection-auth-type-selector')).toContainText('No Auth');

    await moveTo(page, guest, 'Host Parent');

    await expect(page.getByTestId('collection-auth-type-selector')).toContainText('Inherit');
    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Host Parent');
  });
});
