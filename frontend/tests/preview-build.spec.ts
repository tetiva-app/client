import { test, expect } from '@playwright/test';

test.describe('Preview build', () => {
  test('locks sign-in and publishing on the rail and says why on hover', async ({ page }) => {
    await page.goto('/?flavor=preview');

    const sync = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(sync).toHaveAttribute('aria-disabled', 'true');
    await sync.hover();
    await expect(page.getByText('Preview build: sign-in is unavailable')).toBeVisible();
    await sync.click({ force: true });
    await expect(page.getByRole('dialog')).toHaveCount(0);

    const publications = page.getByTestId('activity-publications');
    await expect(publications).toHaveAttribute('aria-disabled', 'true');
    await publications.hover();
    await expect(page.getByText('Preview build: sign-in is unavailable')).toBeVisible();
    await publications.click({ force: true });
    await expect(page.getByTestId('activity-collections').locator('div.bg-primary')).toHaveCount(1);

    await expect(page.getByTestId('preview-badge')).toHaveText(/^Preview build \d+\.\d+\.\d+$/);
  });

  test('has no Updates section and names itself in Settings', async ({ page }) => {
    await page.goto('/?flavor=preview');
    await page.getByRole('button', { name: 'Settings' }).click();

    await expect(page.getByTestId('settings-nav-about')).toBeVisible();
    await expect(page.getByTestId('settings-nav-updates')).toHaveCount(0);
    await expect(page.getByTestId('settings-nav')).toContainText('Tetiva Preview');
  });

  test('the regular build keeps sign-in open', async ({ page }) => {
    await page.goto('/');

    const sync = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(sync).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('preview-badge')).toHaveCount(0);
    await sync.click();
    await expect(page.getByRole('dialog')).toBeVisible();
  });
});
