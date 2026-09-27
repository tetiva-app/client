import { test, expect, type Page } from '@playwright/test';

type Protocol = 'HTTP' | 'gRPC';

async function createCollection(page: Page, name: string) {
  await page.goto('/');
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Collection name').fill(name);
  await dialog.getByPlaceholder('Collection name').press('Enter');
  const item = page.locator('aside').getByText(name);
  await expect(item).toBeVisible();
  await item.click();
}

async function addRequest(page: Page, collection: string, name: string, protocol: Protocol = 'HTTP') {
  await page.locator('aside').getByText(collection).click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: protocol, exact: true }).click();
  await dialog.getByPlaceholder('Request name').fill(name);
  await dialog.getByPlaceholder('Request name').press('Enter');
}

const tabStrip = (page: Page) => page.getByRole('button', { name: /^Docs\b/ }).locator('..');
const tab = (page: Page, label: string) => tabStrip(page).getByRole('button', { name: new RegExp(`^${label}\\b`) });
const list = (page: Page) => page.getByTestId('example-list');
const requestTab = (page: Page, name: string) => page.locator('div.group', { hasText: name });

async function openExamples(page: Page, protocol: Protocol = 'HTTP') {
  await createCollection(page, 'Examples');
  await addRequest(page, 'Examples', 'First', protocol);
  await tab(page, 'Examples').click();
}

// Mock services are module singletons, so the page can reach the "backend" the app talks to.
async function deleteOnBackend(page: Page, name: string) {
  await page.evaluate(async (exampleName) => {
    const { getExampleService } = await import('/src/services/index.ts');
    const pinia = (document.querySelector('#app') as any).__vue_app__.config.globalProperties.$pinia;
    const loaded = Object.values(pinia._s.get('examples').byRequest).flat() as { id: string; name: string; version: number }[];
    const found = loaded.find(e => e.name === exampleName)!;
    await (await getExampleService()).delete({ id: found.id, version: found.version });
  }, name);
}

test.describe('Examples tab', () => {
  test('New example stays local until it is saved', async ({ page }) => {
    await openExamples(page);

    await page.getByTestId('example-new-empty').click();

    await expect(list(page)).toContainText('New example');
    await expect(list(page)).toContainText('Not saved yet');
    await expect(tab(page, 'Examples')).not.toContainText('(1)');

    await page.getByTestId('example-save').click();

    await expect(list(page)).not.toContainText('Not saved yet');
    await expect(tab(page, 'Examples')).toContainText('(1)');
  });

  test('discarding an untouched new example asks nothing and leaves no trace', async ({ page }) => {
    await openExamples(page);
    await page.getByTestId('example-new-empty').click();

    await page.getByRole('button', { name: 'Discard example' }).click();

    await expect(page.getByText('No examples yet.')).toBeVisible();
    await expect(page.getByRole('alertdialog')).toHaveCount(0);
  });

  test('an edited example marks the Examples tab and the request tab until it is saved on leave', async ({ page }) => {
    await openExamples(page);
    await page.getByTestId('example-new-empty').click();
    await page.getByRole('textbox', { name: 'Example name' }).fill('Typed');

    await expect(tab(page, 'Examples').getByTitle('Unsaved examples')).toBeVisible();
    await expect(tab(page, 'Examples')).toHaveAccessibleName(/unsaved/);
    await expect(requestTab(page, 'First').getByTitle('Unsaved changes')).toBeVisible();

    await addRequest(page, 'Examples', 'Second');
    await expect(requestTab(page, 'Second')).toBeVisible();
    await requestTab(page, 'First').click();
    await tab(page, 'Examples').click();

    await expect(list(page)).toContainText('Typed');
    await expect(list(page)).not.toContainText('Not saved yet');
    await expect(tab(page, 'Examples')).toContainText('(1)');
    await expect(tab(page, 'Examples').getByTitle('Unsaved examples')).toHaveCount(0);
    await expect(requestTab(page, 'First').getByTitle('Unsaved changes')).toHaveCount(0);
  });

  test('a draft whose example was deleted elsewhere stays listed and saves as a new example', async ({ page }) => {
    await openExamples(page);
    await page.getByTestId('example-new-empty').click();
    await page.getByRole('textbox', { name: 'Example name' }).fill('Doomed');
    await page.getByTestId('example-save').click();
    await expect(tab(page, 'Examples')).toContainText('(1)');

    await page.getByRole('textbox', { name: 'Status text' }).fill('Still mine');
    await deleteOnBackend(page, 'Doomed');
    await page.keyboard.press('Meta+s');

    await expect(list(page)).toContainText('Deleted elsewhere');
    await expect(page.getByTestId('example-deleted-elsewhere')).toBeVisible();
    await expect(tab(page, 'Examples')).not.toContainText('(1)');

    await page.getByTestId('example-deleted-elsewhere').getByRole('button', { name: 'Save as new' }).click();

    await expect(list(page)).not.toContainText('Deleted elsewhere');
    await expect(tab(page, 'Examples')).toContainText('(1)');
    await expect(page.getByRole('textbox', { name: 'Status text' })).toHaveValue('Still mine');
  });

  test('the list marks the selected example and says which ones are unsaved', async ({ page }) => {
    await openExamples(page);
    await page.getByTestId('example-new-empty').click();
    await page.getByRole('textbox', { name: 'Example name' }).fill('Picked');

    const row = list(page).getByRole('button', { name: /Picked/ });
    await expect(row).toHaveAttribute('aria-current', 'true');
    await expect(row).toHaveAccessibleName(/unsaved/);

    await page.getByTestId('example-save').click();
    await expect(row).not.toHaveAccessibleName(/unsaved/);
  });

  test('gRPC examples show the status name next to its code', async ({ page }) => {
    await openExamples(page, 'gRPC');
    await page.getByTestId('example-new-empty').click();

    await expect(list(page)).toContainText('OK (0)');
    await page.getByRole('spinbutton', { name: 'gRPC status code' }).fill('5');
    await expect(list(page)).toContainText('NOT_FOUND (5)');
  });

  test('deleting in a workspace that is not synced does not mention anyone else', async ({ page }) => {
    await openExamples(page);
    await page.getByTestId('example-new-empty').click();
    await page.getByTestId('example-save').click();
    await expect(tab(page, 'Examples')).toContainText('(1)');

    await page.getByRole('button', { name: 'Delete example' }).click();

    const dialog = page.getByRole('alertdialog');
    await expect(dialog).toContainText('"New example" will be deleted.');
    await expect(dialog).not.toContainText('everyone');
  });
});

test.describe('Save as example', () => {
  async function sent(page: Page, url: string) {
    await createCollection(page, 'Responses');
    await addRequest(page, 'Responses', 'Sent');
    await page.locator('[aria-placeholder="Enter request URL"]').fill(url);
    await page.locator('button', { hasText: 'Send' }).click();
  }

  test('naming offers visible Save and Cancel and says what Enter and Esc do', async ({ page }) => {
    await sent(page, 'https://api.example.com/users');
    await page.getByTestId('save-as-example').click();

    const name = page.getByRole('textbox', { name: 'Example name' });
    await expect(name).toHaveAccessibleDescription('Press Enter to save or Escape to cancel');
    await expect(page.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Save', exact: true }).click();

    await expect(name).toHaveCount(0);
    await tab(page, 'Examples').click();
    await expect(list(page)).toContainText('200 OK');
  });

  test('Cancel closes the name field without saving', async ({ page }) => {
    await sent(page, 'https://api.example.com/users');
    await page.getByTestId('save-as-example').click();

    await page.getByRole('button', { name: 'Cancel', exact: true }).click();

    await expect(page.getByRole('textbox', { name: 'Example name' })).toHaveCount(0);
    await expect(tab(page, 'Examples')).not.toContainText('(1)');
  });

  test('a disabled button tells assistive tech why', async ({ page }) => {
    await sent(page, 'https://api.example.com/binary');

    await expect(page.getByTestId('save-as-example')).toBeDisabled();
    await expect(page.getByTestId('save-as-example')).toHaveAccessibleDescription("Binary responses can't be saved as examples");
  });
});
