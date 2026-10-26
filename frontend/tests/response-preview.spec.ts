import { test, expect, type Page } from '@playwright/test';

async function send(page: Page, url: string, html?: string) {
  await page.goto('/');
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Preview');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const col = page.locator('aside').getByText('Preview');
  await col.click();
  await col.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByPlaceholder('Request name').fill('Req');
  await reqDialog.getByPlaceholder('Request name').press('Enter');

  if (html !== undefined) await serveHtml(page, html);
  await page.locator('[aria-placeholder="Enter request URL"]').fill(url);
  await page.locator('button', { hasText: 'Send' }).click();
  await expect(page.getByText('200').first()).toBeVisible({ timeout: 5000 });
}

async function serveHtml(page: Page, html: string) {
  await page.evaluate(async (page) => {
    const { getRequestService } = await import('/src/services/index.ts');
    const service = await getRequestService();
    const execute = service.execute.bind(service);
    service.execute = async (req) => {
      const res = await execute(req);
      return { ...res, data: { ...res.data, headers: { 'Content-Type': ['text/html'] }, body: page, size: page.length } };
    };
  }, html);
}

async function previewDocs(page: Page, pages: string[]): Promise<string[]> {
  await page.goto('/');
  return page.evaluate(async (pages) => {
    const { htmlPreviewDoc } = await import('/src/lib/response-preview.ts');
    return pages.map(htmlPreviewDoc);
  }, pages);
}

const POLICY_FIRST = '<!doctype html><html><head>'
  + `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`
  + '<meta http-equiv="x-dns-prefetch-control" content="off">';

const HOSTILE_LINKS = '<a id="self" target="_self" href="https://attacker.test/a">self link</a>'
  + '<svg width="60" height="20"><a id="svg" xlink:href="https://attacker.test/s"><rect width="60" height="20"/></a></svg>'
  + '<div><template shadowrootmode="open"><a id="shadow" target="_self" href="https://attacker.test/t">shadow link</a></template></div>';

const body = (page: Page) => page.locator('[data-response-body]');
const toggle = (page: Page) => page.getByTestId('body-view-toggle');

test.describe('Response preview', () => {
  test('shows an image response as a picture with its size', async ({ page }) => {
    await send(page, 'https://api.example.com/binary');

    const img = body(page).getByTestId('image-preview').locator('img');
    await expect(img).toBeVisible();
    await expect(img).toHaveAttribute('src', /^blob:/);
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBe(160);
    await expect(body(page)).toContainText('PNG');
    await expect(body(page)).toContainText('160 × 120 px');
    await expect(body(page)).toContainText('157 B');
    await expect(body(page).getByRole('button', { name: 'Save to file' })).toBeVisible();
  });

  test('falls back to Save to file when the image is over the limit', async ({ page }) => {
    await send(page, 'https://cdn.example.com/huge.png');

    await expect(body(page)).toContainText('Binary response');
    await expect(body(page)).toContainText('Image is too large to preview (over 10 MB)');
    await expect(body(page).getByRole('button', { name: 'Save to file' })).toBeVisible();
    await expect(body(page).locator('img')).toHaveCount(0);
  });

  test('draws SVG through an img so its script stays dead, with the source a click away', async ({ page }) => {
    await send(page, 'https://api.example.com/logo.svg');

    const img = body(page).locator('img');
    await expect(img).toHaveAttribute('src', /^blob:/);
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBe(96);
    expect(await page.title()).not.toBe('svg script ran');
    await expect(body(page).locator('svg')).toHaveCount(0);

    await toggle(page).getByRole('button', { name: 'Source' }).click();
    await expect(body(page).locator('.cm-content')).toContainText('<circle');
  });

  test('renders HTML only on request, with scripts and network cut off', async ({ page }) => {
    const reachedNetwork: string[] = [];
    await page.route(/tracker\.example\.com|shop\.example\.com/, (route) => {
      reachedNetwork.push(route.request().url());
      return route.abort();
    });

    await send(page, 'https://api.example.com/receipt.html');

    await expect(toggle(page).getByRole('button', { name: 'Source' })).toHaveAttribute('aria-pressed', 'true');
    await expect(body(page).locator('.cm-content')).toContainText('Order #1042');
    await expect(body(page).locator('iframe')).toHaveCount(0);

    await toggle(page).getByRole('button', { name: 'Preview' }).click();

    const frame = body(page).getByTestId('html-preview');
    await expect(frame).toHaveAttribute('sandbox', '');
    await expect(frame).toHaveAttribute('srcdoc', /Content-Security-Policy/);
    const doc = page.frameLocator('[data-testid="html-preview"]');
    await expect(doc.locator('#title')).toHaveText('Order #1042 confirmed');
    await doc.getByText('View order').click();
    await expect(doc.locator('#title')).toHaveText('Order #1042 confirmed');
    await expect(page.getByTestId('response-copy-body')).toBeVisible();
    expect(reachedNetwork).toEqual([]);
  });

  test('puts the policy first in head however the page starts', async ({ page }) => {
    const [nbsp, junk] = await previewDocs(page, [
      '\u00a0<!doctype html><img src="https://attacker.test/p">',
      'oops <!DOCTYPE html><html><head><meta http-equiv="refresh" content="0;url=https://attacker.test/r">'
        + '<base href="https://attacker.test/"><link rel="preconnect" href="https://attacker.test"></head>'
        + `<body>${HOSTILE_LINKS}<map><area href="https://attacker.test/m"></map></body></html>`,
    ]);

    expect(nbsp.startsWith(POLICY_FIRST)).toBe(true);
    expect(nbsp).toContain('<img src="https://attacker.test/p">');
    expect(junk.startsWith(POLICY_FIRST)).toBe(true);
    expect(junk).toContain('oops');
    expect(junk).toContain('self link');
    expect(junk).toContain('shadow link');
    expect(junk).not.toContain('attacker.test');
  });

  test('keeps a hostile page off the network, links included', async ({ page }) => {
    const reachedNetwork: string[] = [];
    await page.route(/attacker\.test/, (route) => {
      reachedNetwork.push(route.request().url());
      return route.abort();
    });

    await send(page, 'https://api.example.com/page', `\u00a0<!doctype html><img src="https://attacker.test/p">${HOSTILE_LINKS}`);
    await toggle(page).getByRole('button', { name: 'Preview' }).click();

    const doc = page.frameLocator('[data-testid="html-preview"]');
    await doc.locator('#self').click();
    await doc.locator('#svg').click();
    await doc.locator('#shadow').click();
    await expect(doc.locator('#self')).toHaveText('self link');
    expect(reachedNetwork).toEqual([]);
  });

  test('shows the response again when a click still navigates the preview away', async ({ page }) => {
    await page.route(/attacker\.test/, (route) => route.fulfill({ contentType: 'text/html', body: '<p id="remote">remote page</p>' }));

    await send(page, 'https://api.example.com/page',
      '<p id="order">Order #1042</p><svg width="80" height="30"><a id="anim"><text y="20">Click</text>'
      + '<set attributeName="href" to="https://attacker.test/x"/></a></svg>');
    await toggle(page).getByRole('button', { name: 'Preview' }).click();

    const doc = page.frameLocator('[data-testid="html-preview"]');
    await doc.locator('#anim').click();
    await expect(doc.locator('#order')).toHaveText('Order #1042');
    await expect(doc.locator('#remote')).toHaveCount(0);
  });
});
