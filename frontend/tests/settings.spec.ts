import { test, expect, type Page } from '@playwright/test';

const SECTIONS = ['interface', 'editor', 'publishing', 'mcp', 'updates', 'about'] as const;

async function openSettings(page: Page, size = { width: 1280, height: 800 }) {
  await page.setViewportSize(size);
  await page.addInitScript(() => {
    (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
  });
  await page.goto('/');
  await page.getByRole('button', { name: 'Settings' }).click();
  await expect(page.getByTestId('settings-dialog')).toBeVisible();
  // The dialog zooms in; a box measured mid-animation is scaled down.
  await page.getByTestId('settings-dialog').evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)));
}

async function contentOverflow(page: Page) {
  return page.getByTestId('settings-content').evaluate((el) => ({
    x: el.scrollWidth - el.clientWidth,
    y: el.scrollHeight - el.clientHeight,
  }));
}

test.describe('Settings dialog', () => {
  test('is a wide dialog with the section list on the left', async ({ page }) => {
    await openSettings(page);

    const size = await page.getByTestId('settings-dialog').evaluate((el) => {
      const r = el.getBoundingClientRect();
      return { width: r.width, height: r.height, computed: getComputedStyle(el).width };
    });
    expect(size.computed).toBe('880px');
    expect(size.width).toBeCloseTo(880, 0);
    expect(size.height).toBeCloseTo(620, 0);
    await expect(page.getByTestId('settings-nav-interface')).toHaveAttribute('aria-current', 'page');
    await expect(page.getByTestId('settings-section-interface')).toBeVisible();
  });

  for (const lang of ['English', 'Русский'] as const) {
    test(`every section fits a 1280×800 window without scrolling in ${lang}`, async ({ page }) => {
      await openSettings(page);
      await page.getByTestId(lang === 'English' ? 'settings-language-en' : 'settings-language-ru').click();

      for (const id of SECTIONS) {
        await page.getByTestId(`settings-nav-${id}`).click();
        await expect(page.getByTestId(`settings-section-${id}`)).toBeVisible();
        if (id === 'mcp') await expect(page.getByTestId('settings-mcp-scheme')).toBeVisible();
        expect(await contentOverflow(page), id).toEqual({ x: 0, y: 0 });
      }
    });

    test(`nothing overflows sideways at 960×640 in ${lang}`, async ({ page }) => {
      await openSettings(page, { width: 960, height: 640 });
      await page.getByTestId(lang === 'English' ? 'settings-language-en' : 'settings-language-ru').click();

      const box = (await page.getByTestId('settings-dialog').boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.y).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(960);
      expect(box.y + box.height).toBeLessThanOrEqual(640);
      for (const id of SECTIONS) {
        await page.getByTestId(`settings-nav-${id}`).click();
        await expect(page.getByTestId(`settings-section-${id}`)).toBeVisible();
        expect((await contentOverflow(page)).x, id).toBe(0);
      }
    });
  }

  test('switches the language live and keeps what was typed in the search', async ({ page }) => {
    await openSettings(page);
    const search = page.getByTestId('settings-search');
    await search.fill('язык');
    await expect(page.locator('[data-row="language"]')).toBeVisible();
    await expect(page.locator('[data-row="theme"]')).toBeHidden();

    await page.getByTestId('settings-language-ru').click();
    await expect(page.getByTestId('settings-nav-interface')).toContainText('Интерфейс');
    await expect(page.getByTestId('settings-dialog')).toContainText('Настройки');
    await expect(page.locator('html')).toHaveAttribute('lang', 'ru');
    await expect(search).toHaveValue('язык');
    await expect(page.locator('[data-row="language"]')).toContainText('Язык');

    await page.getByTestId('settings-language-en').click();
    await expect(page.getByTestId('settings-nav-interface')).toContainText('Interface');
    await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  });

  test('search finds a row by its English or Russian name and hides the rest', async ({ page }) => {
    await openSettings(page);
    const search = page.getByTestId('settings-search');

    await search.fill('token');
    await expect(page.getByTestId('settings-section-mcp')).toBeVisible();
    await expect(page.locator('[data-row="mcp-token"]')).toBeVisible();
    await expect(page.locator('[data-row="mcp-address"]')).toBeHidden();
    await expect(page.getByTestId('settings-nav-interface')).toHaveClass(/opacity-40/);
    await expect(page.getByTestId('settings-nav-mcp')).not.toHaveClass(/opacity-40/);

    await search.fill('токен');
    await expect(page.locator('[data-row="mcp-token"]')).toBeVisible();

    await search.fill('zzzz');
    await expect(page.getByTestId('settings-search-empty')).toContainText('Nothing found for “zzzz”');
    await search.fill('$&');
    await expect(page.getByTestId('settings-search-empty')).toContainText('Nothing found for “$&”');

    await search.press('Escape');
    await expect(search).toHaveValue('');
    await expect(page.getByTestId('settings-dialog')).toBeVisible();
    await expect(page.getByTestId('settings-section-mcp')).toBeVisible();
    await expect(page.locator('[data-row="mcp-address"]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.getByTestId('settings-dialog')).toHaveCount(0);
  });

  test('words a rejected MCP address in the app language', async ({ page }) => {
    await openSettings(page);
    await page.getByTestId('settings-language-ru').click();
    await page.getByTestId('settings-nav-mcp').click();

    const addr = page.locator('[data-row="mcp-address"] input');
    await addr.fill('127.0.0.1:99999');
    await addr.blur();
    await expect(page.getByTestId('settings-mcp-error'))
      .toHaveText('Укажите адрес как хост:порт, порт от 1 до 65535, например 127.0.0.1:9300.');
  });

  test('reopens on the section it was closed on', async ({ page }) => {
    await openSettings(page);
    await page.getByTestId('settings-nav-updates').click();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('settings-dialog')).toHaveCount(0);

    await page.getByRole('button', { name: 'Settings' }).click();
    await expect(page.getByTestId('settings-section-updates')).toBeVisible();
  });

  test('turning publishing off marks the section and the cabinet link opens the published pages', async ({ page }) => {
    await openSettings(page);
    await page.getByTestId('settings-nav-publishing').click();

    await page.getByTestId('settings-publishing-switch').click();
    await expect(page.getByTestId('settings-nav-publishing')).toContainText('off');
    const stored = await page.evaluate(() => JSON.parse(localStorage.getItem('gophercourier.settings') ?? '{}'));
    expect(stored.publishingEnabled).toBe(false);

    await page.getByTestId('settings-cabinet-link').click();
    const opened = await page.evaluate(() => (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl);
    expect(opened).toBe('https://app.tetiva.app/app/published');
  });

  test('Show welcome screen closes Settings and replays the welcome', async ({ page }) => {
    await openSettings(page);
    await page.getByTestId('settings-nav-about').click();
    await page.getByTestId('show-welcome').click();

    await expect(page.getByTestId('settings-dialog')).toHaveCount(0);
    await expect(page.getByTestId('onboarding-modal')).toBeVisible();
  });
});
