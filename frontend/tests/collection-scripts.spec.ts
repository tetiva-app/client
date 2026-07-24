import { test, expect, type Page } from '@playwright/test';

async function createCollection(page: Page, name: string) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  await expect(sidebar.getByText(name)).toBeVisible();
}

// TODO(e2e): the "Edit Scripts" dialog was removed — rewrite these against the CollectionEditor scripts UI (opened via "Open Details").
test.describe.skip('Collection Scripts Dialog', () => {
  test('should open Edit Scripts dialog via context menu', async ({ page }) => {
    await createCollection(page, 'Scripts Collection');

    const sidebar = page.locator('aside');
    const col = sidebar.getByText('Scripts Collection');

    await col.click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText('Edit Scripts — Scripts Collection');
    await expect(dialog).toContainText('Pre-request and post-response scripts');
  });

  test('should show Pre-request and Post-response tabs in script editor', async ({ page }) => {
    await createCollection(page, 'Scripts Tabs');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Scripts Tabs').click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const preBtn = dialog.locator('button', { hasText: 'Pre-request' });
    const postBtn = dialog.locator('button', { hasText: 'Post-response' });

    await expect(preBtn).toBeVisible();
    await expect(postBtn).toBeVisible();

    // Pre-request is selected by default
    await expect(preBtn).toHaveClass(/bg-primary\/10/);
  });

  test('should type scripts and switch between tabs', async ({ page }) => {
    await createCollection(page, 'Scripts Input');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Scripts Input').click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const editors = dialog.locator('.cm-content');
    await editors.first().click();
    await page.keyboard.type('console.log("pre");');

    await dialog.locator('button', { hasText: 'Post-response' }).click();

    await editors.last().click();
    await page.keyboard.type('console.log("post");');

    await dialog.locator('button', { hasText: 'Pre-request' }).click();
    await expect(editors.first()).toContainText('console.log("pre")');
  });

  test('should save scripts and close dialog', async ({ page }) => {
    await createCollection(page, 'Scripts Save');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Scripts Save').click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const editors = dialog.locator('.cm-content');
    await editors.first().click();
    await page.keyboard.type('pm.test("ok", true);');

    await dialog.locator('button', { hasText: 'Save' }).click();

    await expect(dialog).not.toBeVisible({ timeout: 3000 });
  });

  test('should cancel without saving', async ({ page }) => {
    await createCollection(page, 'Scripts Cancel');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Scripts Cancel').click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const editors = dialog.locator('.cm-content');
    await editors.first().click();
    await page.keyboard.type('should not save');

    await dialog.locator('button', { hasText: 'Cancel' }).click();

    await expect(dialog).not.toBeVisible();
  });

  test('should persist scripts after save and reopen', async ({ page }) => {
    await createCollection(page, 'Scripts Persist');

    const sidebar = page.locator('aside');
    const col = sidebar.getByText('Scripts Persist');

    await col.click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();
    let dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    const editors = dialog.locator('.cm-content');
    await editors.first().click();
    await page.keyboard.type('persist-test');

    await dialog.locator('button', { hasText: 'Save' }).click();
    await expect(dialog).not.toBeVisible({ timeout: 3000 });

    await col.click({ button: 'right' });
    await page.getByRole('menu').getByText('Edit Scripts').click();
    dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    await expect(dialog.locator('.cm-content').first()).toContainText('persist-test');
  });
});
