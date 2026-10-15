import { test, expect, type Page } from '@playwright/test';

type Protocol = 'HTTP' | 'GraphQL';

const urlField = (page: Page, protocol: Protocol) => ({
  HTTP: page.locator('[aria-placeholder="Enter request URL"]'),
  GraphQL: page.getByPlaceholder('https://api.example.com/graphql'),
})[protocol];

async function createRequest(page: Page, url: string, protocol: Protocol = 'HTTP') {
  await page.goto('/');

  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill('Orders');
  await dialog.getByPlaceholder('Collection name').press('Enter');

  const collection = page.locator('aside').getByText('Orders');
  await expect(collection).toBeVisible();
  await collection.click();
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const reqDialog = page.getByRole('dialog');
  await reqDialog.getByRole('button', { name: protocol, exact: true }).click();
  await reqDialog.getByPlaceholder('Request name').fill('Get order');
  await reqDialog.getByPlaceholder('Request name').press('Enter');
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await urlField(page, protocol).fill(url);
}

async function send(page: Page, protocol: Protocol = 'HTTP') {
  const label = protocol === 'HTTP' ? 'Send' : 'Query';
  await page.getByRole('button', { name: 'More actions' }).locator('..').getByRole('button', { name: label, exact: true }).click();
}

const compareButton = (page: Page) => page.getByTestId('compare-button').getByRole('button', { name: 'Compare', exact: true });
const chooseBaseline = (page: Page) => page.getByTestId('compare-button').getByRole('button', { name: 'Compare with another response' });
const compareView = (page: Page) => page.getByTestId('response-compare');

async function sendTwice(page: Page, protocol: Protocol = 'HTTP') {
  await send(page, protocol);
  await expect(page.getByText('142ms').first()).toBeVisible();
  await send(page, protocol);
  await expect(page.getByText('318ms').first()).toBeVisible();
}

test.describe('Response compare', () => {
  test('compares an HTTP response with the previous send and survives a re-send', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/orders/1');
    await sendTwice(page);
    await expect(page.getByText('+176')).toBeVisible();

    await compareButton(page).click();

    const view = compareView(page);
    await expect(view).toBeVisible();
    await expect(page.getByTestId('compare-collapsed')).toContainText('Request editor is collapsed while comparing');
    await expect(view.locator('[data-side="A"]')).toContainText('142ms');
    await expect(view.locator('[data-side="B"]')).toContainText('318ms');
    await expect(view.getByTestId('compare-nav')).toContainText('Change 1 of 1');
    await expect(view.locator('.cm-mergeView')).toBeVisible();
    await expect(view.getByRole('button', { name: 'Split' })).toHaveAttribute('aria-pressed', 'true');

    await view.getByRole('button', { name: 'Next change' }).click();
    await view.getByRole('button', { name: 'Previous change' }).click();
    await expect(view.getByTestId('compare-nav')).toContainText('Change 1 of 1');

    await view.getByRole('button', { name: 'Unified' }).click();
    await expect(view.getByRole('button', { name: 'Unified' })).toHaveAttribute('aria-pressed', 'true');
    await expect(view.locator('.cm-mergeView')).toHaveCount(0);
    await expect(view.getByTestId('diff-pane')).toBeVisible();

    await view.getByRole('switch').click();
    await expect(view.getByRole('switch')).toHaveAttribute('aria-checked', 'false');
    await expect(view.getByText('Masked:')).toHaveCount(0);

    await send(page);
    await expect(view.locator('[data-side="B"]')).toContainText('494ms');
    await expect(view).toBeVisible();

    await view.getByRole('button', { name: 'Close compare' }).click();
    await expect(view).toHaveCount(0);
    await expect(page.locator('[data-response-body]')).toBeVisible();
  });

  test('compares a GraphQL response with the previous send', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/graphql', 'GraphQL');
    await sendTwice(page, 'GraphQL');

    await compareButton(page).click();

    await expect(compareView(page).getByTestId('compare-nav')).toContainText('Change 1 of 1');
  });

  test('explains that the first response has nothing to compare with', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/orders/1');
    await send(page);
    await expect(page.getByText('142ms')).toBeVisible();

    await compareButton(page).click();

    const popover = page.getByTestId('compare-popover');
    await expect(popover).toContainText('This is the first response of “Get order”.');
    await expect(popover).toContainText('None for this request.');
    await expect(compareView(page)).toHaveCount(0);
  });

  test('compares two History records, request first', async ({ page }) => {
    await createRequest(page, 'https://api.example.com/orders/1');
    await sendTwice(page);

    await chooseBaseline(page).click();
    const popover = page.getByTestId('compare-popover');
    await expect(popover.locator('[data-row="send"]')).toHaveCount(1);
    await popover.getByRole('button', { name: /All 2 sends in History/ }).click();

    await expect(page.getByText(/-click: pick two/)).toBeVisible();
    const records = page.locator('[data-history-item]');
    await expect(records).toHaveCount(2);
    await records.nth(0).click({ modifiers: ['ControlOrMeta'] });
    await records.nth(1).click({ modifiers: ['ControlOrMeta'] });
    await expect(page.getByText('2 selected')).toBeVisible();
    await page.getByTestId('history-compare').click();

    const pair = page.getByTestId('history-compare-view');
    await expect(pair).toContainText('Compare · Get order');
    await expect(pair.getByTestId('request-diff')).toBeVisible();
    await expect(pair).toContainText('The compared request fields match.');

    await pair.getByRole('button', { name: 'Open response body' }).click();
    await expect(pair.getByTestId('diff-pane')).toBeVisible();

    await pair.getByRole('button', { name: 'Close compare' }).click();
    await expect(pair).toHaveCount(0);
  });

  test('opens Unified on a narrow window', async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 700 });
    await createRequest(page, 'https://api.example.com/orders/1');
    await sendTwice(page);

    await compareButton(page).click();

    const view = compareView(page);
    await expect(view.getByRole('button', { name: 'Unified' })).toHaveAttribute('aria-pressed', 'true');
    await expect(view.locator('.cm-mergeView')).toHaveCount(0);
  });
});
