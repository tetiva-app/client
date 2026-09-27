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

test.describe('Publication', () => {
  test('publishes from the menu, manages the page in the Publish tab and takes it down', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Petstore API');

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Publish…').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('Publish “Petstore API”');
    await expect(page.getByRole('tab', { name: /Publish/ })).toHaveCount(0);

    const submit = dialog.getByTestId('publish-submit');
    await expect(dialog.getByTestId('scan-warning')).toBeVisible();
    await expect(submit).toBeDisabled();
    await dialog.getByTestId('publish-acknowledge').check();
    await expect(submit).toBeEnabled();
    await submit.click();
    await expect(dialog).toBeHidden();

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Publication…').click();
    const tab = page.getByRole('tab', { name: /Publish/ });
    await expect(tab).toHaveAttribute('aria-selected', 'true');
    await expect(tab).toContainText('(Public)');
    await expect(page.getByTestId('publication-published')).toContainText('https://share.tetiva.app/');

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Delete').click();
    await expect(page.getByRole('alertdialog')).toContainText('The collection is published — its page will be taken down.');
    await page.getByRole('alertdialog').getByRole('button', { name: 'Cancel' }).click();

    await page.getByTestId('publication-unpublish').click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Unpublish' }).click();
    await expect(page.getByTestId('publication-empty')).toBeVisible();
    await expect(tab).not.toContainText('(Public)');
  });

  test('picks the environment of the page from the keyboard', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Petstore API');
    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Publish…').click();

    const dialog = page.getByRole('dialog');
    const environment = dialog.getByRole('combobox', { name: 'Environment' });
    await expect(environment).toHaveText('None');
    // The closing menu hands focus back to its row when it unmounts, which would pull it out of an open list.
    await expect(page.getByRole('menu', { includeHidden: true })).toHaveCount(0);

    await environment.focus();
    await page.keyboard.press('Enter');
    const options = page.getByRole('listbox').getByRole('option');
    await expect(options).toHaveText(['None', 'Default']);
    await expect(options.first()).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expect(options.nth(1)).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(page.getByRole('listbox')).toHaveCount(0);
    await expect(environment).toHaveText('Default');
    await expect(environment).toBeFocused();
    await expect(dialog.getByText('Environment Default')).toBeVisible();
  });

  test('offers publishing only on top-level collections', async ({ page }) => {
    await page.goto('/');
    const root = await createCollection(page, 'Root');

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Sub-Collection').click();
    const input = page.getByRole('dialog').getByPlaceholder('Collection name');
    await input.fill('Child');
    await input.press('Enter');
    await root.click();

    await page.locator('aside').getByText('Child').click({ button: 'right' });
    await expect(page.getByRole('menu')).toBeVisible();
    await expect(page.getByRole('menu').getByText(/Publish/)).toHaveCount(0);
  });
});
