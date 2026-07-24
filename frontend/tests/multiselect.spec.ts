import { test, expect } from '@playwright/test';

test.describe('Multi-selection', () => {
  test('should allow multi-selection with Meta/Ctrl key and not expand/collapse folders', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Collection A');
    await input.press('Enter');

    await page.getByTitle('New Collection').click();
    dialog = page.getByRole('dialog');
    input = dialog.getByPlaceholder('Collection name');
    await input.fill('Collection B');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const colA = sidebar.getByText('Collection A');
    const colB = sidebar.getByText('Collection B');

    await expect(colA).toBeVisible();
    await expect(colB).toBeVisible();

    const btnA = page.locator('button', { hasText: 'Collection A' }).first();
    const btnB = page.locator('button', { hasText: 'Collection B' }).first();

    await btnA.click();
    await expect(btnA).toHaveAttribute('aria-expanded', 'true');

    await btnB.click({ modifiers: ['Meta'] });
    
    // bg-primary/10 marks a selected row
    await expect(btnB).toHaveClass(/bg-primary\/10/);
    
    await expect(btnB).toHaveAttribute('aria-expanded', 'false');

    await btnA.click({ modifiers: ['Meta'] });
    await expect(btnA).toHaveClass(/bg-primary\/10/);
    await expect(btnA).toHaveAttribute('aria-expanded', 'true');
  });
});
