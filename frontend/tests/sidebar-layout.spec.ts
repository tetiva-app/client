import { test, expect, type Locator, type Page } from '@playwright/test';

const LEGACY_PANELS = '{"collapsedSize":0,"collapsible":true,"defaultSize":20,"maxSize":35,"minSize":15,"sizeUnit":"%"},'
  + '{"collapsible":false,"defaultSize":80,"sizeUnit":"%"}';

function sidebarWidth(page: Page) {
  return page.locator('[data-slot="resizable-panel"]').first()
    .evaluate((el) => Math.round(el.getBoundingClientRect().width));
}

function sidebarOffBy(page: Page, px: number) {
  return sidebarWidth(page).then((width) => Math.abs(width - px));
}

function savedSidebar(page: Page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('gophercourier.settings') ?? '{}').sidebar ?? null);
}

async function resizeWindow(page: Page, width: number) {
  await page.setViewportSize({ width, height: 800 });
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
}

function placeholderFits(input: Locator) {
  return input.evaluate((el: HTMLInputElement) => {
    const cs = getComputedStyle(el);
    const ctx = document.createElement('canvas').getContext('2d')!;
    ctx.font = `${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
    const box = el.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    return ctx.measureText(el.placeholder).width <= box;
  });
}

test.describe('Sidebar layout', () => {
  test('a narrow window keeps the search and the History selection readable', async ({ page }) => {
    await page.setViewportSize({ width: 856, height: 720 });
    await page.goto('/');
    const sidebar = page.locator('aside');

    expect(await placeholderFits(sidebar.getByPlaceholder('Search', { exact: true }))).toBe(true);

    await page.getByRole('button', { name: 'History', exact: true }).click();
    const records = page.locator('[data-history-item]');
    await records.nth(0).click({ modifiers: ['ControlOrMeta'] });
    await records.nth(1).click({ modifiers: ['ControlOrMeta'] });
    const count = sidebar.getByText('2 selected');
    await expect(count).toBeVisible();
    expect(await count.evaluate((el: HTMLElement) => el.scrollWidth <= el.clientWidth)).toBe(true);
  });

  test('the sidebar keeps its width when the app reopens in a smaller window', async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 900 });
    await page.goto('/');
    const panel = page.locator('[data-slot="resizable-panel"]').first();
    const handle = (await page.locator('[data-slot="resizable-handle"]').first().boundingBox())!;
    const start = (await panel.boundingBox())!.width;
    await page.mouse.move(handle.x, handle.y + 200);
    await page.mouse.down();
    await page.mouse.move(handle.x + 300 - start, handle.y + 200, { steps: 8 });
    await page.mouse.up();
    const chosen = (await panel.boundingBox())!.width;
    expect(Math.abs(chosen - 300)).toBeLessThan(2);

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.reload();
    await expect.poll(async () => Math.abs((await panel.boundingBox())!.width - chosen)).toBeLessThan(1);
  });

  test('a sidebar collapsed with Enter stays collapsed through a window resize', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/');
    const handle = page.locator('[data-slot="resizable-handle"]').first();
    await handle.press('Enter');
    await expect.poll(() => sidebarWidth(page)).toBe(0);
    expect(await savedSidebar(page)).toEqual({ width: 256, collapsed: true });

    await resizeWindow(page, 1100);
    expect(await sidebarWidth(page)).toBe(0);

    await handle.press('Enter');
    await expect.poll(() => sidebarOffBy(page, 256)).toBeLessThan(2);
    const box = (await handle.boundingBox())!;
    await page.mouse.move(box.x, box.y + 200);
    await page.mouse.down();
    await page.mouse.move(box.x + 40, box.y + 200, { steps: 6 });
    await page.mouse.up();
    expect(await sidebarOffBy(page, 296)).toBeLessThan(3);
    expect(await savedSidebar(page)).toEqual({ width: await sidebarWidth(page), collapsed: false });
  });

  test('expanding with Enter restores the saved width and holds it in a narrower window', async ({ page }) => {
    await page.addInitScript(() => {
      const settings = JSON.parse(localStorage.getItem('gophercourier.settings') ?? '{}');
      localStorage.setItem('gophercourier.settings', JSON.stringify({ ...settings, sidebar: { width: 300, collapsed: true } }));
    });
    await page.setViewportSize({ width: 1000, height: 800 });
    await page.goto('/');
    await expect.poll(() => sidebarWidth(page)).toBe(0);

    const handle = page.locator('[data-slot="resizable-handle"]').first();
    await handle.press('Enter');
    await expect.poll(() => sidebarOffBy(page, 300)).toBeLessThan(2);
    expect(await savedSidebar(page)).toEqual({ width: 300, collapsed: false });

    await resizeWindow(page, 950);
    expect(await sidebarOffBy(page, 300)).toBeLessThan(2);

    await handle.press('ArrowRight');
    await expect.poll(() => sidebarWidth(page)).toBeGreaterThan(305);
    expect(await savedSidebar(page)).toEqual({ width: await sidebarWidth(page), collapsed: false });
  });

  test('the sidebar width saved by the previous layout storage carries over', async ({ page }) => {
    await page.addInitScript((panels) => {
      localStorage.setItem('reka:main-layout', JSON.stringify({ [panels]: { expandToSizes: {}, layout: [25, 75] } }));
    }, LEGACY_PANELS);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/');

    await expect.poll(() => sidebarOffBy(page, 320)).toBeLessThan(2);
    expect(await savedSidebar(page)).toEqual({ width: 320, collapsed: false });
    expect(await page.evaluate(() => localStorage.getItem('reka:main-layout'))).toBeNull();
  });

  test('the request tabs stay on one line in a narrow window', async ({ page }) => {
    await page.setViewportSize({ width: 856, height: 720 });
    await page.goto('/');
    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('Collection name').fill('Orders API');
    await dialog.getByPlaceholder('Collection name').press('Enter');
    const col = page.locator('aside').getByText('Orders API');
    await col.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    await page.getByRole('dialog').getByPlaceholder('Request name').fill('List orders');
    await page.getByRole('dialog').getByPlaceholder('Request name').press('Enter');
    await page.locator('[aria-placeholder="Enter request URL"]').click();
    await page.keyboard.type('https://api.example.com/orders?page=1&limit=20');

    const tabs = page.locator('.border-b.border-border.px-3').first().locator('button');
    await expect(tabs.first()).toContainText('(2)');
    const lines = await tabs.evaluateAll((els) => els.map((el) => {
      const range = document.createRange();
      range.selectNodeContents(el);
      return new Set([...range.getClientRects()].map((r) => Math.round(r.top))).size;
    }));
    expect(lines).toEqual(lines.map(() => 1));
  });

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
