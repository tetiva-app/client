import { test, expect, type Page } from '@playwright/test';

// The open animation itself moves the frame; the jump under test is the async
// content landing after it.
const OPEN_ANIMATION_MS = 250;

// Sampling runs in the page and starts before the click, so the clock is anchored
// to the frame appearing rather than to the round-trip of the driving command.
async function trackDialogTop(page: Page, open: () => Promise<void>): Promise<number[]> {
  await page.evaluate(() => {
    const samples: Array<[number, number]> = [];
    (window as unknown as { __dialogTops: typeof samples }).__dialogTops = samples;
    let appeared = 0;
    const id = setInterval(() => {
      const el = document.querySelector('[role="dialog"]');
      if (!el) return;
      if (!appeared) appeared = performance.now();
      samples.push([performance.now() - appeared, el.getBoundingClientRect().top]);
    }, 100);
    setTimeout(() => clearInterval(id), 3000);
  });

  await open();
  await page.waitForTimeout(1800);

  return page.evaluate((settleMs) => {
    const samples = (window as unknown as { __dialogTops: Array<[number, number]> }).__dialogTops;
    return samples.filter(([t]) => t >= settleMs).map(([, top]) => top);
  }, OPEN_ANIMATION_MS);
}

async function pageOverflow(page: Page): Promise<number> {
  return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
}

// Mock build: MockSyncService approves the browser sign-in on a timer, and accepts
// any credentials on a server that cannot complete one.
test.describe('Sync connect modal', () => {
  async function openModal(page: Page, scenario = '') {
    // The sign-in URL goes through openExternal; recording it keeps the system
    // browser out of the run, and the short delay collapses the scripted wait.
    await page.addInitScript(() => {
      (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
      localStorage.setItem('tetiva.mockSignInMs', '200');
    });
    await page.goto(scenario ? `/?mock=${scenario}` : '/');
    await page.getByRole('button', { name: 'Sync', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    return dialog;
  }

  test('no server field by default, toggle reveals empty input', async ({ page }) => {
    const dialog = await openModal(page);

    await expect(dialog.getByTestId('signin-browser')).toBeVisible();
    await expect(dialog.locator('#server-url')).toHaveCount(0);

    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await expect(dialog.locator('#server-url')).toBeVisible();
    await expect(dialog.locator('#server-url')).toHaveValue('');
  });

  test('signing in without a custom server shows the cloud label', async ({ page }) => {
    const dialog = await openModal(page, 'signin-approve');

    await dialog.getByTestId('signin-browser').click();

    await expect(dialog.getByText('Tetiva Cloud')).toBeVisible();
    await expect(dialog.getByText('browser@example.com')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
  });

  // A custom server that cannot complete a browser sign-in is the only place the
  // email form is left.
  test('connect to a custom server shows the normalized raw address', async ({ page }) => {
    const dialog = await openModal(page, 'signin-unsupported');

    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await dialog.locator('#server-url').fill('https://sync.corp.local/');
    await dialog.locator('#login-email').fill('test@example.com');
    await dialog.locator('#login-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Connect', exact: true }).click();

    await expect(dialog.getByText('sync.corp.local:443')).toBeVisible();
    await expect(dialog.getByText('Tetiva Cloud')).toHaveCount(0);
  });

  test('keeps the frame still while the connected state loads', async ({ page }) => {
    const dialog = await openModal(page, 'signin-approve');
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('sync-device')).toHaveCount(3);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const tops = await trackDialogTop(page, () =>
      page.getByRole('button', { name: 'Sync', exact: true }).click());

    expect(tops.length).toBeGreaterThan(8);
    expect(Math.max(...tops) - Math.min(...tops)).toBeLessThanOrEqual(2);
    expect(await pageOverflow(page)).toBeLessThanOrEqual(0);
  });

  test('keeps the frame still with parked changes to report', async ({ page }) => {
    await page.goto('/?mock=parked');
    const dialog = page.getByRole('dialog');
    const tops = await trackDialogTop(page, () =>
      page.getByRole('button', { name: 'Sync', exact: true }).click());

    expect(tops.length).toBeGreaterThan(8);
    expect(Math.max(...tops) - Math.min(...tops)).toBeLessThanOrEqual(2);
    expect(await pageOverflow(page)).toBeLessThanOrEqual(0);
    expect(await dialog.evaluate(el => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(0);
    expect(await dialog.evaluate(el => window.innerHeight - el.getBoundingClientRect().bottom))
      .toBeGreaterThanOrEqual(0);
  });
});
