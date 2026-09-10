import { test, expect, type Page } from '@playwright/test';

const DOCS_PLACEHOLDER = '[aria-placeholder^="Document this request"]';
const COLLECTION_PLACEHOLDER = '[aria-placeholder^="Add a description for this collection"]';

async function createCollection(page: Page, name: string) {
  await page.goto('/');
  await page.getByTitle('New Collection').click();
  const dialog = page.getByRole('dialog');
  const input = dialog.getByPlaceholder('Collection name');
  await input.fill(name);
  await input.press('Enter');

  const item = page.locator('aside').getByText(name);
  await expect(item).toBeVisible();
  return item;
}

async function openRequestDocs(page: Page, collectionName: string, requestName: string) {
  const collection = await createCollection(page, collectionName);
  await collection.click();
  await collection.click({ button: 'right' });
  await page.getByRole('menu').getByText('New Request').click();

  const dialog = page.getByRole('dialog');
  const input = dialog.getByPlaceholder('Request name');
  await input.fill(requestName);
  await input.press('Enter');

  await page.getByRole('button', { name: 'Docs', exact: true }).click();
}

type Rgba = { r: number; g: number; b: number; a: number };

// Chromium serialises color-mix() as `color(srgb <0..1> …)`, plain colors as rgb(a).
function parseColor(value: string): Rgba {
  const n = (value.match(/-?[\d.]+/g) ?? []).map(Number);
  const scale = value.startsWith('color(') ? 255 : 1;
  return { r: n[0] * scale, g: n[1] * scale, b: n[2] * scale, a: n[3] ?? 1 };
}

function luminance(c: Rgba): number {
  const [r, g, b] = [c.r, c.g, c.b].map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const la = luminance(parseColor(a));
  const lb = luminance(parseColor(b));
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

async function openSlashMenu(page: Page, collectionName: string) {
  await openRequestDocs(page, collectionName, 'Get Users');
  await page.getByRole('button', { name: 'Add description' }).click();
  await page.locator(DOCS_PLACEHOLDER).click();
  await page.keyboard.type('/');
  await expect(page.getByRole('option').first()).toBeVisible();
  // CodeMirror drops navigation keys for its first interactionDelay (75ms),
  // and an ignored ArrowDown falls through to cursorLineDown, closing the menu.
  await page.waitForTimeout(200);
}

function menuColors(page: Page) {
  return page.evaluate(() => {
    const tooltip = document.querySelector('.cm-tooltip-autocomplete') as HTMLElement;
    const items = Array.from(tooltip.querySelectorAll('li')) as HTMLElement[];
    const selected = items.find((li) => li.hasAttribute('aria-selected'))!;
    const plain = items.find((li) => !li.hasAttribute('aria-selected'))!;
    const detail = selected.querySelector('.cm-completionDetail') as HTMLElement;
    return {
      selectedLabel: selected.textContent ?? '',
      menuBg: getComputedStyle(tooltip).backgroundColor,
      selectedBg: getComputedStyle(selected).backgroundColor,
      selectedFg: getComputedStyle(selected).color,
      selectedDetailFg: getComputedStyle(detail).color,
      plainBg: getComputedStyle(plain).backgroundColor,
    };
  });
}

test.describe('Description editor', () => {
  test('starts on an empty-state button and opens the editor on click', async ({ page }) => {
    await openRequestDocs(page, 'Docs Empty Coll', 'Get Users');

    const addButton = page.getByRole('button', { name: 'Add description' });
    await expect(addButton).toBeVisible();
    await expect(page.locator(DOCS_PLACEHOLDER)).not.toBeVisible();

    await addButton.click();
    await expect(page.locator(DOCS_PLACEHOLDER)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Bold' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Table' })).toBeVisible();
  });

  test('previews by default and only the pencil reopens the editor', async ({ page }) => {
    await openRequestDocs(page, 'Docs Preview Coll', 'Get Users');

    await page.getByRole('button', { name: 'Add description' }).click();
    const editor = page.locator(DOCS_PLACEHOLDER);
    await editor.fill('# Title\n\nSome notes.');

    await page.getByRole('button', { name: 'Preview description' }).click();
    const preview = page.locator('.prose');
    await expect(preview.getByRole('heading', { name: 'Title' })).toBeVisible();
    await expect(editor).not.toBeVisible();

    // Clicking the rendered text must not swallow text selection.
    await preview.getByText('Some notes.').click();
    await expect(editor).not.toBeVisible();

    await page.getByRole('button', { name: 'Edit description' }).click();
    await expect(editor).toBeVisible();
  });

  test('toolbar wraps the selection through a CodeMirror transaction', async ({ page }) => {
    await openRequestDocs(page, 'Docs Toolbar Coll', 'Get Users');

    await page.getByRole('button', { name: 'Add description' }).click();
    const editor = page.locator(DOCS_PLACEHOLDER);
    await editor.fill('hello');
    await page.keyboard.press('ControlOrMeta+a');
    await page.getByRole('button', { name: 'Bold' }).click();
    await expect(editor).toHaveText('**hello**');

    await page.getByRole('button', { name: 'Bullet list' }).click();
    await expect(editor).toHaveText('- **hello**');

    // Enter continues the list; Cmd+Enter belongs to Send, not to the editor.
    await page.keyboard.press('ControlOrMeta+ArrowRight');
    await page.keyboard.press('Enter');
    await page.keyboard.type('two');
    await expect(editor).toHaveText('- **hello**- two');

    await page.keyboard.press('ControlOrMeta+Enter');
    await expect(editor).toHaveText('- **hello**- two');
  });

  test('collection overview gets the same editor', async ({ page }) => {
    const collection = await createCollection(page, 'Docs Coll Overview');
    await collection.dblclick();

    await page.getByRole('button', { name: 'Add description' }).click();
    const editor = page.locator(COLLECTION_PLACEHOLDER);
    await editor.fill('- one\n- two');

    await page.getByRole('button', { name: 'Preview description' }).click();
    await expect(page.locator('.prose li')).toHaveCount(2);
  });

  test('slash menu opens on /heading and orders commands by usefulness', async ({ page }) => {
    await openSlashMenu(page, 'Docs Slash Order Coll');

    await expect(page.getByRole('option')).toHaveText([
      /^\/heading/, /^\/list/, /^\/code/, /^\/table/, /^\/quote/, /^\/link/,
    ]);
    expect((await menuColors(page)).selectedLabel).toContain('/heading');
  });

  test('the selected slash command is visibly highlighted in dark theme', async ({ page }) => {
    await openSlashMenu(page, 'Docs Slash Dark Coll');
    await page.keyboard.press('ArrowDown');

    const c = await menuColors(page);
    expect(c.selectedLabel).toContain('/list');
    expect(parseColor(c.selectedBg).a).toBeGreaterThan(0.5);
    expect(c.selectedBg).not.toBe(c.plainBg);
    // --accent equals --popover in this theme, so a token mix-up made the
    // highlight vanish while aria-selected kept moving.
    expect(contrast(c.selectedBg, c.menuBg)).toBeGreaterThan(2);
    expect(contrast(c.selectedFg, c.selectedBg)).toBeGreaterThan(4.5);
    expect(contrast(c.selectedDetailFg, c.selectedBg)).toBeGreaterThan(4.5);
  });

  test('hovering a slash command tints it without imitating the selection', async ({ page }) => {
    await openSlashMenu(page, 'Docs Slash Hover Coll');

    const hovered = page.getByRole('option').nth(2);
    await hovered.hover();
    const bg = await hovered.evaluate((el) => getComputedStyle(el).backgroundColor);
    const c = await menuColors(page);

    expect(parseColor(bg).a).toBeGreaterThan(0);
    expect(bg).not.toBe(c.plainBg);
    expect(bg).not.toBe(c.selectedBg);
  });

  test.describe('light theme', () => {
    test.use({ colorScheme: 'light' });

    test('the selected slash command is visibly highlighted', async ({ page }) => {
      await openSlashMenu(page, 'Docs Slash Light Coll');
      await page.keyboard.press('ArrowDown');

      const c = await menuColors(page);
      expect(parseColor(c.selectedBg).a).toBeGreaterThan(0.5);
      expect(c.selectedBg).not.toBe(c.plainBg);
      expect(contrast(c.selectedBg, c.menuBg)).toBeGreaterThan(2);
      expect(contrast(c.selectedFg, c.selectedBg)).toBeGreaterThan(4.5);
      expect(contrast(c.selectedDetailFg, c.selectedBg)).toBeGreaterThan(4.5);
    });
  });
});

async function startDescription(page: Page, collection: string) {
  await openRequestDocs(page, collection, 'Get Users');
  await page.getByRole('button', { name: 'Add description' }).click();
  await page.locator(DOCS_PLACEHOLDER).click();
}

// CodeMirror renders the kernel's padding spaces as NBSP.
async function editorText(page: Page) {
  return (await page.locator(DOCS_PLACEHOLDER).innerText()).replace(/\u00a0/g, ' ');
}

async function pasteText(page: Page, text: string) {
  await page.locator(DOCS_PLACEHOLDER).evaluate((el, value) => {
    const data = new DataTransfer();
    data.setData('text/plain', value);
    el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
  }, text);
}

test.describe('description tables', () => {
  test('/table opens the size picker and inserts a formatted table', async ({ page }) => {
    await startDescription(page, 'Tables A');
    await page.keyboard.type('/tab');
    await expect(page.getByRole('option', { name: /\/table/ })).toBeVisible();
    // CodeMirror ignores Enter for its first interactionDelay (75ms) after the
    // menu opens, which a synthetic keystroke would otherwise beat.
    await page.waitForTimeout(200);
    await page.keyboard.press('Enter');
    await page.getByRole('gridcell', { name: '3 × 2 table' }).click();
    expect(await editorText(page)).toContain('| Column 1 | Column 2 | Column 3 |');
    await expect(page.locator(DOCS_PLACEHOLDER)).toBeFocused();
    await page.keyboard.type('Name');
    expect(await editorText(page)).toContain('| Name | Column 2 | Column 3 |');
    // Plain typing leaves the delimiter at the old width; the next Tab reformats it.
    expect(await editorText(page)).toContain('| -------- | -------- | -------- |');
    await page.keyboard.press('Tab');
    expect(await editorText(page)).toContain('| ---- | -------- | -------- |');
  });

  test('Tab and Enter navigate cells and keep columns aligned', async ({ page }) => {
    await startDescription(page, 'Tables B');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();
    await page.keyboard.type('Name');
    await page.keyboard.press('Tab');
    await page.keyboard.type('Age');
    await page.keyboard.press('Enter');
    await page.keyboard.type('Ann');
    await page.keyboard.press('Tab');
    await page.keyboard.type('30');
    await page.keyboard.press('Enter');
    const text = await editorText(page);
    expect(text).toContain('| Name | Age |');
    expect(text).toContain('| ---- | --- |');
    expect(text).toContain('| Ann  | 30  |');
    expect(text.trim().split('\n')).toHaveLength(4);
  });

  test('Enter on an empty last row leaves the table', async ({ page }) => {
    await startDescription(page, 'Tables C');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();
    await page.keyboard.press('Enter');
    await page.keyboard.press('Enter');
    await page.keyboard.type('after');
    const lines = (await editorText(page)).trim().split('\n');
    expect(lines).toHaveLength(3);
    expect(lines[0]).toBe('| Column 1 | Column 2 |');
    expect(lines[2]).toBe('after');
  });

  test('table actions appear only inside a table and delete a column', async ({ page }) => {
    await startDescription(page, 'Tables D');
    await expect(page.getByRole('button', { name: 'Table actions' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '3 × 1 table' }).click();
    await page.getByRole('button', { name: 'Table actions' }).click();
    await page.getByRole('menuitem', { name: 'Delete column' }).click();
    expect(await editorText(page)).not.toContain('Column 1');
    expect(await editorText(page)).toContain('| Column 2 | Column 3 |');
    await expect(page.locator(DOCS_PLACEHOLDER)).toBeFocused();
  });

  test('pasting TSV inserts a table', async ({ page }) => {
    await startDescription(page, 'Tables E');
    await pasteText(page, 'name\tage\nAnn\t30');
    expect(await editorText(page)).toContain('| name | age |');
    expect(await editorText(page)).toContain('| Ann  | 30  |');
  });

  test('Tab indents a list item and Escape leaves the editor', async ({ page }) => {
    await startDescription(page, 'Tables F');
    await page.keyboard.type('- item');
    await page.keyboard.press('Tab');
    expect(await editorText(page)).toContain('  - item');
    await page.keyboard.press('Escape');
    await expect(page.locator(DOCS_PLACEHOLDER)).not.toBeFocused();
  });

  test('pasting a URL over a selection makes a link', async ({ page }) => {
    await startDescription(page, 'Tables G');
    await page.keyboard.type('docs');
    await page.keyboard.press('Shift+Home');
    await pasteText(page, 'https://example.com/docs');
    expect(await editorText(page)).toContain('[docs](https://example.com/docs)');
  });

  test('typing a fence keeps it three backticks wide', async ({ page }) => {
    await startDescription(page, 'Tables H');
    await page.keyboard.type('Run:');
    await page.keyboard.press('Enter');
    await page.keyboard.type('```bash');
    // closeBrackets pairs the third backtick unless the editor stops it, and the
    // stray fourth one turns the fence into a paragraph.
    expect((await editorText(page)).split('\n')[1]).toBe('```bash');
    await page.keyboard.press('End');
    await page.keyboard.press('Enter');
    await page.keyboard.type('echo hi');
    expect((await editorText(page)).split('\n')).toEqual(['Run:', '```bash', 'echo hi']);
  });

  test('inline backticks still pair and skip', async ({ page }) => {
    await startDescription(page, 'Tables H2');
    await page.keyboard.type('run `curl');
    expect(await editorText(page)).toBe('run `curl`');
  });

  test('row commands are disabled on the header row', async ({ page }) => {
    await startDescription(page, 'Tables I');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();

    await page.getByRole('button', { name: 'Table actions' }).click();
    await expect(page.getByRole('menuitem', { name: 'Delete row' })).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByRole('menuitem', { name: 'Move row up' })).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByRole('menuitem', { name: 'Delete column' })).not.toHaveAttribute('aria-disabled', 'true');
    await page.keyboard.press('Escape');

    // Enter moves the caret into the body row, where the same commands apply.
    await page.keyboard.press('Enter');
    await page.getByRole('button', { name: 'Table actions' }).click();
    await expect(page.getByRole('menuitem', { name: 'Delete row' })).not.toHaveAttribute('aria-disabled', 'true');
  });

  test('the table menu opens clear of the left half of the editor', async ({ page }) => {
    await startDescription(page, 'Tables J');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '3 × 1 table' }).click();
    await page.getByRole('button', { name: 'Table actions' }).click();

    const menu = (await page.getByRole('menu').boundingBox())!;
    const editor = (await page.locator(DOCS_PLACEHOLDER).boundingBox())!;
    expect(menu.x).toBeGreaterThan(editor.x + editor.width / 2);
  });

  test('inserting a table keeps the selected text', async ({ page }) => {
    await startDescription(page, 'Tables K');
    await page.keyboard.type('important notes');
    await page.keyboard.press('ControlOrMeta+a');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();

    const text = await editorText(page);
    expect(text.startsWith('important notes')).toBe(true);
    expect(text).toContain('| Column 1 | Column 2 |');
    // The blank line between them is what keeps the paragraph out of the table.
    expect(text).not.toContain('notes\n| Column 1');
  });

  test('a table inserted above a paragraph does not swallow it in the preview', async ({ page }) => {
    await startDescription(page, 'Tables K2');
    await page.keyboard.type('intro paragraph');
    await page.keyboard.press('Home');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();
    await page.getByRole('button', { name: 'Preview description' }).click();

    const preview = page.locator('.prose');
    // A single newline after the table makes markdown-it read the paragraph as a row.
    await expect(preview.locator('table tr')).toHaveCount(2);
    await expect(preview.locator('p')).toHaveText(['intro paragraph']);
  });

  test('pasted tab-indented code stays code', async ({ page }) => {
    await startDescription(page, 'Tables L');
    await pasteText(page, '\tif err != nil {\n\t\treturn err\n\t}');
    const text = await editorText(page);
    expect(text).toContain('if err != nil {');
    expect(text).not.toContain('| --- |');
  });

  test('TSV pasted inside an unclosed fence stays plain text', async ({ page }) => {
    await startDescription(page, 'Tables M');
    await page.keyboard.type('```');
    await page.keyboard.press('Enter');
    await pasteText(page, 'name\tage\nAnn\t30');
    const text = await editorText(page);
    expect(text).toContain('name');
    expect(text).not.toContain('| ---- |');
  });

  test('the size grid is a grid and announces the highlighted cell', async ({ page }) => {
    await startDescription(page, 'Tables N');
    await page.getByRole('button', { name: 'Insert table' }).click();

    const grid = page.getByRole('grid', { name: 'Table size' });
    const activeLabel = () => grid.evaluate((el) => {
      const id = el.getAttribute('aria-activedescendant') ?? '';
      return document.getElementById(id)?.getAttribute('aria-label') ?? '';
    });
    expect(await activeLabel()).toBe('3 × 2 table');
    await page.keyboard.press('ArrowRight');
    expect(await activeLabel()).toBe('4 × 2 table');
    await expect(grid.getByRole('gridcell', { name: '4 × 2 table' })).toHaveAttribute('aria-selected', 'true');
    await expect(grid.getByRole('gridcell', { name: '8 × 6 table' })).toHaveAttribute('aria-selected', 'false');
  });

  test('the size grid reads as cells, not dots', async ({ page }) => {
    await startDescription(page, 'Tables O');
    await page.getByRole('button', { name: 'Insert table' }).click();

    const cell = await page.getByRole('gridcell', { name: '8 × 6 table' }).evaluate((el) => {
      const style = getComputedStyle(el);
      const popover = el.closest('[data-slot="popover-content"]') as HTMLElement;
      return {
        radius: parseFloat(style.borderTopLeftRadius),
        size: parseFloat(style.width),
        border: style.borderTopColor,
        popoverBg: getComputedStyle(popover).backgroundColor,
      };
    });
    // A 6px radius on a 16px box renders as a dot; the grid has to look like cells.
    expect(cell.radius).toBeLessThanOrEqual(cell.size / 6);
    // WCAG 1.4.11: the grid's extent is only visible through these borders.
    expect(contrast(cell.border, cell.popoverBg)).toBeGreaterThan(3);
  });
});

// The pipes and the delimiter row are the shape of a table in edit mode, so they are
// read as text, not as decoration.
async function pipeContrast(page: Page) {
  return page.locator(DOCS_PLACEHOLDER).evaluate((editor) => {
    const opaqueBg = (start: HTMLElement): string => {
      for (let el: HTMLElement | null = start; el; el = el.parentElement) {
        const bg = getComputedStyle(el).backgroundColor;
        if (bg && !/^rgba\(0, 0, 0, 0\)$/.test(bg) && bg !== 'transparent') return bg;
      }
      return 'rgb(255, 255, 255)';
    };
    const pipe = Array.from(editor.querySelectorAll('span'))
      .find(span => span.textContent === '|') as HTMLElement;
    return { fg: getComputedStyle(pipe).color, bg: opaqueBg(pipe) };
  });
}

test.describe('table markup contrast', () => {
  test('pipes stay readable in dark theme', async ({ page }) => {
    await startDescription(page, 'Tables P');
    await page.getByRole('button', { name: 'Insert table' }).click();
    await page.getByRole('gridcell', { name: '2 × 1 table' }).click();
    const { fg, bg } = await pipeContrast(page);
    expect(contrast(fg, bg)).toBeGreaterThan(4.5);
  });

  test.describe('light theme', () => {
    test.use({ colorScheme: 'light' });

    test('pipes stay readable', async ({ page }) => {
      await startDescription(page, 'Tables Q');
      await page.getByRole('button', { name: 'Insert table' }).click();
      await page.getByRole('gridcell', { name: '2 × 1 table' }).click();
      const { fg, bg } = await pipeContrast(page);
      expect(contrast(fg, bg)).toBeGreaterThan(4.5);
    });
  });
});
