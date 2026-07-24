import { test, expect, type Page } from '@playwright/test';

async function createAndOpenRequest(page: Page) {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Env Selector Test');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const sidebar = page.locator('aside');
  const col = sidebar.getByText('Env Selector Test');
  await col.click();

  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Test Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

// The env selector is a plain button (not combobox) next to the Send button.
// Mock preloads "Default" environment as active.
function getEnvButton(page: Page) {
  return page.locator('.relative.shrink-0 > button').filter({ hasText: /Default|No Environment/ });
}

test.describe('Environment Selector', () => {
  test('should show Default environment as active by default', async ({ page }) => {
    await createAndOpenRequest(page);

    const envBtn = getEnvButton(page);
    await expect(envBtn).toContainText('Default');
    await expect(envBtn).toHaveClass(/text-emerald-400/);
  });

  test('should open dropdown and show environments list', async ({ page }) => {
    await createAndOpenRequest(page);

    const envBtn = getEnvButton(page);
    await envBtn.click();

    await expect(page.getByText('No Environment')).toBeVisible();
    await expect(page.getByText('Manage Environments...')).toBeVisible();
  });

  test('should switch to No Environment', async ({ page }) => {
    await createAndOpenRequest(page);

    const envBtn = getEnvButton(page);
    await envBtn.click();

    await page.locator('button').filter({ hasText: 'No Environment' }).last().click();

    await expect(envBtn).toContainText('No Environment');
    await expect(envBtn).toHaveClass(/text-muted-foreground/);
  });

  test('should switch from No Environment back to Default', async ({ page }) => {
    await createAndOpenRequest(page);

    let envBtn = getEnvButton(page);
    await envBtn.click();
    await page.locator('button').filter({ hasText: 'No Environment' }).last().click();
    await expect(envBtn).toContainText('No Environment');

    await envBtn.click();
    const defaultOption = page.locator('button').filter({ hasText: 'Default' }).last();
    await defaultOption.click();

    await expect(envBtn).toContainText('Default');
    await expect(envBtn).toHaveClass(/text-emerald-400/);
  });

  test('should open Manage Environments modal from dropdown', async ({ page }) => {
    await createAndOpenRequest(page);

    const envBtn = getEnvButton(page);
    await envBtn.click();

    await page.getByText('Manage Environments...').click();

    const dialog = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    await expect(dialog).toBeVisible();
  });

  test('should close dropdown on click outside', async ({ page }) => {
    await createAndOpenRequest(page);

    const envBtn = getEnvButton(page);
    await envBtn.click();

    await expect(page.getByText('Manage Environments...')).toBeVisible();

    // Click on the fixed overlay (click-outside handler)
    await page.locator('.fixed.inset-0.z-40').first().click();

    await expect(page.getByText('Manage Environments...')).not.toBeVisible();
  });
});
