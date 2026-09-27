import { test, expect, type Page } from '@playwright/test';

type Protocol = 'HTTP' | 'GraphQL' | 'gRPC' | 'WebSocket';

const SNIPPETS_BUNDLE = /\/snippets\/dist\/snippets\.mjs/;

const urlField = (page: Page, protocol: Protocol) => ({
  HTTP: page.locator('[aria-placeholder="Enter request URL"]'),
  GraphQL: page.getByPlaceholder('https://api.example.com/graphql'),
  gRPC: page.getByPlaceholder('host:port'),
  WebSocket: page.getByPlaceholder('ws://… or wss://…'),
})[protocol];

async function createRequest(page: Page, url: string, protocol: Protocol = 'HTTP') {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Code Collection');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collectionItem = page.locator('aside').getByText('Code Collection');
  await expect(collectionItem).toBeVisible();
  await collectionItem.click();
  await collectionItem.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: protocol, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Code Request');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await urlField(page, protocol).fill(url);
}

const moreActions = (page: Page) => page.getByRole('button', { name: 'More actions' });
const menuItem = (page: Page, name: string) => page.getByRole('menuitem', { name, exact: true });
const codeDialog = (page: Page) => page.getByRole('dialog', { name: 'Generate code' });
const generatedCode = (page: Page) => page.getByRole('textbox', { name: 'Generated code' });
const runButtons = (page: Page) => moreActions(page).locator('..');
const codeTab = (page: Page) => page.getByRole('button', { name: /^Docs\b/ }).locator('..').getByRole('button', { name: /^Code\b/ });

async function openGenerateCode(page: Page, url: string) {
  await createRequest(page, url);
  await moreActions(page).click();
  await menuItem(page, 'Generate code…').click();
  await expect(codeDialog(page)).toBeVisible();
}

async function copyAsFromSubmenu(page: Page, label: string) {
  await moreActions(page).click();
  await menuItem(page, 'Copy as').hover();
  await menuItem(page, label).click();
}

test.describe('Copy as menu', () => {
  test('offers cURL, the other languages and Generate code; the request has no Code tab', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/users');
    await expect(codeTab(page)).toHaveCount(0);

    await moreActions(page).click();
    await expect(page.getByRole('menu').first().getByRole('menuitem')).toHaveText(['Copy as cURL', 'Copy as', 'Generate code…']);

    await menuItem(page, 'Copy as').hover();
    await expect(page.getByRole('menu').nth(1).getByRole('menuitem')).toHaveText(
      ['Python', 'JavaScript', 'Go', 'Java (HttpClient)', 'Java (OkHttp)', 'C#', 'PHP'],
    );
  });

  test('copies as cURL with the unsaved URL', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await createRequest(page, 'https://api.example.com/unsaved');

    await moreActions(page).click();
    await menuItem(page, 'Copy as cURL').click();

    await expect(page.getByText('Copied as cURL', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain("'https://api.example.com/unsaved'");
  });

  test('copies as Python in one click and Generate code opens on Python', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await createRequest(page, 'https://api.example.com/users');

    await copyAsFromSubmenu(page, 'Python');

    await expect(page.getByText('Copied as Python', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('import requests');

    await moreActions(page).click();
    await menuItem(page, 'Generate code…').click();
    await expect(codeDialog(page).getByRole('combobox', { name: 'Language' })).toHaveText('Python');
  });

  test('counts the warnings and opens Details on the copied language', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await createRequest(page, 'https://api.example.com/{{a b}}');

    await copyAsFromSubmenu(page, 'Go');

    const toast = page.getByText(/^Copied as Go · \d+ warnings?$/);
    await expect(toast).toBeVisible();
    await page.getByRole('button', { name: 'Details' }).click();

    await expect(codeDialog(page).getByRole('combobox', { name: 'Language' })).toHaveText('Go');
    await expect(page.getByRole('list', { name: 'Snippet warnings' })).toContainText('Variable "a b" contains unsafe characters');
  });

  test('goes away when Close Tab switches to another tab under it', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/users');
    await page.locator('aside').getByText('Code Collection').click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    await page.getByRole('dialog').getByPlaceholder('Request name').fill('Second Request');
    await page.getByRole('dialog').getByPlaceholder('Request name').press('Enter');
    await expect(page.getByRole('dialog')).toHaveCount(0);

    await moreActions(page).click();
    await expect(page.getByRole('menu')).toBeVisible();
    // The native accelerator's app:close-tab handler calls this store action directly.
    await page.evaluate(() => {
      const app = (document.querySelector('#app') as any).__vue_app__;
      const store = app.config.globalProperties.$pinia._s.get('requests');
      store.closeTab(store.activeTabId);
    });

    await expect(page.getByRole('menu')).toHaveCount(0);
    await expect(page.locator('body')).not.toHaveCSS('pointer-events', 'none');
  });
});

const OTHER_PROTOCOLS = {
  GraphQL: {
    url: 'https://api.example.com/graphql',
    top: ['Copy as cURL', 'Copy as', 'Generate code…'],
    submenu: ['Python', 'JavaScript', 'Go', 'Java (HttpClient)', 'Java (OkHttp)', 'C#', 'PHP'],
    language: 'cURL',
    copy: { item: 'Copy as cURL', toast: /^Copied as cURL$/, code: "'https://api.example.com/graphql'" },
  },
  gRPC: {
    url: 'localhost:50051',
    top: ['Copy as gRPCurl', 'Generate code…'],
    submenu: [],
    language: 'gRPCurl',
    copy: { item: 'Copy as gRPCurl', toast: /^Copied as gRPCurl · \d+ warnings?$/, code: 'grpcurl -plaintext' },
  },
  WebSocket: {
    url: 'wss://echo.example.com/socket',
    top: ['Copy as websocat', 'Copy as JavaScript', 'Generate code…'],
    submenu: [],
    language: 'websocat',
    copy: { item: 'Copy as websocat', toast: /^Copied as websocat( · \d+ warnings?)?$/, code: 'wss://echo.example.com/socket' },
  },
} as const;

for (const [protocol, spec] of Object.entries(OTHER_PROTOCOLS) as [Protocol, (typeof OTHER_PROTOCOLS)[keyof typeof OTHER_PROTOCOLS]][]) {
  test.describe(`Copy as menu on ${protocol}`, () => {
    test('offers its languages and Generate code; the request has no Code tab', async ({ page }) => {
      await createRequest(page, spec.url, protocol);
      await expect(codeTab(page)).toHaveCount(0);

      await moreActions(page).click();
      await expect(page.getByRole('menu').first().getByRole('menuitem')).toHaveText([...spec.top]);
      if (spec.submenu.length) {
        await menuItem(page, 'Copy as').hover();
        await expect(page.getByRole('menu').nth(1).getByRole('menuitem')).toHaveText([...spec.submenu]);
      }

      await menuItem(page, 'Generate code…').click();
      await expect(codeDialog(page).getByRole('combobox', { name: 'Language' })).toHaveText(spec.language);
    });

    test('copies in one click', async ({ page, context }) => {
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);
      await createRequest(page, spec.url, protocol);

      await moreActions(page).click();
      await menuItem(page, spec.copy.item).click();

      await expect(page.getByText(spec.copy.toast)).toBeVisible();
      expect(await page.evaluate(() => navigator.clipboard.readText())).toContain(spec.copy.code);
    });
  });
}

// The mock answers a URL containing "slow" after 3 s.
test.describe('Cancel next to the menu', () => {
  test('stops a slow GraphQL query and drops its late answer', async ({ page }) => {
    await createRequest(page, 'https://slow.example.com/graphql', 'GraphQL');
    const idle = page.getByText(/to execute a GraphQL request/);

    await runButtons(page).getByRole('button', { name: 'Query', exact: true }).click();
    await runButtons(page).getByRole('button', { name: 'Cancel' }).click();

    await expect(runButtons(page).getByRole('button', { name: 'Query', exact: true })).toBeVisible();
    await expect(idle).toBeVisible();
    await page.waitForTimeout(3500);
    await expect(idle).toBeVisible();
  });

  test('stops a slow gRPC call and drops its late answer', async ({ page }) => {
    await createRequest(page, 'slow.example.com:443', 'gRPC');
    await page.getByRole('button', { name: 'Load services' }).click();
    await page.getByRole('button', { name: 'Select method...' }).click();
    await page.getByRole('button', { name: 'UNARY GetUser' }).click();
    const idle = page.getByText(/to send a gRPC request/);

    await runButtons(page).getByRole('button', { name: 'Invoke', exact: true }).click();
    await runButtons(page).getByRole('button', { name: 'Cancel' }).click();

    await expect(runButtons(page).getByRole('button', { name: 'Invoke', exact: true })).toBeVisible();
    await expect(idle).toBeVisible();
    await page.waitForTimeout(3500);
    await expect(idle).toBeVisible();
  });
});

test.describe('Generate code dialog', () => {
  test('shows the unsaved request as cURL and follows later edits', async ({ page }) => {
    await openGenerateCode(page, 'https://api.example.com/users');
    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/users'");
    await expect(page.getByRole('button', { name: 'Copy', exact: true })).toBeEnabled();

    await page.keyboard.press('Escape');
    await expect(codeDialog(page)).toHaveCount(0);
    await urlField(page, 'HTTP').fill('https://api.example.com/orders');
    await moreActions(page).click();
    await menuItem(page, 'Generate code…').click();

    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/orders'");
  });

  test('switches the language and highlights the new code', async ({ page }) => {
    await openGenerateCode(page, 'https://api.example.com/users');
    await expect(generatedCode(page)).toContainText('curl');

    await page.getByRole('combobox', { name: 'Language' }).click();
    await page.getByRole('listbox').getByRole('option', { name: 'Python' }).click();

    await expect(page.getByRole('combobox', { name: 'Language' })).toHaveText('Python');
    await expect(generatedCode(page)).toContainText('import requests');
    await expect(generatedCode(page).locator('span', { hasText: /^import$/ })).toBeVisible();
  });

  test('picks the language from the keyboard', async ({ page }) => {
    await openGenerateCode(page, 'https://api.example.com/users');
    const language = page.getByRole('combobox', { name: 'Language' });
    await expect(generatedCode(page)).toContainText('curl');

    await language.focus();
    await page.keyboard.press('Enter');
    const options = page.getByRole('listbox').getByRole('option');
    await expect(options).toHaveText(['cURL', 'Python', 'JavaScript', 'Go', 'Java (HttpClient)', 'Java (OkHttp)', 'C#', 'PHP']);
    await expect(options.first()).toBeFocused();
    for (const next of [1, 2, 3]) {
      await page.keyboard.press('ArrowDown');
      await expect(options.nth(next)).toBeFocused();
    }
    await page.keyboard.press('Enter');

    await expect(page.getByRole('listbox')).toHaveCount(0);
    await expect(language).toHaveText('Go');
    await expect(language).toBeFocused();
    await expect(generatedCode(page)).toContainText('package main');
  });

  test('offers secret values only while variables are resolved', async ({ page }) => {
    await openGenerateCode(page, 'https://api.example.com/users');
    const resolve = page.getByRole('switch', { name: 'Resolve variables' });
    const secrets = page.getByRole('switch', { name: 'Include secret values' });

    await expect(resolve).toBeChecked();
    await expect(secrets).toBeEnabled();
    await expect(secrets).not.toBeChecked();

    await secrets.click();
    await expect(secrets).toBeChecked();

    await resolve.click();
    await expect(secrets).toBeDisabled();
    await expect(secrets).not.toBeChecked();

    await resolve.click();
    await expect(secrets).toBeEnabled();
    await expect(secrets).not.toBeChecked();
  });

  test('announces warnings politely', async ({ page }) => {
    await openGenerateCode(page, 'https://api.example.com/{{a b}}');
    const warnings = page.getByRole('list', { name: 'Snippet warnings' });

    await expect(warnings).toHaveAttribute('aria-live', 'polite');
    await expect(warnings).toContainText('Variable "a b" contains unsafe characters and is shown as VAR_0');
  });

  test('copies the code', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await openGenerateCode(page, 'https://api.example.com/users');
    await expect(generatedCode(page)).toContainText("curl 'https://api.example.com/users'");

    await page.getByRole('button', { name: 'Copy', exact: true }).click();

    await expect(page.getByText('Copied', { exact: true })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("curl 'https://api.example.com/users'");
  });

  test('reports a snippet library that fails to load as an alert', async ({ page }) => {
    await page.route(SNIPPETS_BUNDLE, (route) => route.abort());
    await openGenerateCode(page, 'https://api.example.com/users');

    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Copy', exact: true })).toBeDisabled();
  });

  test('marks the code busy while it is being generated', async ({ page }) => {
    let release!: () => void;
    const released = new Promise<void>((resolve) => { release = resolve; });
    await page.route(SNIPPETS_BUNDLE, async (route) => {
      await released;
      await route.continue();
    });
    await openGenerateCode(page, 'https://api.example.com/users');

    const status = page.getByRole('status').filter({ hasText: 'Generating code' });
    await expect(status).toBeVisible();
    await expect(page.locator('[aria-busy="true"]').getByRole('textbox', { name: 'Generated code' })).toBeVisible();

    release();

    await expect(status).toHaveCount(0);
    await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
    await expect(generatedCode(page)).toContainText('curl');
  });
});
