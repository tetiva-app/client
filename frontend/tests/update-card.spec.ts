import { test, expect } from '@playwright/test';

test.describe('Sidebar update card', () => {
  test('Later folds the card into a line, × hides it, and both survive a reload', async ({ page }) => {
    await page.goto('/?mock=update-ready');
    const card = page.getByTestId('update-card');
    await expect(card).toBeVisible();
    await expect(card).toContainText('Tetiva 1.2.2 downloaded');
    await expect(page.getByTestId('update-card-primary')).toContainText('Restart');

    await page.getByTestId('update-card-later').click();
    await expect(card).toHaveCount(0);
    await expect(page.getByTestId('update-line')).toContainText('1.2.2 ready');

    await page.reload();
    await expect(page.getByTestId('update-line')).toBeVisible();
    await expect(page.getByTestId('update-card')).toHaveCount(0);

    await page.getByTestId('update-line-dismiss').click();
    await expect(page.getByTestId('update-slot')).toHaveCount(0);

    await page.reload();
    await expect(page.getByTestId('update-badge')).toBeVisible();
    await expect(page.getByTestId('update-slot')).toHaveCount(0);
  });

  test('stays below the tree on every sidebar panel', async ({ page }) => {
    await page.goto('/?mock=update-ready');
    for (const section of ['history', 'publications', 'collections']) {
      await page.getByTestId(`activity-${section}`).click();
      await expect(page.getByTestId('update-card')).toBeInViewport({ ratio: 1 });
    }
  });

  test('shows the apt command with a working Copy', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.goto('/?mock=update-apt');
    await expect(page.getByTestId('update-card')).toContainText('Tetiva 1.2.2 in apt.tetiva.app');
    await expect(page.getByTestId('update-apt-command')).toContainText('sudo apt install --only-upgrade tetiva');

    await page.getByTestId('update-apt-copy').click();
    await expect(page.getByTestId('update-apt-copy')).toHaveAttribute('aria-label', 'Copied');
    expect(await page.evaluate(() => navigator.clipboard.readText()))
      .toBe('sudo apt update && sudo apt install --only-upgrade tetiva');
  });

  test('the Settings dot opens the Updates section with the restart', async ({ page }) => {
    await page.goto('/?mock=update-ready');
    await page.getByTestId('activity-settings').click();
    await expect(page.getByTestId('settings-section-updates')).toBeVisible();
    await expect(page.getByTestId('settings-update-restart')).toBeVisible();
  });

  test('the restarting overlay covers Settings when the restart starts there', async ({ page }) => {
    await page.goto('/?mock=update-ready');
    await page.getByTestId('activity-settings').click();
    await page.getByTestId('settings-update-restart').click();

    await expect(page.getByTestId('restart-overlay')).toBeVisible();
    const onTop = await page.evaluate(() =>
      document.elementFromPoint(innerWidth / 2, innerHeight / 2)?.closest('[data-testid="restart-overlay"]') != null);
    expect(onTop).toBe(true);
  });
});
