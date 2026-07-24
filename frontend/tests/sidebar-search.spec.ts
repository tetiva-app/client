import { test, expect, type Page } from '@playwright/test'

async function createCollection(page: Page, name: string) {
  await page.getByTitle('New Collection').click()
  const dialog = page.getByRole('dialog')
  const input = dialog.getByPlaceholder('Collection name')
  await input.fill(name)
  await input.press('Enter')
  await expect(dialog).toBeHidden()
}

test.describe('Sidebar search', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('filters tree and highlights match', async ({ page }) => {
    await createCollection(page, 'Authorization')
    await createCollection(page, 'Users')

    const sidebar = page.locator('aside')
    await expect(sidebar.getByText('Authorization')).toBeVisible()
    await expect(sidebar.getByText('Users', { exact: true })).toBeVisible()

    await page.getByPlaceholder('Search').fill('auth')

    await expect(sidebar.getByText('Users', { exact: true })).toHaveCount(0)

    const authRow = sidebar.getByText('Authorization', { exact: false })
    await expect(authRow).toBeVisible()
    await expect(sidebar.locator('mark')).toHaveCount(1)
    await expect(sidebar.locator('mark')).toHaveText('Auth')
  })

  test('Esc clears search and restores tree', async ({ page }) => {
    await createCollection(page, 'Authorization')
    await createCollection(page, 'Users')

    const sidebar = page.locator('aside')
    const input = page.getByPlaceholder('Search')
    await input.fill('auth')
    await expect(sidebar.getByText('Users', { exact: true })).toHaveCount(0)

    await input.press('Escape')
    await expect(input).toHaveValue('')
    await expect(sidebar.getByText('Users', { exact: true })).toBeVisible()
    await expect(sidebar.getByText('Authorization')).toBeVisible()
    await expect(sidebar.locator('mark')).toHaveCount(0)
  })

  test('shows No matches banner', async ({ page }) => {
    await createCollection(page, 'Authorization')
    await page.getByPlaceholder('Search').fill('nonexistent')
    await expect(page.getByText('No matches')).toBeVisible()
  })

  test('ignores queries shorter than 2 chars', async ({ page }) => {
    await createCollection(page, 'Authorization')
    await createCollection(page, 'Users')
    const sidebar = page.locator('aside')

    await page.getByPlaceholder('Search').fill('a')

    await expect(sidebar.getByText('Authorization')).toBeVisible()
    await expect(sidebar.getByText('Users', { exact: true })).toBeVisible()
    await expect(sidebar.locator('mark')).toHaveCount(0)
  })

  test('Cmd/Ctrl+F focuses sidebar search input', async ({ page }) => {
    await createCollection(page, 'Authorization')
    // focus must start outside the input for the assertion to mean anything
    await page.locator('body').click()
    await page.locator('body').press('ControlOrMeta+f')
    const input = page.getByPlaceholder('Search')
    await expect(input).toBeFocused()
  })
})
