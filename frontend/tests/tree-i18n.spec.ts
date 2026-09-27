import { test, expect, type Page } from '@playwright/test';

async function openInRussian(page: Page) {
  await page.setViewportSize({ width: 960, height: 640 });
  await page.addInitScript(() => {
    const key = 'gophercourier.settings';
    const saved = JSON.parse(localStorage.getItem(key) ?? '{}');
    localStorage.setItem(key, JSON.stringify({ ...saved, language: 'ru' }));
  });
  await page.goto('/');
}

async function createCollection(page: Page, name: string) {
  await page.getByTitle('Новая коллекция').click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Новая коллекция');
  await dialog.getByPlaceholder('Название коллекции').fill(name);
  await dialog.getByPlaceholder('Название коллекции').press('Enter');
  const item = page.locator('aside').getByText(name);
  await expect(item).toBeVisible();
  return item;
}

test.describe('Collection tree in Russian', () => {
  test('words the header and the empty tree', async ({ page }) => {
    await openInRussian(page);
    const sidebar = page.locator('aside');

    await expect(sidebar).toContainText('Коллекций пока нет. Нажмите +, чтобы создать.');
    await expect(sidebar.getByPlaceholder('Поиск')).toBeVisible();
    await expect(sidebar.getByTitle('Новая коллекция')).toBeVisible();

    await sidebar.getByTitle('Импорт').click();
    const menu = page.getByRole('menu');
    await expect(menu.getByRole('menuitem')).toHaveText(['Импорт из файла…', 'Импорт по ссылке…']);
    await page.keyboard.press('Escape');

    await sidebar.getByPlaceholder('Поиск').fill('nothing');
    await expect(sidebar.getByTitle('Очистить поиск')).toBeVisible();
  });

  test('keeps every menu item on one line', async ({ page }) => {
    await openInRussian(page);
    const root = await createCollection(page, 'Petstore API');

    await root.click({ button: 'right' });
    const menu = page.getByRole('menu');
    await expect(menu.getByText('Новая подколлекция')).toBeVisible();
    await expect(menu.getByText('Опубликовать…')).toBeVisible();
    // The menu zooms in; a box measured mid-animation is scaled down.
    await menu.evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)));

    const items = await menu.getByRole('menuitem').evaluateAll((els) =>
      els.map((el) => ({ text: el.textContent?.trim(), height: el.getBoundingClientRect().height })));
    expect(items.map((i) => i.text)).toEqual([
      'Новый запрос', 'Новая подколлекция', 'Переименовать', 'Открыть коллекцию', 'Опубликовать…',
      'Импорт из файла…', 'Экспорт в Postman', 'Переместить…', 'Удалить',
    ]);
    const single = items[0].height;
    for (const item of items) expect(item.height, item.text).toBeCloseTo(single, 0);
    expect((await menu.boundingBox())!.width).toBeGreaterThanOrEqual(192);
  });

  test('asks before deleting and renames in Russian', async ({ page }) => {
    await openInRussian(page);
    const root = await createCollection(page, 'Petstore API');

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Переименовать').click();
    const rename = page.getByRole('dialog');
    await expect(rename).toContainText('Переименовать коллекцию');
    await expect(rename).toContainText('Введите новое название для «Petstore API».');
    await rename.getByRole('button', { name: 'Отмена' }).click();
    await expect(rename).toBeHidden();

    await root.click({ button: 'right' });
    await page.getByRole('menu').getByText('Удалить').click();
    const confirm = page.getByRole('alertdialog');
    await expect(confirm).toContainText('Удалить коллекцию');
    await expect(confirm).toContainText('Удалить «Petstore API» со всем содержимым?');
    await confirm.getByRole('button', { name: 'Удалить' }).click();
    await expect(page.locator('aside').getByText('Petstore API')).toHaveCount(0);
  });
});
