import { test, expect } from '@playwright/test';

test.describe('Sidebar layout', () => {
  test('sidebar scrolls when collections overflow viewport', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 500 });
    await page.goto('/');

    const count = 25;
    for (let i = 0; i < count; i++) {
      await page.getByTitle('New Collection').click();
      const dialog = page.getByRole('dialog');
      const input = dialog.getByPlaceholder('Collection name');
      await input.fill(`Collection ${i}`);
      await input.press('Enter');
      await expect(dialog).toBeHidden();
    }

    const sidebar = page.locator('aside');
    await expect(sidebar.getByText(`Collection 0`)).toBeVisible();

    const viewport = sidebar.locator('[data-slot="scroll-area-viewport"]').first();

    const metrics = await viewport.evaluate((el: HTMLElement) => ({
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    }));

    expect(metrics.clientHeight).toBeGreaterThan(0);
    expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight);

    await viewport.evaluate((el: HTMLElement) => { el.scrollTop = el.scrollHeight; });
    await expect(sidebar.getByText(`Collection ${count - 1}`)).toBeVisible();
  });

  test('chevron appears immediately on sub-collections containing requests', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Collection name').fill('Root');
    await dialog.getByPlaceholder('Collection name').press('Enter');
    await expect(dialog).toBeHidden();

    const sidebar = page.locator('aside');
    const rootItem = sidebar.getByText('Root', { exact: true });
    await expect(rootItem).toBeVisible();

    await rootItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Sub-Collection').click();
    dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Collection name').fill('Child');
    await dialog.getByPlaceholder('Collection name').press('Enter');
    await expect(dialog).toBeHidden();

    await rootItem.click();
    const childItem = sidebar.getByText('Child', { exact: true });
    await expect(childItem).toBeVisible();

    await childItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Request name').fill('Ping');
    await dialog.getByPlaceholder('Request name').press('Enter');
    await expect(dialog).toBeHidden();

    // Collapse then re-expand root to force Child to remount from scratch.
    await rootItem.click();
    await expect(childItem).toBeHidden();
    await rootItem.click();
    await expect(childItem).toBeVisible();

    // chevron must render even though Child has never been expanded
    const childRow = childItem.locator('xpath=ancestor::button[1]');
    const chevron = childRow.locator('svg.lucide-chevron-right');
    await expect(chevron).toBeVisible();
  });
});
