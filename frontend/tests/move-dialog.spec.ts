import { test, expect, type Page } from '@playwright/test';

async function createCollections(page: Page, names: string[]) {
  await page.goto('/');

  for (const name of names) {
    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Collection name').fill(name);
    await dialog.getByPlaceholder('Collection name').press('Enter');
  }

  const sidebar = page.locator('aside');
  for (const name of names) {
    await expect(sidebar.getByText(name)).toBeVisible();
  }
}

async function createRequest(page: Page, collectionName: string, requestName: string) {
  const sidebar = page.locator('aside');
  const col = sidebar.getByText(collectionName);
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Request name').fill(requestName);
  await dialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

test.describe('Move To Dialog', () => {
  test('should open Move dialog from multi-select context menu (2 collections selected)', async ({ page }) => {
    await createCollections(page, ['Col A', 'Col B', 'Destination']);

    const sidebar = page.locator('aside');
    const btnA = page.locator('button', { hasText: 'Col A' }).first();
    const btnB = page.locator('button', { hasText: 'Col B' }).first();

    await btnA.click({ modifiers: ['Meta'] });
    await btnB.click({ modifiers: ['Meta'] });

    await btnB.click({ button: 'right' });
    const menu = page.getByRole('menu');
    await expect(menu).toBeVisible();
    await expect(menu.getByText('Move to...')).toBeVisible();

    await menu.getByText('Move to...').click();

    const moveDialog = page.getByRole('alertdialog');
    await expect(moveDialog).toBeVisible();
    await expect(moveDialog).toContainText('Move 2 item(s) to...');
  });

  test('should show Root and collections in move dialog', async ({ page }) => {
    await createCollections(page, ['Alpha', 'Beta']);

    const sidebar = page.locator('aside');
    const btnA = page.locator('button', { hasText: 'Alpha' }).first();
    const btnB = page.locator('button', { hasText: 'Beta' }).first();

    await btnA.click({ modifiers: ['Meta'] });
    await btnB.click({ modifiers: ['Meta'] });
    await btnA.click({ button: 'right' });
    await page.getByRole('menu').getByText('Move to...').click();

    const moveDialog = page.getByRole('alertdialog');
    await expect(moveDialog).toBeVisible();

    await expect(moveDialog.getByText('Root (top level)')).toBeVisible();
    // TODO(e2e): assert the selected collections are disabled in the target list
  });

  test('should move collections into a target collection', async ({ page }) => {
    await createCollections(page, ['Move Me 1', 'Move Me 2', 'Container']);

    const sidebar = page.locator('aside');
    const btn1 = page.locator('button', { hasText: 'Move Me 1' }).first();
    const btn2 = page.locator('button', { hasText: 'Move Me 2' }).first();

    await btn1.click({ modifiers: ['Meta'] });
    await btn2.click({ modifiers: ['Meta'] });

    await btn1.click({ button: 'right' });
    await page.getByRole('menu').getByText('Move to...').click();

    const moveDialog = page.getByRole('alertdialog');
    await expect(moveDialog).toBeVisible();

    await moveDialog.getByText('Container').click();
    await moveDialog.getByRole('button', { name: 'Move', exact: true }).click();

    await expect(moveDialog).not.toBeVisible();

    await sidebar.getByText('Container').click();
    await expect(sidebar.getByText('Move Me 1')).toBeVisible();
    await expect(sidebar.getByText('Move Me 2')).toBeVisible();
  });

  test('should move requests via multi-select', async ({ page }) => {
    await createCollections(page, ['Source Col', 'Target Col']);
    await createRequest(page, 'Source Col', 'Req 1');
    await createRequest(page, 'Source Col', 'Req 2');

    const sidebar = page.locator('aside');

    const sourceCol = sidebar.getByText('Source Col');
    await sourceCol.click();
    await expect(sidebar.getByText('Req 1')).toBeVisible();
    await expect(sidebar.getByText('Req 2')).toBeVisible();

    const req1 = sidebar.getByText('Req 1');
    const req2 = sidebar.getByText('Req 2');

    await req1.click({ modifiers: ['Meta'] });
    await req2.click({ modifiers: ['Meta'] });

    await req2.click({ button: 'right' });
    await page.getByRole('menu').getByText('Move to...').click();

    const moveDialog = page.getByRole('alertdialog');
    await expect(moveDialog).toBeVisible();

    await moveDialog.getByText('Target Col').click();
    await moveDialog.getByRole('button', { name: 'Move', exact: true }).click();

    await expect(moveDialog).not.toBeVisible();

    await sidebar.getByText('Target Col').click();
    await expect(sidebar.getByText('Req 1')).toBeVisible();
    await expect(sidebar.getByText('Req 2')).toBeVisible();
  });

  test('should cancel move dialog', async ({ page }) => {
    await createCollections(page, ['Stay 1', 'Stay 2']);

    const sidebar = page.locator('aside');
    const btn1 = page.locator('button', { hasText: 'Stay 1' }).first();
    const btn2 = page.locator('button', { hasText: 'Stay 2' }).first();

    await btn1.click({ modifiers: ['Meta'] });
    await btn2.click({ modifiers: ['Meta'] });
    await btn1.click({ button: 'right' });
    await page.getByRole('menu').getByText('Move to...').click();

    const moveDialog = page.getByRole('alertdialog');
    await expect(moveDialog).toBeVisible();

    await moveDialog.getByRole('button', { name: 'Cancel' }).click();
    await expect(moveDialog).not.toBeVisible();

    await expect(sidebar.getByText('Stay 1')).toBeVisible();
    await expect(sidebar.getByText('Stay 2')).toBeVisible();
  });
});
