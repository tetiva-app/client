import { test, expect } from '@playwright/test';

// The Vite mock build (MockWebSocketService) echoes each sent frame back as an
// inbound message, so the connect → send → echo → disconnect flow runs without a backend.
test.describe('WebSocket', () => {
  async function createWsRequest(page, collName: string, reqName: string) {
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
    await expect(page.getByText('No messages yet')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Send' })).toBeDisabled();
    // Environment selector is present so {{vars}} in the URL can resolve
    // (mock has an active "Default" environment).
    await expect(page.getByRole('button', { name: 'Default', exact: true })).toBeVisible();
  });

  test('connect → send → echo → disconnect round-trip', async ({ page }) => {
    await createWsRequest(page, 'WS Echo Coll', 'Echo');

    const url = page.getByPlaceholder('ws://… or wss://…');
    await url.fill('ws://127.0.0.1:9876');

    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(url).toBeDisabled();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    const sendBtn = page.getByRole('button', { name: 'Send' });
    await expect(sendBtn).toBeEnabled();

    const compose = page.locator('.cm-content').last();
    await compose.click();
    await compose.fill('ping-123');
    await sendBtn.click();

    // The log shows the optimistic outgoing row (↑) and the echoed inbound (↓),
    // both carrying the payload → exactly two rows with 'ping-123'.
    const log = page.getByRole('list');
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

    const compose = page.locator('.cm-content').last();
    await compose.click();
    await compose.fill('via-hotkey');
    await page.keyboard.press('Meta+Enter');

    // Outgoing (↑) + echoed inbound (↓) → two rows with the payload.
    const log = page.getByRole('list');
    await expect(log.getByText('via-hotkey')).toHaveCount(2);
  });
});
