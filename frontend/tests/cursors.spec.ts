import { test, expect, type Locator, type Page } from '@playwright/test';

// What the pointer shows over each control: the element under its centre decides, so a disabled
// button that lets the pointer through to a wrapper is judged by that wrapper.
async function expectCursors(scope: Locator, label: string) {
  await expect(scope.first()).toBeVisible();
  const wrong = await scope.first().evaluate((root) => {
    const controls = 'button, [role="button"], [role="tab"], [role="menuitem"], [role="option"], [role="combobox"],'
      + ' [role="switch"], [role="radio"], input[type="checkbox"], select, a[href], label';
    const disabled = ':disabled, [aria-disabled="true"], [data-disabled]';
    const out: string[] = [];
    for (const el of root.querySelectorAll<HTMLElement>(controls)) {
      const box = el.getBoundingClientRect();
      if (box.width === 0 || box.height === 0) continue;
      const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
      if (!hit || !(el.contains(hit) || hit.contains(el))) continue;
      const off = el.matches(disabled) || (el.tagName === 'LABEL' && el.querySelector(disabled) !== null);
      const want = off ? 'not-allowed' : 'pointer';
      const got = getComputedStyle(hit).cursor;
      if (got !== want) {
        const name = (el.getAttribute('aria-label') || el.textContent || el.tagName).trim().replace(/\s+/g, ' ').slice(0, 40);
        out.push(`${el.tagName.toLowerCase()} "${name}": ${got}, want ${want}`);
      }
    }
    return out;
  });
  expect(wrong, label).toEqual([]);
}

async function createCollection(page: Page, name: string) {
  await page.goto('/');
  await page.getByTitle('New Collection').click();
  const input = page.getByRole('dialog').getByPlaceholder('Collection name');
  await input.fill(name);
  await input.press('Enter');
  await expect(page.locator('aside').getByText(name)).toBeVisible();
}

async function addRequest(page: Page, collection: string, name: string) {
  await page.locator('aside').getByText(collection).click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();
  const input = page.getByRole('dialog').getByPlaceholder('Request name');
  await input.fill(name);
  await input.press('Enter');
}

const requestTab = (page: Page, label: string) =>
  page.getByRole('button', { name: /^Docs\b/ }).locator('..').getByRole('button', { name: new RegExp(`^${label}\\b`) });

// Mock services are module singletons, so the page can hand the "backend" a status to report.
async function reportPublication(page: Page, collection: string, status: Record<string, unknown>) {
  await page.evaluate(async ([name, over]) => {
    const { useCollectionStore } = await import('/src/stores/collections.ts');
    const { usePublicationsStore } = await import('/src/stores/publications.ts');
    const { getPublicationService } = await import('/src/services/index.ts');
    const { emptyPublicationStatus } = await import('/src/services/mock-publication.ts');
    const id = [...useCollectionStore().collectionsMap.values()].find(c => c.name === name)!.id;
    const service = await getPublicationService() as unknown as { setStatus(id: string, st: unknown): void };
    service.setStatus(id, { ...emptyPublicationStatus(), ...over });
    await usePublicationsStore().refresh(id);
  }, [collection, status] as const);
}

const PUBLISHED = {
  published: true, canManage: true, slug: 'petstore', publicUrl: 'https://share.tetiva.app/petstore', visibility: 'public',
  revision: 2, hasChanges: 'yes',
  settings: { environmentId: '', environmentName: '', environmentMissing: false, includeScripts: true, publishAsIs: [] },
};

test.describe('Pointer over the controls of the new screens', () => {
  test('Code tab and its language list', async ({ page }) => {
    await createCollection(page, 'Code');
    await addRequest(page, 'Code', 'List');
    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');
    await requestTab(page, 'Code').click();
    await expect(page.getByRole('textbox', { name: 'Generated code' })).toContainText('curl');

    await expectCursors(page.getByTestId('code-snippet-panel'), 'code toolbar');
    await page.getByRole('switch', { name: 'Resolve variables' }).click();
    await expectCursors(page.getByTestId('code-snippet-panel'), 'code toolbar with secrets locked');

    await page.getByRole('combobox', { name: 'Language' }).click();
    await expectCursors(page.getByRole('listbox'), 'language list');
  });

  test('Examples tab, its editor and Save as example', async ({ page }) => {
    await createCollection(page, 'Examples');
    await addRequest(page, 'Examples', 'Sent');
    await requestTab(page, 'Examples').click();
    await expectCursors(page.getByTestId('examples-panel'), 'no examples');

    await page.getByTestId('example-new-empty').click();
    await expectCursors(page.getByTestId('examples-panel'), 'new example');
    await page.getByTestId('example-save').click();
    await expect(page.getByTestId('example-save')).toBeDisabled();
    await expectCursors(page.getByTestId('examples-panel'), 'saved example');

    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/binary');
    await page.locator('button', { hasText: 'Send' }).click();
    await expect(page.getByTestId('save-as-example')).toBeDisabled();
    await expectCursors(page.getByTestId('save-as-example').locator('xpath=../..'), 'binary response');

    await page.locator('[aria-placeholder="Enter request URL"]').fill('https://api.example.com/users');
    await page.locator('button', { hasText: 'Send' }).click();
    await expect(page.getByTestId('save-as-example')).toBeEnabled();
    await expectCursors(page.getByTestId('save-as-example').locator('xpath=../..'), 'json response');
    await page.getByTestId('save-as-example').click();
    await expectCursors(page.getByRole('textbox', { name: 'Example name' }).locator('..'), 'naming the example');
  });

  test('collection menu, Publish tab and its states', async ({ page }) => {
    await createCollection(page, 'Petstore');
    await page.locator('aside').getByText('Petstore').click({ button: 'right' });
    await expectCursors(page.getByRole('menu'), 'collection menu');
    await page.getByRole('menu').getByText('Open Details').click();

    await expectCursors(page.getByRole('tablist'), 'collection sections');
    await page.getByRole('tab', { name: /Publish/ }).click();
    await expectCursors(page.getByTestId('publication-panel'), 'not published');

    await reportPublication(page, 'Petstore', { available: false, reasonUnavailable: 'not_logged_in' });
    await expect(page.getByTestId('publication-signin')).toBeVisible();
    await expectCursors(page.getByTestId('publication-panel'), 'signed out');

    await reportPublication(page, 'Petstore', PUBLISHED);
    await expect(page.getByTestId('publication-published')).toBeVisible();
    await expectCursors(page.getByTestId('publication-panel'), 'published');

    await page.getByTestId('publication-unpublish').click();
    await expectCursors(page.getByRole('alertdialog'), 'unpublish confirmation');
  });

  test('publish dialog', async ({ page }) => {
    await createCollection(page, 'Petstore');
    await page.locator('aside').getByText('Petstore').click({ button: 'right' });
    await page.getByRole('menu').getByText('Publish…').click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByTestId('publish-submit')).toBeDisabled();

    await expectCursors(dialog, 'publish dialog, warnings unchecked');
    await dialog.getByTestId('publish-acknowledge').check();
    await expectCursors(dialog, 'publish dialog, ready');
  });

  test('import menu and the import-from-link dialog', async ({ page }) => {
    await page.goto('/');
    await page.getByTitle('Import').click();
    await expectCursors(page.getByRole('menu'), 'import menu');
    await page.getByRole('menu').getByText('Import from Link…').click();

    const dialog = page.getByRole('dialog');
    await expectCursors(dialog, 'import link, empty');
    await dialog.getByTestId('import-link-input').fill('https://share.tetiva.app/petstore');
    await expectCursors(dialog, 'import link, filled');
  });

  test('request tab menu', async ({ page }) => {
    await createCollection(page, 'Tabs');
    await addRequest(page, 'Tabs', 'Detachable');
    await page.locator('div.group', { hasText: 'Detachable' }).first().click({ button: 'right' });
    await expectCursors(page.getByRole('menu'), 'tab menu');
  });
});
