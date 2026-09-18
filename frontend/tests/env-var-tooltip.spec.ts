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

test.describe('Env var drill-through from script editor', () => {
  test('Cmd+Click on undefined var opens modal with prefilled key', async ({ page }) => {
    await createCollection(page, 'Env Var Tooltip');

    const sidebar = page.locator('aside');
    await sidebar.getByText('Env Var Tooltip').dblclick();

    const scriptsTab = page.getByRole('tab', { name: 'Scripts' });
    await scriptsTab.click();

    const editorContent = page.locator('[role="tabpanel"]:not([hidden]) .cm-content').first();
    await expect(editorContent).toBeVisible();

    await editorContent.click();
    await page.keyboard.type('const x = "{{myVar}}"');

    // Undefined variables get the `cm-var-unresolved` class
    const varSpan = editorContent.locator('.cm-var-unresolved').first();
    await expect(varSpan).toBeVisible();
    await varSpan.click({ modifiers: ['Meta'] });

    const modal = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    await expect(modal).toBeVisible();

    // undefined variable → prefill mode
    const varNameInput = modal.getByPlaceholder('Variable name');
    await expect(varNameInput).toHaveValue('myVar');
  });
});
