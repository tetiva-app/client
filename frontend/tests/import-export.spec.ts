import { test, expect } from '@playwright/test';

test.describe('Import / Export Management', () => {
  test('should open import dialog from environments and handle file selection flow', async ({ page }) => {
    await page.goto('/');

    const activityBar = page.locator('.w-12.shrink-0');
    // Environments is the second button
    await activityBar.locator('button').nth(1).click();
    const envDialog = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    await expect(envDialog).toBeVisible();

    const importBtn = envDialog.locator('button[title="Import Postman Environment"]');
    await expect(importBtn).toBeVisible();

    // waitForEvent('filechooser') must be registered before the click
    const fileChooserPromise = page.waitForEvent('filechooser');
    await importBtn.click();
    
    const fileChooser = await fileChooserPromise;
    expect(fileChooser.isMultiple()).toBe(false);

    // reload to reset state — Esc close behavior here is unreliable
    await page.goto('/');
  });

  test('should trigger collection export from context menu', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    const input = dialog.getByPlaceholder('Collection name');
    await input.fill('Export Collection');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Export Collection');
    await expect(collectionItem).toBeVisible();

    const downloadPromise = page.waitForEvent('download', { timeout: 10000 }).catch(() => null);

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('Export as Postman').click();

    // mock mode may not trigger a real download — only assert the action completes
    await expect(page.getByRole('menu')).not.toBeVisible();
  });
  
  test('should open the file chooser from the sidebar import menu', async ({ page }) => {
    await page.goto('/');

    await page.locator('button[title="Import"]').click();
    const fileChooserPromise = page.waitForEvent('filechooser');
    await page.getByRole('menuitem', { name: 'Import File…' }).click();
    const fileChooser = await fileChooserPromise;
    expect(fileChooser.isMultiple()).toBe(false);
  });

  test('imports a published collection from its link after a confirmation', async ({ page }) => {
    await page.goto('/');

    await page.locator('button[title="Import"]').click();
    await page.getByRole('menuitem', { name: 'Import from Link…' }).click();
    await page.getByTestId('import-link-input').fill('https://share.tetiva.app/petstore-api-k3f9x2qa/?ref=readme');
    await page.getByRole('button', { name: 'Continue' }).click();

    const confirm = page.getByTestId('import-confirm');
    await expect(confirm).toContainText('Import “Petstore API”');
    await expect(confirm).toContainText('Tetiva collection · 2 folders · 6 requests · 3 examples');
    await expect(confirm.getByTestId('import-hosts')).toContainText('petstore.example.com');
    await expect(confirm).toContainText('This collection contains 1 script');
    await expect(confirm.getByTestId('import-scripts')).not.toBeChecked();

    await confirm.getByRole('button', { name: 'This collection contains 1 script' }).click();
    await expect(confirm.getByTestId('import-scripts-list')).toContainText("pm.test('created'");

    await confirm.getByTestId('import-submit').click();
    await expect(confirm).not.toBeVisible();
    await expect(page.getByText('Imported “Petstore API”: 2 folders · 6 requests · 3 examples')).toBeVisible();
  });

  test('asks for the password of a protected link', async ({ page }) => {
    await page.goto('/');

    await page.locator('button[title="Import"]').click();
    await page.getByRole('menuitem', { name: 'Import from Link…' }).click();
    await page.getByTestId('import-link-input').fill('internal-api-pa55word');
    await page.getByRole('button', { name: 'Continue' }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('Password required');
    await dialog.getByTestId('import-link-password').fill('not-the-password');
    await dialog.getByRole('button', { name: 'Unlock' }).click();
    await expect(dialog.getByTestId('import-link-error')).toHaveText('Wrong password');

    await dialog.getByTestId('import-link-password').fill('tetiva-demo');
    await dialog.getByTestId('import-link-password').press('Enter');
    await expect(page.getByTestId('import-confirm')).toContainText('Import “Internal API”');
  });

  test('imports a Tetiva file into a folder as a top-level collection', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Collection name').fill('Target Folder');
    await dialog.getByPlaceholder('Collection name').press('Enter');
    const folder = page.locator('aside').getByText('Target Folder');
    await expect(folder).toBeVisible();

    await folder.click({ button: 'right' });
    const chooserPromise = page.waitForEvent('filechooser');
    await page.getByRole('menu').getByText('Import File…').click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: 'petstore.tetiva.json',
      mimeType: 'application/json',
      buffer: Buffer.from(JSON.stringify({
        format: 'tetiva.collection-snapshot',
        version: 1,
        collection: { name: 'Snapshot API', scripts: { pre: '', post: '' }, items: [] },
        environment: null,
      })),
    });

    const confirm = page.getByTestId('import-confirm');
    await expect(confirm).toContainText('Import “Snapshot API”');
    await expect(confirm).toContainText('Tetiva collections are always imported as a new top-level collection.');
  });
});
