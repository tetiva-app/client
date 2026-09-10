import { test, expect, type Page } from '@playwright/test';

// The Vite mock build (MockWebSocketService) echoes each sent frame back as an
// inbound message, so the connect → send → echo → disconnect flow runs without a backend.
test.describe('WebSocket', () => {
  async function createWsRequest(page: Page, collName: string, reqName: string) {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const cDialog = page.getByRole('dialog');
    const cInput = cDialog.getByPlaceholder('Collection name');
    await cInput.fill(collName);
    await cInput.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText(collName);
    await expect(collectionItem).toBeVisible();
    await collectionItem.click();

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();

    const reqDialog = page.getByRole('dialog');
    await expect(reqDialog).toContainText('New Request');
    await reqDialog.getByRole('button', { name: 'WebSocket' }).click();
    const rInput = reqDialog.getByPlaceholder('Request name');
    await rInput.fill(reqName);
    await rInput.press('Enter');
  }

  test('creates a websocket request and renders the WS editor', async ({ page }) => {
    await createWsRequest(page, 'WS Render Coll', 'Echo');

    await expect(page.getByPlaceholder('ws://… or wss://…')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Connect' })).toBeVisible();
    // The sidebar row wears the protocol badge, never an HTTP method.
    const sidebarRow = page.locator('aside [data-tree-item-id]').filter({ hasText: 'Echo' });
    await expect(sidebarRow).toContainText('WS');
    await expect(sidebarRow).not.toContainText('GET');
    await expect(page.getByText('No messages yet')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Send' })).toBeDisabled();
    // Environment selector is present so {{vars}} in the URL can resolve
    // (mock has an active "Default" environment).
    await expect(page.getByRole('button', { name: 'Default', exact: true })).toBeVisible();
  });

  test('renders the six request tabs', async ({ page }) => {
    await createWsRequest(page, 'WS Tabs Coll', 'Echo');

    for (const label of ['Messages', 'Params', 'Auth', 'Headers', 'Scripts', 'Docs']) {
      await expect(page.getByRole('button', { name: label, exact: true })).toBeVisible();
    }

    await page.getByRole('button', { name: 'Params', exact: true }).click();
    await expect(page.getByPlaceholder('Param name')).toBeVisible();

    await page.getByRole('button', { name: 'Auth', exact: true }).click();
    await expect(page.getByText('Authorization Type')).toBeVisible();

    // WS only runs a pre-connect script, so the post-response phase is absent.
    await page.getByRole('button', { name: 'Scripts', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Pre-connect' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Post-response' })).toHaveCount(0);
  });

  test('connect → send → echo → disconnect round-trip', async ({ page }) => {
    await createWsRequest(page, 'WS Echo Coll', 'Echo');

    const url = page.getByPlaceholder('ws://… or wss://…');
    await url.fill('ws://127.0.0.1:9876');

    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(url).toBeDisabled();
    const sendBtn = page.getByRole('button', { name: 'Send' });
    await expect(sendBtn).toBeEnabled();

    const compose = page.locator('[aria-placeholder="Message"]');
    await compose.click();
    await compose.fill('ping-123');
    await sendBtn.click();

    // The log shows the optimistic outgoing row (↑) and the echoed inbound (↓),
    // both carrying the payload → exactly two rows with 'ping-123'.
    const log = page.getByTestId('ws-log');
    await expect(log.getByText('ping-123')).toHaveCount(2);
    await expect(log.getByText('↑')).toBeVisible();
    await expect(log.getByText('↓')).toBeVisible();

    await page.getByRole('button', { name: 'Disconnect' }).click();
    await expect(url).toBeEnabled();
    await expect(page.getByRole('button', { name: 'Connect' })).toBeVisible();
    await expect(sendBtn).toBeDisabled();
  });

  test('sends the composed message on Cmd+Enter', async ({ page }) => {
    await createWsRequest(page, 'WS Hotkey Coll', 'Echo');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876');
    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();

    const compose = page.locator('[aria-placeholder="Message"]');
    await compose.click();
    await compose.fill('via-hotkey');
    await page.keyboard.press('Meta+Enter');

    // Outgoing (↑) + echoed inbound (↓) → two rows with the payload.
    await expect(page.getByTestId('ws-log').getByText('via-hotkey')).toHaveCount(2);
  });

  // Cmd+S reaches the WS tab through RequestEditor's window handler, which skips
  // only gRPC and GraphQL; a websocket early-return there would silently break it.
  test('saves the Docs description on Cmd+S', async ({ page }) => {
    await createWsRequest(page, 'WS Notes Coll', 'Echo');

    await page.getByRole('button', { name: 'Docs', exact: true }).click();
    await page.getByRole('button', { name: 'Add description' }).click();
    const docs = page.locator('[aria-placeholder^="Document this request"]');
    await docs.fill('Echo endpoint, no auth.');

    // Two dots: the open tab in the tab bar and the WS editor's own tab strip.
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(2);
    await page.keyboard.press('Meta+s');
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);
  });

  test('saves a header on Cmd+S', async ({ page }) => {
    await createWsRequest(page, 'WS Headers Coll', 'Echo');

    await page.getByRole('button', { name: 'Headers', exact: true }).click();
    await page.getByPlaceholder('Header name').fill('X-Test');
    await page.getByPlaceholder('Value').fill('42');
    await page.getByPlaceholder('Value').press('Enter');

    await expect(page.getByTitle('Unsaved changes')).toHaveCount(2);
    await page.keyboard.press('Meta+s');
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);
    // The badges read like the HTTP ones: a header count, not a dot.
    await expect(page.getByRole('button', { name: /^Headers/ })).toContainText('(1)');
  });

  test('badges count headers and name the auth type, as HTTP does', async ({ page }) => {
    await createWsRequest(page, 'WS Badges Coll', 'Echo');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876/?a=1&b=2');
    await expect(page.getByRole('button', { name: /^Params/ })).toContainText('(2)');

    await page.getByRole('button', { name: 'Auth', exact: true }).click();
    await page.locator('button[role="combobox"]').last().click();
    await page.locator('[role="listbox"] [role="option"]', { hasText: 'Bearer Token' }).click();
    await expect(page.getByRole('button', { name: /^Auth/ })).toContainText('(Bearer)');

    await page.getByRole('button', { name: /^Headers/ }).click();
    await page.getByPlaceholder('Header name').fill('X-One');
    await page.getByPlaceholder('Value').fill('1');
    await page.getByPlaceholder('Value').press('Enter');
    await expect(page.getByRole('button', { name: /^Headers/ })).toContainText('(1)');
  });

  test('saves a message and loads it back from a chip', async ({ page }) => {
    await createWsRequest(page, 'WS Saved Coll', 'Echo');

    await page.getByRole('button', { name: 'Text', exact: true }).click();
    const compose = page.locator('[aria-placeholder="Message"]');
    await compose.click();
    await compose.fill('hello saved');

    await page.getByRole('button', { name: 'Save', exact: true }).click();
    const nameInput = page.getByLabel('Message name');
    await nameInput.fill('Greeting');
    await nameInput.press('Enter');

    const chip = page.getByTestId('ws-saved-messages').getByRole('button', { name: 'Greeting', exact: true });
    await expect(chip).toBeVisible();
    // commitWsSettings persists the document, so nothing stays dirty.
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);

    await compose.fill('');
    await page.getByRole('button', { name: 'JSON', exact: true }).click();
    await chip.click();
    await expect(compose).toHaveText('hello saved');
    await expect(page.getByRole('button', { name: 'Text', exact: true })).toHaveAttribute('aria-pressed', 'true');
  });

  test('renames a saved message in its own place in the row', async ({ page }) => {
    await createWsRequest(page, 'WS Rename Coll', 'Echo');

    await page.getByRole('button', { name: 'Text', exact: true }).click();
    const compose = page.locator('[aria-placeholder="Message"]');
    for (const name of ['Alpha', 'Beta']) {
      await compose.click();
      await compose.fill(name.toLowerCase());
      await page.getByRole('button', { name: 'Save', exact: true }).click();
      const nameInput = page.getByLabel('Message name');
      await nameInput.fill(name);
      await nameInput.press('Enter');
      await expect(page.getByLabel('Message name')).toHaveCount(0);
    }

    const chips = page.getByTestId('ws-saved-messages');
    const beta = chips.getByRole('button', { name: 'Beta', exact: true });
    const betaX = (await beta.boundingBox())!.x;

    await chips.getByRole('button', { name: 'Alpha', exact: true }).dblclick();
    const rename = page.getByLabel('Message name');
    await expect(rename).toHaveValue('Alpha');
    // The input takes the renamed chip's slot, so it stays left of the next chip
    // and the old name is no longer shown twice.
    expect((await rename.boundingBox())!.x).toBeLessThan(betaX);
    await expect(chips.getByRole('button', { name: 'Alpha', exact: true })).toHaveCount(0);

    await rename.fill('Alpha 2');
    await rename.press('Enter');
    await expect(chips.getByRole('button', { name: 'Alpha 2', exact: true })).toBeVisible();
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);
  });

  test('shows a focus ring on the chip delete button', async ({ page }) => {
    await createWsRequest(page, 'WS Focus Coll', 'Echo');

    await page.getByRole('button', { name: 'Text', exact: true }).click();
    const compose = page.locator('[aria-placeholder="Message"]');
    await compose.click();
    await compose.fill('hello');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    const nameInput = page.getByLabel('Message name');
    await nameInput.fill('Greeting');
    await nameInput.press('Enter');

    // Tab from the chip label: keyboard focus is what :focus-visible reacts to.
    await page.getByTestId('ws-saved-messages').getByRole('button', { name: 'Greeting', exact: true }).focus();
    await page.keyboard.press('Tab');
    const del = page.getByRole('button', { name: 'Delete Greeting' });
    await expect(del).toBeFocused();
    await expect
      .poll(() =>
        del.evaluate((el) => {
          const s = getComputedStyle(el);
          const ring = s.boxShadow !== 'none' || s.outlineStyle !== 'none';
          return `${s.opacity}|${ring ? 'ring' : 'flat'}`;
        }),
      )
      .toBe('1|ring');
  });

  test('rejects a binary message that is not base64', async ({ page }) => {
    await createWsRequest(page, 'WS Binary Coll', 'Echo');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876');
    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();

    await page.getByRole('button', { name: 'Binary', exact: true }).click();
    const binary = page.getByLabel('Binary message (base64)');
    await binary.fill('not base64');
    await page.getByRole('button', { name: 'Send' }).click();

    await expect(page.getByText('Binary messages must be base64')).toBeVisible();
    const log = page.getByTestId('ws-log');
    await expect(log.getByText('not base64')).toHaveCount(0);

    // A valid payload goes through and the echo comes back tagged as binary.
    await binary.fill('aGVsbG8=');
    await page.getByRole('button', { name: 'Send' }).click();
    await expect(log.getByText('aGVsbG8=')).toHaveCount(2);
    await expect(log.getByText('bin')).toHaveCount(2);
  });

  test('stores ping and subprotocols from the settings popover', async ({ page }) => {
    await createWsRequest(page, 'WS Settings Coll', 'Echo');

    await page.getByRole('button', { name: 'Socket settings' }).click();
    await page.getByRole('spinbutton').fill('5');
    await page.getByPlaceholder('graphql-ws, json').fill('graphql-ws, json');
    await page.getByRole('button', { name: 'Apply' }).click();

    await expect(page.getByRole('button', { name: 'Apply' })).toHaveCount(0);
    await expect(page.getByTitle('Unsaved changes')).toHaveCount(0);

    await page.getByRole('button', { name: 'Socket settings' }).click();
    await expect(page.getByRole('spinbutton')).toHaveValue('5');
    await expect(page.getByPlaceholder('graphql-ws, json')).toHaveValue('graphql-ws, json');
    // Nothing is live yet, so no reconnect hint.
    await expect(page.getByText('Applies on the next connection.')).toHaveCount(0);
  });

  test('warns that connection settings apply on the next connection', async ({ page }) => {
    await createWsRequest(page, 'WS Settings Live Coll', 'Echo');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876');
    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();

    await page.getByRole('button', { name: 'Socket settings' }).click();
    await expect(page.getByText('Applies on the next connection.')).toBeVisible();
  });

  test('shows the pre-connect script console in the log', async ({ page }) => {
    await createWsRequest(page, 'WS Script Coll', 'Echo');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876');

    await page.getByRole('button', { name: 'Scripts', exact: true }).click();
    const script = page.locator('.cm-content').first();
    await script.click();
    await page.keyboard.type("console.log('hi')");

    await page.getByRole('button', { name: 'Messages', exact: true }).click();
    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();

    // Mock service answers with a fixed script outcome when a pre-script exists.
    const log = page.getByTestId('ws-log');
    await expect(log.getByText('mock pre-connect')).toBeVisible();
    await expect(log.locator('li[data-dir="system"]')).toHaveCount(1);
  });

  test('offers a WS chip in the history filter', async ({ page }) => {
    await page.goto('/');

    await page.getByRole('button', { name: 'History' }).click();
    await page.getByTitle('Filters').click();

    await expect(page.getByRole('button', { name: 'WS', exact: true })).toBeVisible();
  });
});
