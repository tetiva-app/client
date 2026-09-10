import { test, expect, type Browser, type Locator, type Page } from '@playwright/test';

// Runs against the Go backend and cmd/signinmock, not the Vite mock build. Every
// process is started by hand (task common:build:server hangs — manual equivalent):
//   cd client && export PATH="$HOME/go/bin:$PATH"
//   wails3 generate bindings -f '-tags server' -clean=true -ts
//   (cd frontend && npm run build)
//   go build -tags server -o bin/client-server . && go build -o bin/signinmock ./cmd/signinmock
//   DATA=$(mktemp -d) && TETIVA_DATA_DIR=$DATA ./bin/client-server &
//   ./bin/signinmock &
//   cd frontend && TETIVA_REAL_BACKEND=1 npx playwright test \
//     --project=real-backend --workers=1 tests/signin-real-backend.spec.ts
const ENABLED = process.env.TETIVA_REAL_BACKEND === '1';

const MOCK_GRPC = '127.0.0.1:9878';
const MOCK_HTTP = 'http://127.0.0.1:9879';
const MOCK_EMAIL = 'browser@signinmock.local';

// One backend, one SQLite file and one sign-in manager for the whole file, so
// nothing here may run beside anything else.
test.describe.configure({ mode: 'serial' });

interface MockMode {
  noRedirect?: boolean;
  verifyGate?: boolean;
}

async function setMode(page: Page, mode: MockMode) {
  const res = await page.request.post(`${MOCK_HTTP}/__mode`, { data: mode });
  expect(res.status()).toBe(204);
}

// The server build injects no Wails runtime, so the query flag is what makes the
// frontend pick the real services; without it these scenarios run on the mocks.
async function openApp(page: Page) {
  const mockMarkers: string[] = [];
  page.on('console', (m) => {
    if (m.text().includes('Running in browser mode with mock services')) mockMarkers.push(m.text());
  });
  await page.goto('/?wails=1');
  await expect(page.getByTitle('New Collection')).toBeVisible();
  expect(mockMarkers).toEqual([]);
}

// Records the URL the app would open instead of launching the system browser:
// a real launch would consume the one-shot loopback outside the test's reach.
async function noSystemBrowser(page: Page) {
  await page.addInitScript(() => {
    (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
  });
}

function lastExternalUrl(page: Page): Promise<string | undefined> {
  return page.evaluate(() => (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl);
}

async function openSync(page: Page): Promise<Locator> {
  await page.getByRole('button', { name: 'Sync', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();

  return dialog;
}

function customToggle(dialog: Locator): Locator {
  return dialog.getByRole('button', { name: 'Use custom server' });
}

// The stored server address reopens the custom section by itself a tick after
// the dialog appears, so a single blind toggle can just as well close it again.
async function useMockServer(dialog: Locator) {
  const field = dialog.locator('#server-url');
  const toggle = customToggle(dialog);
  await expect(toggle).toBeVisible({ timeout: 20_000 });
  await expect(async () => {
    if (!(await field.isVisible())) await toggle.click();
    await expect(field).toBeVisible({ timeout: 1_000 });
  }).toPass({ timeout: 20_000 });

  await field.fill(MOCK_GRPC);
}

async function startSignIn(dialog: Locator, page: Page): Promise<string> {
  const button = dialog.getByTestId('signin-browser');
  await expect(button).toBeVisible({ timeout: 20_000 });
  await button.click();
  await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible({ timeout: 20_000 });
  await expect.poll(() => lastExternalUrl(page), { timeout: 20_000 }).toBeTruthy();

  const url = await lastExternalUrl(page) as string;
  expect(url).toContain('#claim=');

  return url;
}

// The cabinet gets a context of its own, so the app's own window cannot consume
// the one-shot loopback.
async function openCabinet(browser: Browser, url: string): Promise<Page> {
  const tab = await (await browser.newContext()).newPage();
  await tab.goto(url);

  return tab;
}

async function consent(tab: Page, action: 'approve' | 'deny') {
  await expect(tab.locator(`#${action}`)).toBeEnabled({ timeout: 15_000 });
  await tab.locator(`#${action}`).click();
}

async function expectConnected(dialog: Locator) {
  await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible({ timeout: 40_000 });
  await expect(dialog.getByText(MOCK_EMAIL)).toBeVisible();
}

test.describe('Browser sign-in against the real backend', () => {
  test.skip(!ENABLED, 'needs TETIVA_REAL_BACKEND=1 plus bin/client-server on :8080 and cmd/signinmock on :9878/:9879');
  test.setTimeout(120_000);

  test.beforeEach(async ({ page }) => {
    test.skip(!ENABLED, 'needs the real backend');

    await noSystemBrowser(page);
    await openApp(page);

    // The status arrives after the dialog opens, so settle which view won before
    // reading: a connected modal has no sign-in button to press.
    const dialog = await openSync(page);
    const disconnect = dialog.getByRole('button', { name: 'Disconnect' });
    await expect(disconnect.or(customToggle(dialog)).first()).toBeVisible({ timeout: 20_000 });
    await page.waitForTimeout(500);
    if (await disconnect.isVisible()) {
      await disconnect.click();
      await expect(customToggle(dialog)).toBeVisible({ timeout: 20_000 });
    }
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    expect((await page.request.post(`${MOCK_HTTP}/__reset`, { data: {} })).status()).toBe(204);
    await setMode(page, {});
  });

  test('an approval in the browser redirects to the loopback and connects the app', async ({ page, browser }) => {
    const dialog = await openSync(page);
    await useMockServer(dialog);
    const url = await startSignIn(dialog, page);

    const tab = await openCabinet(browser, url);
    await consent(tab, 'approve');
    await expect(tab.locator('body')).toContainText('You can close this tab', { timeout: 15_000 });
    await tab.context().close();

    await expectConnected(dialog);
  });

  test('the poll alone finishes the sign-in when no redirect comes back', async ({ page, browser }) => {
    await setMode(page, { noRedirect: true });

    const dialog = await openSync(page);
    await useMockServer(dialog);
    const url = await startSignIn(dialog, page);

    const tab = await openCabinet(browser, url);
    await consent(tab, 'approve');
    await expect(tab.locator('#status')).toHaveText('Done. Return to Tetiva.', { timeout: 15_000 });
    await tab.context().close();

    await expectConnected(dialog);
  });

  test('a refusal in the browser leaves an error and Try again', async ({ page, browser }) => {
    const dialog = await openSync(page);
    await useMockServer(dialog);
    const url = await startSignIn(dialog, page);

    const tab = await openCabinet(browser, url);
    await consent(tab, 'deny');
    await expect(tab.locator('#status')).toHaveText('Denied. Return to Tetiva.', { timeout: 15_000 });
    await tab.context().close();

    // The exact §2.5 copy, so the mock suite and the real one cannot drift apart.
    await expect(dialog.getByTestId('signin-browser-error'))
      .toHaveText('Sign-in was denied in the browser', { timeout: 40_000 });
    await expect(dialog.getByTestId('signin-browser-retry')).toBeVisible();
  });

  test('Cancel drops the request the browser would have approved', async ({ page, browser }) => {
    const dialog = await openSync(page);
    await useMockServer(dialog);
    const url = await startSignIn(dialog, page);

    await dialog.getByTestId('signin-browser-cancel').click();
    await expect(dialog.getByText('Sign-in cancelled.')).toBeVisible();
    await expect(dialog.getByTestId('signin-browser-retry')).toBeVisible();

    const tab = await openCabinet(browser, url);
    await expect(tab.locator('#error')).toContainText('Cannot continue', { timeout: 15_000 });
    await expect(tab.locator('#approve')).toBeDisabled();
    await tab.context().close();
  });

  test('the email gate holds the panel, moves the deadline out and then connects', async ({ page, browser }) => {
    await setMode(page, { verifyGate: true });

    const dialog = await openSync(page);
    await useMockServer(dialog);
    const url = await startSignIn(dialog, page);

    const tab = await openCabinet(browser, url);
    await consent(tab, 'approve');
    await expect(tab.locator('body')).toContainText('You can close this tab', { timeout: 15_000 });
    await tab.context().close();

    // The gate lasts two polls, so the hint and the extended deadline are read
    // from one snapshot of the panel rather than asserted one after the other.
    const panel = dialog.getByTestId('signin-browser-pending');
    await expect.poll(async () => {
      const text = await panel.textContent().catch(() => '') ?? '';

      return /Confirm your email/.test(text) && /Expires in (2\d|30):\d\d/.test(text);
    }, { timeout: 40_000, intervals: [100] }).toBe(true);

    await expectConnected(dialog);
  });
});
