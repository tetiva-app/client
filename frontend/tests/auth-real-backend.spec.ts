import { test, expect, type Locator, type Page } from '@playwright/test';

// Runs against the Go backend, not the Vite mock build, so all three processes
// are started by hand before the run:
//   cd client/frontend && npm run build
//   cd client && export PATH="$HOME/go/bin:$PATH" && go build -tags server -o bin/client-server .
//   DATA=$(mktemp -d) && TETIVA_DATA_DIR=$DATA ./bin/client-server &
//   go run ./cmd/authmock &
//   cd frontend && TETIVA_REAL_BACKEND=1 npx playwright test --project=real-backend
const ENABLED = process.env.TETIVA_REAL_BACKEND === '1';

const RESOURCE = 'http://127.0.0.1:9877/api/whoami';
// 33 s is just over the provider's 30 s refresh skew, so the first send uses the
// token fetched by hand and a send a few seconds later has to refresh it.
const TOKEN_URL = 'http://127.0.0.1:9877/oauth2/token?expires_in=33';
const FLOW_TOKEN_URL = 'http://127.0.0.1:9877/oauth2/token';
const AUTHORIZE_URL = 'http://127.0.0.1:9877/oauth2/authorize';
const DEVICE_URL = 'http://127.0.0.1:9877/oauth2/device';

// One backend, one SQLite file and one fixed loopback port for the whole file,
// so nothing here may run beside anything else.
test.describe.configure({ mode: 'serial' });

function editorTabs(page: Page): Locator {
  return page.locator('.border-b.border-border.px-3').first();
}

function responseBody(page: Page): Locator {
  return page.locator('[data-response-body]');
}

function historyRows(page: Page): Locator {
  return page.locator('aside div.group').filter({ hasText: '/api/whoami' });
}

async function createRequest(page: Page, collection: Locator, name: string) {
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Request name').fill(name);
  await dialog.getByPlaceholder('Request name').press('Enter');

  const url = page.locator('[aria-placeholder="Enter request URL"]');
  await expect(url).toBeVisible();
  // Creating a request emits collection:updated, and the refetch it triggers
  // replaces the editor buffer — typing before it lands loses the URL.
  await page.waitForTimeout(1_000);
  await url.fill(RESOURCE);
  await expect(url).toContainText(RESOURCE);
  await editorTabs(page).locator('button', { hasText: 'Auth' }).click();
}

async function selectAuthType(page: Page, label: string) {
  await page.getByTestId('auth-type-selector').click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
  await expect(page.getByTestId('auth-type-selector')).toContainText(label);
}

async function send(page: Page) {
  await page.locator('button', { hasText: 'Send' }).first().click();
}

async function createCollection(page: Page, name: string): Promise<Locator> {
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');
  const item = page.locator('aside').getByText(name).first();
  await expect(item).toBeVisible();

  return item;
}

// The server build injects no Wails runtime into the HTML, so the query flag is
// what makes the frontend pick the real services over the mocks.
async function openApp(page: Page) {
  const mockMarkers: string[] = [];
  page.on('console', (m) => {
    if (m.text().includes('Running in browser mode with mock services')) mockMarkers.push(m.text());
  });
  await page.goto('/?wails=1');
  await expect(page.getByTitle('New Collection')).toBeVisible();
  expect(mockMarkers).toEqual([]);
}

// Records what the app would open instead of launching the system browser: a
// real launch would consume the one-shot callback outside the test's reach.
async function noSystemBrowser(page: Page) {
  await page.addInitScript(() => {
    (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
  });
}

function lastExternalUrl(page: Page): Promise<string | undefined> {
  return page.evaluate(() => (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl);
}

async function selectGrant(page: Page, label: string) {
  await page.getByRole('combobox', { name: 'Grant Type' }).click();
  await page.locator('[role="listbox"] [role="option"]', { hasText: label }).click();
}

// Chromium on macOS does not focus a button on click, so a field is focused
// first or Cmd+S lands on the body.
async function save(page: Page) {
  await page.locator('[aria-placeholder="Client ID"]').click();
  await page.keyboard.press('Meta+s');
}

async function fillCodeGrant(page: Page) {
  await selectAuthType(page, 'OAuth 2.0');
  await selectGrant(page, 'Authorization Code');
  await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill(FLOW_TOKEN_URL);
  await page.locator('[aria-placeholder="https://idp.example.com/oauth2/authorize"]').fill(AUTHORIZE_URL);
  await page.locator('[aria-placeholder="Client ID"]').fill('test-client');
  await page.locator('[aria-placeholder="Client Secret"]').fill('test-secret');
  await save(page);
}

async function startFlow(page: Page): Promise<string> {
  await page.getByTestId('oauth2-get-token').click();
  const panel = page.getByTestId('oauth2-flow-pending');
  await expect(panel).toBeVisible({ timeout: 20_000 });
  const url = await panel.getAttribute('data-authorize-url');
  expect(url).toContain('code_challenge_method=S256');

  return url as string;
}

// Walks the authorize URL in a second tab of the same context; the mock's page
// redirects the browser to the loopback listener, which answers the flow.
async function visitInBrowser(page: Page, url: string, expected: RegExp) {
  const tab = await page.context().newPage();
  await tab.goto(url);
  await expect(tab.locator('body')).toContainText(expected, { timeout: 15_000 });
  await tab.close();
}

async function reopenOnAuthTab(page: Page, collectionName: string, requestName: string) {
  const collection = page.locator('aside').getByText(collectionName).first();
  await expect(collection).toBeVisible();
  const request = page.locator('aside').getByText(requestName).first();
  if (!(await request.isVisible())) await collection.click();
  await expect(request).toBeVisible();
  await request.click();
  await expect(page.locator('[aria-placeholder="Enter request URL"]')).toBeVisible();
  await editorTabs(page).locator('button', { hasText: 'Auth' }).click();
  await expect(page.getByTestId('auth-type-selector')).toBeVisible();
}

test.describe('Auth schemes against the real backend', () => {
  test.skip(!ENABLED, 'needs TETIVA_REAL_BACKEND=1 plus bin/client-server on :8080 and cmd/authmock on :9877');

  test('digest, JWT, SigV4 and OAuth 2.0 reach the mock resource', async ({ page }) => {
    test.setTimeout(180_000);

    await openApp(page);
    const collection = await createCollection(page, 'Auth Real');

    // The history count is taken before anything is sent, because the data
    // directory survives a re-run. Sidebar sections are switched only while no
    // request holds unsaved edits: remounting the tree refetches over them.
    const historyButton = page.getByRole('button', { name: 'History', exact: true });
    const collectionsButton = page.getByRole('button', { name: 'Collections', exact: true });
    await historyButton.click();
    const before = await historyRows(page).count();
    await collectionsButton.click();

    await createRequest(page, collection, 'Digest');
    await selectAuthType(page, 'Digest');
    await page.locator('[aria-placeholder="Username"]').fill('digest-user');
    await page.locator('[aria-placeholder="Password"]').fill('digest-password');
    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "digest"', { timeout: 20_000 });
    await expect(responseBody(page)).toContainText('"algorithm": "SHA-256"');

    await createRequest(page, collection, 'JWT');
    await selectAuthType(page, 'JWT Bearer');
    await page.locator('[aria-placeholder="Secret"]').fill('authmock-hs256-secret');
    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "jwt"', { timeout: 20_000 });

    await createRequest(page, collection, 'SigV4');
    await selectAuthType(page, 'AWS Signature');
    await page.locator('[aria-placeholder="AKIA..."]').fill('AKIDEXAMPLE');
    await page.locator('[aria-placeholder="Secret Access Key"]').fill('wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY');
    await page.locator('[aria-placeholder="us-east-1"]').fill('us-east-1');
    await page.locator('[aria-placeholder="execute-api"]').fill('execute-api');
    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "aws_sigv4"', { timeout: 20_000 });

    await createRequest(page, collection, 'OAuth2');
    await selectAuthType(page, 'OAuth 2.0');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill(TOKEN_URL);
    await page.locator('[aria-placeholder="Client ID"]').fill('test-client');
    await page.locator('[aria-placeholder="Client Secret"]').fill('test-secret');

    // Save before minting a token: a later save with changed acquisition fields
    // drops it on purpose.
    await save(page);

    await page.getByTestId('oauth2-get-token').click();
    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until', { timeout: 20_000 });

    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "oauth2"', { timeout: 20_000 });
    await expect(responseBody(page)).toContainText('"grant": "client_credentials"');

    // Inside the refresh skew the send has to renew the token by itself.
    await page.waitForTimeout(5_000);
    await send(page);
    await expect(responseBody(page)).toContainText('"grant": "refresh_token"', { timeout: 20_000 });

    // Five sends, five rows: the digest challenge round trip is not a response
    // of its own, so no 401 was recorded.
    await historyButton.click();
    await expect(historyRows(page)).toHaveCount(before + 5);
    await expect(page.locator('aside div.group').filter({ hasText: '401' })).toHaveCount(0);
  });
});

test.describe('OAuth 2.0 browser flows against the real backend', () => {
  test.skip(!ENABLED, 'needs TETIVA_REAL_BACKEND=1 plus bin/client-server on :8080 and cmd/authmock on :9877');
  test.setTimeout(120_000);

  test('the authorization code grant mints a token the send then uses', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Code');
    await createRequest(page, collection, 'CodeGrant');
    await fillCodeGrant(page);

    const authorizeUrl = await startFlow(page);
    expect(await lastExternalUrl(page)).toBe(authorizeUrl);
    await visitInBrowser(page, authorizeUrl, /You can close this tab/);

    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until', { timeout: 20_000 });
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);

    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "oauth2"', { timeout: 20_000 });
    await expect(responseBody(page)).toContainText('"grant": "authorization_code"');
  });

  test('a denied authorization reports the provider error', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Deny');
    await createRequest(page, collection, 'DeniedGrant');
    await fillCodeGrant(page);

    const authorizeUrl = await startFlow(page);
    // The URL already carries a query, so the flag is set on it rather than
    // appended.
    const denied = new URL(authorizeUrl);
    denied.searchParams.set('deny', '1');
    await visitInBrowser(page, denied.toString(), /You can close this tab/);

    await expect(page.getByTestId('oauth2-token-error')).toContainText('access_denied', { timeout: 20_000 });
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
    await expect(page.getByTestId('oauth2-get-token')).toBeEnabled();
  });

  test('Cancel frees the fixed loopback port for another request', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Port');
    await createRequest(page, collection, 'PortFirst');
    await fillCodeGrant(page);
    await startFlow(page);

    await page.getByTestId('oauth2-flow-cancel').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);

    await createRequest(page, collection, 'PortSecond');
    await fillCodeGrant(page);
    await startFlow(page);
    await expect(page.getByTestId('oauth2-token-error')).toHaveCount(0);

    await page.getByTestId('oauth2-flow-cancel').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
  });

  test('the device grant completes through the verification page', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Device');
    await createRequest(page, collection, 'DeviceGrant');

    await selectAuthType(page, 'OAuth 2.0');
    await selectGrant(page, 'Device Code');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/token"]').fill(FLOW_TOKEN_URL);
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill(DEVICE_URL);
    await page.locator('[aria-placeholder="Client ID"]').fill('test-client');
    await page.locator('[aria-placeholder="Client Secret"]').fill('test-secret');
    await save(page);

    await page.getByTestId('oauth2-get-token').click();
    const userCode = page.getByTestId('oauth2-user-code');
    await expect(userCode).toHaveText(/[A-Z0-9]{4}-[A-Z0-9]{4}/, { timeout: 20_000 });

    // The bare verification_uri would answer 404 and the poller would wait for
    // an approval that can never arrive, so the button must open the complete one.
    await page.getByTestId('oauth2-verification-open').click();
    const opened = await lastExternalUrl(page);
    expect(opened).toContain(encodeURIComponent((await userCode.textContent())!.trim()));
    await visitInBrowser(page, opened as string, /approved/);

    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until', { timeout: 40_000 });
    await send(page);
    await expect(responseBody(page)).toContainText('"scheme": "oauth2"', { timeout: 20_000 });
  });

  test('a reload re-attaches the pending flow and still finishes it', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Reload');
    await createRequest(page, collection, 'Reattached');
    await fillCodeGrant(page);
    const authorizeUrl = await startFlow(page);

    await page.reload();
    await reopenOnAuthTab(page, 'Flow Reload', 'Reattached');

    const panel = page.getByTestId('oauth2-flow-pending');
    await expect(panel).toBeVisible({ timeout: 20_000 });
    expect(await panel.getAttribute('data-authorize-url')).toBe(authorizeUrl);

    await visitInBrowser(page, authorizeUrl, /You can close this tab/);
    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until', { timeout: 20_000 });
  });

  test('a reload restores the panel of the running flow, not of the saved grant', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Grant');
    await createRequest(page, collection, 'GrantSwitch');
    await fillCodeGrant(page);

    // The device grant stays in the buffer, so the reload below restores the
    // saved authorization_code while the flow that is running is a device one.
    await selectGrant(page, 'Device Code');
    await page.locator('[aria-placeholder="https://idp.example.com/oauth2/device"]').fill(DEVICE_URL);
    await page.getByTestId('oauth2-get-token').click();
    const code = page.getByTestId('oauth2-user-code');
    await expect(code).toHaveText(/[A-Z0-9]{4}-[A-Z0-9]{4}/, { timeout: 20_000 });
    const userCode = (await code.textContent())!.trim();

    await page.reload();
    await reopenOnAuthTab(page, 'Flow Grant', 'GrantSwitch');

    await expect(page.getByRole('combobox', { name: 'Grant Type' })).toContainText('Authorization Code');
    await expect(page.getByTestId('oauth2-user-code')).toHaveText(userCode, { timeout: 20_000 });
    await expect(page.getByTestId('oauth2-verification-open')).toBeVisible();

    await page.getByTestId('oauth2-flow-cancel').click();
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);
  });

  test('a reload that lost the record still starts a flow on the same port', async ({ page }) => {
    await noSystemBrowser(page);
    await openApp(page);
    const collection = await createCollection(page, 'Flow Orphan');
    await createRequest(page, collection, 'Orphaned');
    await fillCodeGrant(page);
    await startFlow(page);

    await page.evaluate(() => sessionStorage.clear());
    await page.reload();
    await reopenOnAuthTab(page, 'Flow Orphan', 'Orphaned');
    await expect(page.getByTestId('oauth2-flow-pending')).toHaveCount(0);

    const restarted = await startFlow(page);
    await expect(page.getByTestId('oauth2-token-error')).toHaveCount(0);

    await visitInBrowser(page, restarted, /You can close this tab/);
    await expect(page.getByTestId('oauth2-token-status')).toContainText('Valid until', { timeout: 20_000 });
  });
});
