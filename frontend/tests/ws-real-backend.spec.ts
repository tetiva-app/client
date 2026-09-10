import { test, expect } from '@playwright/test';

// Runs against the Go backend, not the Vite mock build. Both processes are started
// by hand (task common:build:server hangs — this is its manual equivalent):
//   cd client && export PATH="$HOME/go/bin:$PATH"
//   wails3 generate bindings -f '-tags server' -clean=true -ts
//   (cd frontend && npm run build)
//   go build -tags server -o bin/client-server .
//   DATA=$(mktemp -d) && TETIVA_DATA_DIR=$DATA ./bin/client-server &
//   go run ./cmd/wsecho &
//   cd frontend && TETIVA_REAL_BACKEND=1 npx playwright test --project=real-backend
const ENABLED = process.env.TETIVA_REAL_BACKEND === '1';

test.describe('WebSocket against the real backend', () => {
  test.skip(!ENABLED, 'needs TETIVA_REAL_BACKEND=1 plus bin/client-server on :8080 and cmd/wsecho on :9876');

  test('handshake headers, variables, binary frames, keepalive and history', async ({ page }) => {
    test.setTimeout(90_000);

    const mockMarkers: string[] = [];
    page.on('console', (m) => {
      if (m.text().includes('Running in browser mode with mock services')) mockMarkers.push(m.text());
    });

    // The server build injects no Wails runtime into the HTML, so the query flag
    // is what makes the frontend pick the real services over the mocks.
    await page.goto('/?wails=1');
    await expect(page.getByTitle('New Collection')).toBeVisible();
    expect(mockMarkers).toEqual([]);

    const activityBar = page.locator('.w-12.shrink-0');
    await activityBar.locator('button').nth(1).click();
    const envDialog = page.getByRole('dialog').filter({ hasText: 'Manage Environments' });
    const envInput = envDialog.getByPlaceholder('New environment');
    await envInput.fill('E2E');
    await envInput.press('Enter');

    const envList = envDialog.locator('.border-r.border-border');
    await envList.locator('button', { hasText: 'E2E' }).locator('button[title="Set as active"]').click();

    const varsPanel = envDialog.locator('.flex-1.flex.flex-col').last();
    await varsPanel.getByPlaceholder('Variable name').fill('v');
    await varsPanel.getByPlaceholder('Value').fill('from-env');
    await varsPanel.getByPlaceholder('Value').press('Enter');
    await expect(varsPanel.locator('input[value="v"]')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(envDialog).toHaveCount(0);

    await page.getByTitle('New Collection').click();
    const cDialog = page.getByRole('dialog');
    await cDialog.getByPlaceholder('Collection name').fill('WS Real');
    await cDialog.getByPlaceholder('Collection name').press('Enter');

    const collectionItem = page.locator('aside').getByText('WS Real');
    await expect(collectionItem).toBeVisible();
    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    const reqDialog = page.getByRole('dialog');
    await reqDialog.getByRole('button', { name: 'WebSocket' }).click();
    await reqDialog.getByPlaceholder('Request name').fill('Echo');
    await reqDialog.getByPlaceholder('Request name').press('Enter');

    await page.getByPlaceholder('ws://… or wss://…').fill('ws://127.0.0.1:9876');

    await page.getByRole('button', { name: 'Headers', exact: true }).click();
    await page.getByPlaceholder('Header name').fill('X-Test');
    await page.getByPlaceholder('Value').fill('{{v}}');
    await page.getByPlaceholder('Value').press('Enter');

    await page.getByRole('button', { name: 'Scripts', exact: true }).click();
    await page.locator('.cm-content').first().fill("pm.environment.set('v', 'from-script')");

    await page.getByRole('button', { name: 'Messages', exact: true }).click();
    // The ping interval is read while the dial is prepared, so it is stored first.
    await page.getByRole('button', { name: 'Socket settings' }).click();
    await page.getByRole('spinbutton').fill('2');
    await page.getByRole('button', { name: 'Apply' }).click();
    await expect(page.getByRole('button', { name: 'Apply' })).toHaveCount(0);

    await page.getByRole('button', { name: 'Connect' }).click();
    await expect(page.getByRole('button', { name: 'Disconnect' })).toBeVisible();
    await expect(page.getByTestId('ws-status')).toContainText('Connected');

    // wsecho's welcome frame reports the handshake it saw: the {{v}} header was
    // resolved with the value the pre-connect script wrote.
    const log = page.getByTestId('ws-log');
    await expect(log).toContainText('"X-Test":"from-script"');

    const compose = page.locator('[aria-placeholder="Message"]');
    await compose.click();
    await compose.fill('{"hello":"{{v}}"}');
    await page.getByRole('button', { name: 'Send' }).click();
    await expect(log).toContainText('echo: {"hello":"from-script"}');

    await page.getByRole('button', { name: 'Binary', exact: true }).click();
    const binary = page.getByLabel('Binary message (base64)');
    await binary.fill('aGVsbG8=');
    await page.getByRole('button', { name: 'Send' }).click();
    // Outgoing and the byte-for-byte echo, both tagged as binary.
    await expect(log.getByText('aGVsbG8=')).toHaveCount(2);
    await expect(log.getByText('bin')).toHaveCount(2);

    // wsecho ticks every 5 s; the 2 s keepalive ping keeps the socket alive
    // instead of tearing it down.
    await expect(log).toContainText('tick 1', { timeout: 20_000 });
    await expect(page.getByTestId('ws-status')).toContainText('Connected');

    await page.getByRole('button', { name: 'Disconnect' }).click();
    await expect(page.getByRole('button', { name: 'Connect' })).toBeVisible();

    await page.getByRole('button', { name: 'History' }).click();
    const row = page.locator('aside div.group').filter({ hasText: 'ws://127.0.0.1:9876' }).first();
    await expect(row).toContainText('WS');
    await expect(row).toContainText('101');
    await expect(row).toContainText(/\d+ms/);
  });
});
