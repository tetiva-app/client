import { test, expect } from '@playwright/test';

test('a draft replayed from history says its auth was left behind', async ({ page }) => {
  await page.goto('/');
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Replay');
  await dialog.getByPlaceholder('Collection name').press('Enter');
  const collection = page.locator('aside').getByText('Replay', { exact: true });
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  await page.getByRole('dialog').getByPlaceholder('Request name').fill('Users');
  await page.getByRole('dialog').getByPlaceholder('Request name').press('Enter');
  await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');
  await page.getByRole('button', { name: 'More actions' }).locator('..').getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.getByText('alice@example.com')).toBeVisible();

  await page.getByRole('button', { name: 'History', exact: true }).click();
  const row = page.locator('aside').getByText('api.example.com/users').first();
  await row.hover();
  await page.getByTitle('Replay (open as draft)').first().click();

  await expect(page.getByText('This is a draft replayed from history')).toBeVisible();
  await expect(page.getByText('Its auth and credential headers were left out')).toBeVisible();
});
