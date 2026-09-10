import { test, expect, type Locator, type Page } from '@playwright/test';

const CLOUD_HOST = 'app.tetiva.app';
const CUSTOM_SERVER = 'sync.corp.local';

// Every sign-in URL goes through openExternal; this records it instead of
// launching the system browser, and collapses the scripted mock delays.
async function stubExternalOpener(page: Page, signInMs?: number) {
  await page.addInitScript((ms) => {
    (window as unknown as { __TETIVA_NO_EXTERNAL_OPEN__?: boolean }).__TETIVA_NO_EXTERNAL_OPEN__ = true;
    if (ms !== undefined) localStorage.setItem('tetiva.mockSignInMs', String(ms));
  }, signInMs);
}

function lastExternalUrl(page: Page): Promise<string | undefined> {
  return page.evaluate(() => (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl);
}

function forgetExternalUrl(page: Page): Promise<void> {
  return page.evaluate(() => { delete (window as unknown as { __lastExternalUrl?: string }).__lastExternalUrl; });
}

async function openSync(page: Page): Promise<Locator> {
  await page.getByRole('button', { name: 'Sync', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  return dialog;
}

async function openScenario(page: Page, scenario = '', signInMs?: number): Promise<Locator> {
  await stubExternalOpener(page, signInMs);
  await page.goto(scenario ? `/?mock=${scenario}` : '/');
  return openSync(page);
}

async function useCustomServer(dialog: Locator, address: string) {
  await dialog.getByRole('button', { name: 'Use custom server' }).click();
  await dialog.locator('#server-url').fill(address);
}

async function countdownSeconds(panel: Locator): Promise<number> {
  const text = await panel.textContent() ?? '';
  const found = /Expires in (\d\d):(\d\d)/.exec(text);
  if (!found) throw new Error(`no countdown in panel: ${text}`);
  return Number(found[1]) * 60 + Number(found[2]);
}

// Reads the countdown as soon as it exists: the tick corrects a stale clock
// after a second, so a plain expect() poll would never see the first frame.
async function firstCountdownSeconds(panel: Locator): Promise<number> {
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    const text = await panel.textContent() ?? '';
    const found = /Expires in (\d\d):(\d\d)/.exec(text);
    if (found) return Number(found[1]) * 60 + Number(found[2]);
  }
  throw new Error('the countdown never rendered');
}

test.describe('Browser sign-in', () => {
  test('offers the browser buttons and names the host', async ({ page }) => {
    const dialog = await openScenario(page);

    await expect(dialog.getByTestId('signin-browser')).toBeVisible();
    await expect(dialog.getByTestId('signin-browser-register')).toBeVisible();
    await expect(dialog.getByTestId('signin-browser-host'))
      .toHaveText(`Opens ${CLOUD_HOST} in your browser`);
  });

  test('opens the login URL with the claim secret and waits', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();

    const panel = dialog.getByTestId('signin-browser-pending');
    await expect(panel).toContainText('Waiting for you to finish in the browser');
    await expect(panel).toContainText(/Expires in \d\d:\d\d/);
    // The countdown ticks every second inside a live region; only the state
    // sentences may be announced.
    await expect(panel.getByText(/Expires in/)).toHaveAttribute('aria-live', 'off');
    await expect(panel).toContainText('Anyone with this link can approve the sign-in.');
    // The tabs give way to the panel: one sign-in at a time.
    await expect(dialog.getByRole('tab', { name: 'Login' })).toHaveCount(0);

    expect(await lastExternalUrl(page)).toContain('#claim=');
  });

  test('an approved sign-in lands on the connected view', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-approve', 200);
    await dialog.getByTestId('signin-browser').click();

    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(dialog.getByText('browser@example.com')).toBeVisible();
    await expect(dialog.getByText('Tetiva Cloud')).toBeVisible();
  });

  test('a workspace created after signing in can be synced', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-approve', 200);
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    await page.getByRole('button', { name: /Default Workspace/ }).first().click();
    await page.getByRole('button', { name: 'Create workspace' }).click();

    await expect(page.getByText('Sync to server')).toBeVisible();
  });

  test('done without an account falls back to the stored config', async ({ page }) => {
    const failures: string[] = [];
    page.on('pageerror', (e) => failures.push(e.message));

    const dialog = await openScenario(page, 'signin-approve-no-auth', 200);
    await dialog.getByTestId('signin-browser').click();

    await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(dialog.getByText('browser@example.com')).toBeVisible();
    expect(failures).toEqual([]);
  });

  test('an unconfirmed address lands on the waiting screen', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-verify', 200);
    await dialog.getByTestId('signin-browser-register').click();

    const waiting = dialog.getByTestId('sync-verify-waiting');
    await expect(waiting).toBeVisible();
    await expect(waiting).toContainText('browser@example.com');
  });

  test('a pending confirmation keeps the panel and moves the deadline out', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-verify-pending');
    await dialog.getByTestId('signin-browser').click();

    const panel = dialog.getByTestId('signin-browser-pending');
    await expect(panel.getByTestId('signin-browser-verify-hint')).toBeVisible();
    // The request starts with ten minutes on it; anything past that is the
    // extension the server granted while the address is confirmed.
    await expect(panel).toContainText(/Expires in 29:\d\d/);
  });

  test('the countdown ticks down on its own', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();

    const panel = dialog.getByTestId('signin-browser-pending');
    await expect(panel).toContainText(/Expires in \d\d:\d\d/);
    const first = await countdownSeconds(panel);

    await page.waitForTimeout(2100);
    expect(await countdownSeconds(panel)).toBeLessThan(first);
  });

  test('the first countdown frame is the real deadline, not the modal\'s age', async ({ page }) => {
    const dialog = await openScenario(page);
    // The stale clock is the one read when the modal was built, so it has to sit
    // open a while before the flow starts.
    await page.waitForTimeout(3000);

    const started = Date.now();
    await dialog.getByTestId('signin-browser').click();
    const shown = await firstCountdownSeconds(dialog.getByTestId('signin-browser-pending'));
    const elapsed = (Date.now() - started) / 1000;

    // The mock hands out ten minutes from the moment the request is filed.
    expect(shown).toBeLessThanOrEqual(600);
    expect(shown).toBeGreaterThanOrEqual(600 - elapsed - 2);
  });

  test('a refusal offers Try again, which creates a new request', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-denied');
    await dialog.getByTestId('signin-browser').click();

    await expect(dialog.getByTestId('signin-browser-error'))
      .toHaveText('Sign-in was denied in the browser');
    const denied = await lastExternalUrl(page);

    await dialog.getByTestId('signin-browser-retry').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();
    await expect.poll(() => lastExternalUrl(page)).not.toBe(denied);
  });

  test('a refusal leaves the sync indicator unalarmed', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-denied');
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('signin-browser-error')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const indicator = page.getByRole('button', { name: 'Sync', exact: true });
    await expect(indicator.getByTestId('sync-parked-alert')).toHaveCount(0);
    await indicator.hover();
    await expect(page.getByText('Not connected')).toBeVisible();
  });

  test('an expired request says so', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-expired');
    await dialog.getByTestId('signin-browser').click();

    await expect(dialog.getByTestId('signin-browser-error'))
      .toHaveText('Sign-in link expired, try again');
  });

  test('Cancel drops the request and Try again starts a new one', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();

    await dialog.getByTestId('signin-browser-cancel').click();
    await expect(dialog.getByText('Sign-in cancelled.')).toBeVisible();
    await expect(dialog.getByTestId('signin-browser-pending')).toHaveCount(0);

    await dialog.getByTestId('signin-browser-retry').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();
  });

  test('a cancelled attempt does not outlive the dialog', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();
    await dialog.getByTestId('signin-browser-cancel').click();
    await expect(dialog.getByText('Sign-in cancelled.')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const reopened = await openSync(page);
    await expect(reopened.getByTestId('signin-browser')).toBeVisible();
    await expect(reopened.getByTestId('signin-browser-register')).toBeVisible();
    await expect(reopened.getByText('Sign-in cancelled.')).toHaveCount(0);
  });

  test('a cancelled attempt does not hide the form of a self-hosted server', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unreachable-custom');
    await dialog.getByTestId('signin-browser').click();
    await dialog.getByTestId('signin-browser-cancel').click();
    await expect(dialog.getByText('Sign-in cancelled.')).toBeVisible();

    await useCustomServer(dialog, CUSTOM_SERVER);

    await expect(dialog.getByRole('tab', { name: 'Login' })).toBeVisible();
    await expect(dialog.locator('#login-email')).toBeVisible();
  });

  test('a server without the capability keeps the email form', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unsupported');
    await useCustomServer(dialog, CUSTOM_SERVER);

    await expect(dialog.getByRole('tab', { name: 'Login' })).toBeVisible();
    await expect(dialog.locator('#login-email')).toBeVisible();
    await expect(dialog.getByTestId('signin-browser')).toHaveCount(0);
  });

  test('an unreachable cloud is an error with Retry, not a form', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unreachable');

    await expect(dialog.getByTestId('signin-caps-unreachable'))
      .toContainText('Cannot reach Tetiva Cloud.');
    await expect(dialog.getByRole('tab', { name: 'Login' })).toHaveCount(0);

    await dialog.getByTestId('signin-caps-retry').click();
    await expect(dialog.getByTestId('signin-caps-unreachable')).toBeVisible();
  });

  test('the cloud never offers an email form, whatever discovery answers', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-caps-slow');

    const noCredentials = async () => {
      await expect(dialog.getByRole('tab', { name: 'Login' })).toHaveCount(0);
      await expect(dialog.getByRole('tab', { name: 'Register' })).toHaveCount(0);
      await expect(dialog.locator('#login-email')).toHaveCount(0);
      await expect(dialog.locator('#reg-email')).toHaveCount(0);
    };

    await noCredentials();
    await expect(dialog.getByTestId('signin-caps-checking')).toBeVisible();
    await noCredentials();
    await expect(dialog.getByTestId('signin-browser')).toBeVisible();
    await noCredentials();
  });

  test('a cloud without the capability asks for an update, not for credentials', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unsupported');

    await expect(dialog.getByTestId('signin-caps-outdated'))
      .toContainText('needs a newer Tetiva Cloud');
    await expect(dialog.getByTestId('signin-caps-retry')).toBeVisible();
    await expect(dialog.getByRole('tab', { name: 'Login' })).toHaveCount(0);
    await expect(dialog.locator('#login-email')).toHaveCount(0);
  });

  test('an unreachable custom server warns and still offers the form', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unreachable-custom');
    await expect(dialog.getByTestId('signin-browser')).toBeVisible();

    await useCustomServer(dialog, CUSTOM_SERVER);

    await expect(dialog.getByTestId('signin-caps-unreachable'))
      .toContainText('Cannot reach this server.');
    await expect(dialog.getByTestId('signin-caps-retry')).toBeVisible();
    await expect(dialog.locator('#login-email')).toBeVisible();
  });

  test('a half-typed address does not reach the network', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-unreachable-custom');
    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await dialog.locator('#server-url').fill('sync');

    await expect(dialog.getByTestId('signin-caps-incomplete')).toBeVisible();
    // Discovery answers "unreachable" for every custom address in this scenario,
    // so its absence past the debounce is the proof nothing was dialled.
    await page.waitForTimeout(800);
    await expect(dialog.getByTestId('signin-caps-unreachable')).toHaveCount(0);
    await expect(dialog.getByTestId('signin-caps-checking')).toHaveCount(0);

    await dialog.locator('#server-url').fill(CUSTOM_SERVER);
    await expect(dialog.getByTestId('signin-caps-unreachable')).toBeVisible();
  });

  test('a slow answer for the previous address is dropped', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-caps-slow');
    await expect(dialog.getByTestId('signin-caps-checking')).toBeVisible();

    await useCustomServer(dialog, `${CUSTOM_SERVER}:443`);
    const host = dialog.getByTestId('signin-browser-host');
    await expect(host).toHaveText(`Opens ${CUSTOM_SERVER} in your browser`);

    // The cloud answer lands a second later and must not claim the field.
    await page.waitForTimeout(1800);
    await expect(host).toHaveText(`Opens ${CUSTOM_SERVER} in your browser`);
  });

  test('a slow start shows Contacting without the link buttons', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-slow-start');
    await dialog.getByTestId('signin-browser').click();

    const panel = dialog.getByTestId('signin-browser-pending');
    await expect(panel).toContainText('Contacting');
    await expect(panel.getByTestId('signin-browser-open-again')).toHaveCount(0);
    await expect(panel.getByTestId('signin-browser-copy')).toHaveCount(0);

    await expect(panel.getByTestId('signin-browser-open-again')).toBeVisible({ timeout: 5000 });
  });

  test('closing and reopening the modal keeps the panel', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const reopened = await openSync(page);
    await expect(reopened.getByTestId('signin-browser-pending')).toBeVisible();
  });

  test('a reload re-attaches to the running flow and Open again still works', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();
    const opened = await lastExternalUrl(page);

    await page.reload();
    const reopened = await openSync(page);
    const panel = reopened.getByTestId('signin-browser-pending');
    await expect(panel).toBeVisible();

    await forgetExternalUrl(page);
    await panel.getByTestId('signin-browser-open-again').click();
    await expect.poll(() => lastExternalUrl(page)).toBe(opened);
  });

  test('opening the modal before the page settles still adopts the flow', async ({ page }) => {
    const dialog = await openScenario(page);
    await dialog.getByTestId('signin-browser').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();

    await page.reload({ waitUntil: 'commit' });
    const reopened = await openSync(page);
    await expect(reopened.getByTestId('signin-browser-pending')).toBeVisible();
  });
});

test.describe('Register intent from the welcome screen', () => {
  // The welcome only shows up on a profile that has never made the choice.
  test.use({ storageState: { cookies: [], origins: [] } });

  test('highlights Create account', async ({ page }) => {
    await stubExternalOpener(page);
    await page.goto('/');
    await page.getByTestId('onboarding-choice-account').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog.getByTestId('signin-browser-register')).toHaveClass(/bg-primary/);
    await expect(dialog.getByTestId('signin-browser')).not.toHaveClass(/bg-primary/);
  });

  test('starts the flow with the register intent', async ({ page }) => {
    await stubExternalOpener(page);
    await page.goto('/');
    await page.getByTestId('onboarding-choice-account').click();

    const dialog = page.getByRole('dialog');
    await dialog.getByTestId('signin-browser-register').click();
    await expect(dialog.getByTestId('signin-browser-pending')).toBeVisible();

    expect(await lastExternalUrl(page)).toContain('intent=register');
  });
});

test.describe('Sync modal on a localized machine', () => {
  test.use({ locale: 'ru-RU' });

  test('the verification view keeps the modal\'s English', async ({ page }) => {
    const dialog = await openScenario(page, 'signin-verify', 200);
    await dialog.getByTestId('signin-browser-register').click();

    const waiting = dialog.getByTestId('sync-verify-waiting');
    await expect(waiting).toBeVisible();
    await expect(waiting).toContainText('Check your email');
    await expect(waiting).toContainText('We sent a message to browser@example.com');
    await expect(waiting.getByTestId('sync-verify-confirmed')).toHaveText('I confirmed my email');
    await expect(waiting.getByTestId('sync-verify-resend')).toHaveText('Send again');
    await expect(waiting.getByTestId('sync-verify-logout')).toHaveText('Sign out');
    await expect(waiting.getByRole('button', { name: 'Close' })).toBeVisible();
  });
});
