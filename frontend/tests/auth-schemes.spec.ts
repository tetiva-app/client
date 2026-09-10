import { test, expect, type Locator, type Page } from '@playwright/test';

const UNSUPPORTED_COLLECTION = JSON.stringify({
  info: { name: 'Legacy Auth', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
  auth: { type: 'hawk' },
  item: [
    { name: 'Ping', request: { method: 'GET', url: 'https://example.com/ping', auth: { type: 'ntlm' } } },
  ],
});

function editorTabs(page: Page): Locator {
  return page.locator('.border-b.border-border.px-3');
}

// The strip of open request/collection tabs above the editor.
function openTabs(page: Page): Locator {
  return page.locator('div.h-9.border-b.border-border').first();
}

async function createCollection(page: Page, name: string): Promise<Locator> {
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const item = page.locator('aside').getByText(name);
  await expect(item).toBeVisible();
  return item;
}

async function createRequest(page: Page, collection: Locator, name: string) {
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Request name').fill(name);
  await dialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
}

async function openAuthTab(page: Page) {
  await editorTabs(page).locator('button', { hasText: 'Auth' }).click();
  await expect(page.getByTestId('auth-type-selector')).toBeVisible();
}

async function selectAuthType(page: Page, label: string) {
  await page.getByTestId('auth-type-selector').click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
  await expect(page.getByTestId('auth-type-selector')).toContainText(label);
}

// The flow UI opens every URL through openExternal; this records it instead of
// launching the system browser, and stretches the mock flow when asked.
async function stubExternalOpener(page: Page, flowMs?: number) {
  await page.addInitScript((ms) => {
    (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
    if (ms !== undefined) localStorage.setItem('tetiva.mockFlowMs', String(ms));
  }, flowMs);
}

function lastExternalUrl(page: Page): Promise<string | undefined> {
  return page.evaluate(() => (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl);
}

async function selectGrant(page: Page, label: string) {
  await page.getByRole('combobox', { name: 'Grant Type' }).click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
}

async function newRequestOnAuthTab(page: Page, name: string) {
  await page.goto('/');
  const collection = await createCollection(page, name);
  await createRequest(page, collection, `${name} req`);
  await openAuthTab(page);
}

test.describe('Auth schemes', () => {
  test('offers every scheme and renders its form and badge', async ({ page }) => {
    await newRequestOnAuthTab(page, 'Schemes');

    await page.getByTestId('auth-type-selector').click();
    const listbox = page.locator('[role="listbox"]');
    for (const label of ['OAuth 2.0', 'JWT Bearer', 'Digest', 'AWS Signature']) {
      await expect(listbox.locator('[role="option"]', { hasText: label })).toBeVisible();
    }
    await page.keyboard.press('Escape');
    await page.locator('body').click({ position: { x: 5, y: 5 } });

    const authTab = editorTabs(page).locator('button', { hasText: 'Auth' });

    await selectAuthType(page, 'Digest');
    await expect(page.locator('[aria-placeholder="Username"]')).toBeVisible();
    await expect(page.locator('[aria-placeholder="Password"]')).toBeVisible();
    await expect(authTab).toContainText('(Digest)');

    await selectAuthType(page, 'AWS Signature');
    await expect(page.locator('[aria-placeholder="AKIA..."]')).toBeVisible();
    await expect(page.locator('[aria-placeholder="us-east-1"]')).toBeVisible();
    await expect(page.locator('[aria-placeholder="execute-api"]')).toBeVisible();
    await expect(authTab).toContainText('(AWS)');

    await selectAuthType(page, 'JWT Bearer');
    await expect(page.getByRole('combobox', { name: 'Algorithm' })).toContainText('HS256');
    await expect(page.getByText('Payload (claims)')).toBeVisible();
    await expect(authTab).toContainText('(JWT)');

    await selectAuthType(page, 'OAuth 2.0');
    await expect(page.getByRole('combobox', { name: 'Grant Type' })).toContainText('Client Credentials');
    await expect(page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]')).toBeVisible();
    await expect(authTab).toContainText('(OAuth2)');
  });

  test('keeps what was typed when the type is switched away and back', async ({ page }) => {
    await newRequestOnAuthTab(page, 'Drafts');

    await selectAuthType(page, 'Digest');
    await page.locator('[aria-placeholder="Username"]').fill('digest-user');

    await selectAuthType(page, 'AWS Signature');
    await page.locator('[aria-placeholder="us-east-1"]').fill('eu-west-1');
    await expect(page.locator('[aria-placeholder="Username"]')).toHaveCount(0);

    await selectAuthType(page, 'Digest');
    await expect(page.locator('[aria-placeholder="Username"]')).toContainText('digest-user');

    await selectAuthType(page, 'AWS Signature');
    await expect(page.locator('[aria-placeholder="us-east-1"]')).toContainText('eu-west-1');
  });

  test('Get token reports a validity window and Clear drops it', async ({ page }) => {
    await newRequestOnAuthTab(page, 'Tokens');
    await selectAuthType(page, 'OAuth 2.0');

    const status = page.getByTestId('oauth2-token-status');
    await expect(status).toHaveText('No token');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-token-error')).toContainText('Token URL and Client ID are required');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('http://127.0.0.1:9877/oauth2/token');
    await page.locator('[aria-placeholder="Client ID"]').fill('test-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(status).toContainText('Valid until');
    await expect(page.getByTestId('oauth2-token-error')).toHaveCount(0);

    await page.getByTestId('oauth2-clear-token').click();
    await expect(status).toHaveText('No token');
  });

  test('the authorization-code grant opens the browser and finishes', async ({ page }) => {
    await stubExternalOpener(page, 1500);
    await newRequestOnAuthTab(page, 'CodeFlow');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Authorization Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]').fill('https://idp.mock/authorize');
    await page.locator('[aria-placeholder="Client ID"]').fill('code-client');

    await page.getByTestId('oauth2-get-token').click();

    const panel = page.getByTestId('oauth2-flow-pending');
    await expect(panel).toBeVisible();
    await expect(panel).toHaveAttribute('data-authorize-url', /idp\.mock\/authorize/);
    await expect.poll(() => lastExternalUrl(page)).toContain('idp.mock/authorize');

    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until');
    await expect(panel).toHaveCount(0);
  });

  test('the device grant shows its user code and finishes', async ({ page }) => {
    await stubExternalOpener(page, 1500);
    await newRequestOnAuthTab(page, 'DeviceFlow');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();

    await expect(page.getByTestId('oauth2-user-code')).toHaveText('MOCK-CODE');
    await page.getByTestId('oauth2-verification-open').click();
    await expect.poll(() => lastExternalUrl(page)).toContain('idp.mock/device');

    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until');
  });

  test('Cancel drops the pending flow and re-enables the buttons', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'CancelFlow');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();

    await page.getByTestId('oauth2-flow-cancel').click();

    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-token-status')).toHaveText('No token');
    await expect(page.getByTestId('oauth2-get-token')).toBeEnabled();
    await expect(page.getByTestId('oauth2-clear-token')).toBeEnabled();
  });

  test('a pending flow survives a trip to the Params tab', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'KeepFlow');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-user-code')).toHaveText('MOCK-CODE');

    await editorTabs(page).locator('button', { hasText: 'Params' }).click();
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await openAuthTab(page);

    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();
    await expect(page.getByTestId('oauth2-user-code')).toHaveText('MOCK-CODE');
  });

  test('leaving OAuth 2.0 cancels the pending flow of a request', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'SwitchAway');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();

    await selectAuthType(page, 'Bearer Token');
    await selectAuthType(page, 'OAuth 2.0');

    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-get-token')).toBeEnabled();
  });

  test('leaving OAuth 2.0 cancels the pending flow of a collection', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await page.goto('/');
    const collection = await createCollection(page, 'Flow Col');
    await collection.dblclick();
    await page.getByRole('tab', { name: 'Authorization' }).click();

    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'OAuth 2.0' }).click();
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();

    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Bearer Token' }).click();
    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'OAuth 2.0' }).click();

    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-get-token')).toBeEnabled();
  });

  test('a pending flow survives a switch to another tab and back', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await page.goto('/');
    const collection = await createCollection(page, 'Two Tabs');
    await createRequest(page, collection, 'Flowing');
    await openAuthTab(page);
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-user-code')).toHaveText('MOCK-CODE');

    await createRequest(page, collection, 'Other');
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await openTabs(page).getByText('Flowing').click();

    await expect(page.getByTestId('oauth2-user-code')).toHaveText('MOCK-CODE');
  });

  test('an authorization-code grant without an authorize URL names the field', async ({ page }) => {
    await stubExternalOpener(page);
    await newRequestOnAuthTab(page, 'NoAuthUrl');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Authorization Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="Client ID"]').fill('code-client');

    await page.getByTestId('oauth2-get-token').click();

    await expect(page.getByTestId('oauth2-error-authUrl')).toHaveText('URL is required');
    // The message belongs to the input, not to the block around it.
    const authUrl = page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]');
    await expect(authUrl).toHaveAttribute('aria-invalid', 'true');
    const describedBy = await authUrl.getAttribute('aria-describedby');
    expect(describedBy).toBeTruthy();
    await expect(page.locator(`#${describedBy}`)).toHaveText('URL is required');
    await expect(page.getByTestId('oauth2-token-error')).toContainText('one field below needs attention');
    await expect(page.getByTestId('oauth2-token-error')).not.toContainText('authUrl');
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-get-token')).toBeEnabled();
    expect(await lastExternalUrl(page)).toBeUndefined();
  });

  test('a remote http authorize URL is refused on its own field', async ({ page }) => {
    await stubExternalOpener(page);
    await newRequestOnAuthTab(page, 'HttpAuthUrl');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Authorization Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]').fill('http://remote.example.com/authorize');
    await page.locator('[aria-placeholder="Client ID"]').fill('code-client');

    await page.getByTestId('oauth2-get-token').click();

    await expect(page.getByTestId('oauth2-error-authUrl')).toHaveText('http is allowed only for localhost; use https');
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
  });

  test('a redirect port outside the range is refused on its own field', async ({ page }) => {
    await stubExternalOpener(page);
    await newRequestOnAuthTab(page, 'BadPort');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Authorization Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]').fill('https://idp.mock/authorize');
    await page.locator('[aria-placeholder="Client ID"]').fill('code-client');
    await page.locator('[aria-placeholder="Redirect Port"]').fill('99999');

    await page.getByTestId('oauth2-get-token').click();

    await expect(page.getByTestId('oauth2-error-redirectPort'))
      .toHaveText('must be a port number between 0 and 65535; 0 picks a free one');
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
  });

  test('the authorization-code panel can reopen and copy its authorize URL', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'ReopenFlow');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Authorization Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]').fill('https://idp.mock/authorize');
    await page.locator('[aria-placeholder="Client ID"]').fill('code-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();

    await page.evaluate(() => { delete (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl; });
    await page.getByTestId('oauth2-authorize-open').click();
    await expect.poll(() => lastExternalUrl(page)).toContain('idp.mock/authorize');

    await page.getByTestId('oauth2-copy-authorize-url').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();
  });

  test('editing the configuration while a flow runs says the request was cancelled', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'EditWhilePending');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toBeVisible();

    await page.locator('[aria-placeholder="openid profile"]').fill('openid');

    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-flow-notice')).toContainText('the configuration changed');

    // A user action clears it; a background status read does not.
    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-flow-notice')).toHaveCount(0);
  });

  test('the keyboard keeps a button to act on through the whole flow', async ({ page }) => {
    await stubExternalOpener(page, 30_000);
    await newRequestOnAuthTab(page, 'FlowFocus');
    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill('https://idp.mock/device');
    await page.locator('[aria-placeholder="Client ID"]').fill('device-client');

    await page.getByTestId('oauth2-get-token').focus();
    await page.keyboard.press('Enter');

    await expect(page.getByTestId('oauth2-flow-cancel')).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-get-token')).toBeFocused();
  });

  test('Cmd+S saves a collection after a flow button took the click', async ({ page }) => {
    await stubExternalOpener(page);
    await page.goto('/');
    const collection = await createCollection(page, 'Save Col');
    await collection.dblclick();
    await page.getByRole('tab', { name: 'Authorization' }).click();

    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'OAuth 2.0' }).click();
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('https://idp.mock/oauth2/token');
    await page.locator('[aria-placeholder="Client ID"]').fill('col-client');

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until');
    await expect(page.getByTitle('Unsaved changes')).toBeVisible();

    await page.keyboard.press('Meta+s');

    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);
  });

  test('an inherit request names the owning collection and its token status', async ({ page }) => {
    await page.goto('/');
    const collection = await createCollection(page, 'Owner Col');

    await collection.dblclick();
    await page.getByRole('tab', { name: 'Authorization' }).click();

    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'OAuth 2.0' }).click();

    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill('http://127.0.0.1:9877/oauth2/token');
    await page.locator('[aria-placeholder="Client ID"]').fill('test-client');
    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until');

    // Chromium on macOS does not focus a button on click, so Cmd+S would land on
    // the body instead of the collection editor that listens for it.
    await page.locator('[aria-placeholder="Client ID"]').click();
    await page.keyboard.press('Meta+s');

    await createRequest(page, collection, 'Child');
    await openAuthTab(page);

    await expect(page.getByTestId('auth-type-selector')).toContainText('Inherit');
    await expect(page.getByTestId('inherited-auth')).toContainText('OAuth 2.0');
    await expect(page.getByTestId('inherited-auth-owner')).toContainText('Owner Col');
    await expect(page.getByTestId('inherited-token-status')).toContainText('Valid until');
  });

  test('the inherit block follows a change to the owning collection', async ({ page }) => {
    await page.goto('/');
    const collection = await createCollection(page, 'Stale Col');

    await collection.dblclick();
    await page.getByRole('tab', { name: 'Authorization' }).click();
    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Basic Auth' }).click();
    await page.locator('[aria-placeholder="Username"]').click();
    await page.keyboard.type('neo');
    await page.keyboard.press('Meta+s');

    await createRequest(page, collection, 'Child');
    await openAuthTab(page);
    await expect(page.getByTestId('inherited-auth')).toContainText('Basic Auth');

    // The block's own link is the signposted way to the owner, and the request
    // tab stays alive behind it.
    await page.getByTestId('inherited-auth-owner').click();
    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Bearer Token' }).click();
    await page.locator('[aria-placeholder="Paste your token here"]').click();
    await page.keyboard.type('t0ken');
    await page.keyboard.press('Meta+s');

    await openTabs(page).getByText('Child', { exact: true }).click();
    await expect(page.getByTestId('inherited-auth')).toContainText('Bearer Token');
  });

  test('every option of a long list stays inside the viewport and reachable', async ({ page }) => {
    // The editor pane is short and scrolls; a menu drawn inside it used to be
    // clipped, hiding most of the 13 JWT algorithms.
    await page.setViewportSize({ width: 1280, height: 800 });
    await newRequestOnAuthTab(page, 'Clipping');
    await selectAuthType(page, 'JWT Bearer');

    const algorithm = page.getByRole('combobox', { name: 'Algorithm' });
    await algorithm.click();
    const listbox = page.locator('[role="listbox"]');
    const box = await listbox.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.y + box!.height).toBeLessThanOrEqual(800);

    // The highlight follows the keyboard instead of running off the list.
    await algorithm.press('ArrowDown');
    await expect(algorithm).toHaveAttribute('aria-activedescendant', /option-1$/);
    await expect(listbox).toHaveAttribute('id', await algorithm.getAttribute('aria-controls') ?? '');

    // The menu scrolls on its own instead of being cut off by the editor pane,
    // and its last option answers a click where it is drawn.
    const menu = await listbox.evaluate(el => {
      const scrolls = el.scrollHeight > el.clientHeight;
      el.scrollTop = el.scrollHeight;
      const option = el.querySelector('[role="option"]:last-child') as HTMLElement;
      const rect = option.getBoundingClientRect();
      const at = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
      return {
        scrolls,
        clipped: el.closest('.overflow-auto') !== null,
        hits: at === option || option.contains(at),
        label: option.textContent?.trim(),
      };
    });
    expect(menu).toEqual({ scrolls: true, clipped: false, hits: true, label: 'EdDSA' });

    await listbox.locator('[role="option"]', { hasText: 'EdDSA' }).click();
    await expect(algorithm).toContainText('EdDSA');
  });

  test('an empty secret field shows its label, not a row of dots', async ({ page }) => {
    await newRequestOnAuthTab(page, 'Secrets');
    await selectAuthType(page, 'AWS Signature');

    const security = await page
      .locator('[aria-placeholder="Secret Access Key"] .cm-placeholder')
      .evaluate(el => getComputedStyle(el).getPropertyValue('-webkit-text-security'));
    expect(security).toBe('none');

    // The value itself stays masked.
    const content = page.locator('[aria-placeholder="Secret Access Key"]');
    await expect(content).toHaveAttribute('style', /text-security/);

    // The reveal toggle is a comfortable target, not a bare 14px icon.
    const eye = page.getByRole('button', { name: 'Show Secret Access Key' });
    const eyeBox = await eye.boundingBox();
    expect(eyeBox!.width).toBeGreaterThanOrEqual(24);
    expect(eyeBox!.height).toBeGreaterThanOrEqual(24);
  });

  test('an inherited HTTP-only scheme warns on a WebSocket request', async ({ page }) => {
    await page.goto('/');
    const collection = await createCollection(page, 'Digest Col');

    await collection.dblclick();
    await page.getByRole('tab', { name: 'Authorization' }).click();
    await page.getByTestId('collection-auth-type-selector').click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Digest' }).click();

    await page.locator('[aria-placeholder="Username"]').click();
    await page.keyboard.type('neo');
    await page.keyboard.press('Meta+s');

    await collection.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'WebSocket' }).click();
    await dialog.getByPlaceholder('Request name').fill('Socket');
    await dialog.getByPlaceholder('Request name').press('Enter');

    await page.getByRole('button', { name: 'Auth', exact: true }).click();
    await expect(page.getByTestId('inherited-auth')).toContainText('Digest');
    await expect(page.getByTestId('auth-http-only')).toContainText('supported for HTTP requests only');
  });

  test('an import reports the schemes it could not map', async ({ page }) => {
    await page.goto('/');

    const chooserPromise = page.waitForEvent('filechooser');
    await page.locator('button[title="Import Postman Collection"]').click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: 'legacy.postman_collection.json',
      mimeType: 'application/json',
      buffer: Buffer.from(UNSUPPORTED_COLLECTION),
    });

    await expect(page.getByText(/auth type "hawk" is not supported/)).toBeVisible();
    await expect(page.getByText(/auth type "ntlm" is not supported/)).toBeVisible();
  });
});
