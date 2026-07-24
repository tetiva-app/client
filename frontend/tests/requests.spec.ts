import { test, expect } from '@playwright/test';

test.describe('Requests and Tabs', () => {
  test('should create requests, open tabs, and allow tab switching and closing', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    const dialog = page.getByRole('dialog');
    const input = dialog.getByPlaceholder('Collection name');
    await input.fill('API Collection');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('API Collection');
    await expect(collectionItem).toBeVisible();

    await collectionItem.click();

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    
    let reqDialog = page.getByRole('dialog');
    await expect(reqDialog).toContainText('New Request');
    let reqInput = reqDialog.getByPlaceholder('Request name');
    await reqInput.fill('Get Users');
    await reqInput.press('Enter');

    const tab1 = page.getByText('Get Users').nth(1); // the text exists in both sidebar and tab bar
    const tabBar = page.locator('.flex.items-center.h-9.border-b'); // Tab bar container from TabBar.vue
    const getUsersTab = tabBar.getByText('Get Users');
    await expect(getUsersTab).toBeVisible();

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    
    reqDialog = page.getByRole('dialog');
    reqInput = reqDialog.getByPlaceholder('Request name');
    await reqInput.fill('Create User');
    await reqInput.press('Enter');

    const createUserTab = tabBar.getByText('Create User');
    await expect(createUserTab).toBeVisible();

    const activeTabContainer = createUserTab.locator('..');
    await expect(activeTabContainer).toHaveClass(/border-b-primary/);

    await getUsersTab.click();
    const getUsersTabContainer = getUsersTab.locator('..');
    await expect(getUsersTabContainer).toHaveClass(/border-b-primary/);
    await expect(activeTabContainer).not.toHaveClass(/border-b-primary/);

    const closeBtn = activeTabContainer.locator('button[title="Close tab"]');
    await closeBtn.click();
    
    await expect(createUserTab).not.toBeVisible();
    await expect(getUsersTab).toBeVisible();

    const lastCloseBtn = getUsersTabContainer.locator('button[title="Close tab"]');
    await lastCloseBtn.click();

    const mainContent = page.locator('main');
    await expect(mainContent).toContainText('Select a request from the sidebar to get started.');
  });

  test('should delete request and automatically close its tab', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Delete Test');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Delete Test');
    await expect(collectionItem).toBeVisible();

    await collectionItem.click();

    await collectionItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('New Request').click();
    
    dialog = page.getByRole('dialog');
    input = dialog.getByPlaceholder('Request name');
    await input.fill('Req to Delete');
    await input.press('Enter');

    const tabBar = page.locator('.flex.items-center.h-9.border-b');
    const reqTab = tabBar.getByText('Req to Delete');
    await expect(reqTab).toBeVisible();

    const reqItem = sidebar.getByText('Req to Delete');
    await reqItem.click({ button: 'right' });
    await page.getByRole('menu').getByText('Delete').click();

    const confirmDialog = page.getByRole('alertdialog');
    await confirmDialog.getByRole('button', { name: 'Delete' }).click();

    await expect(reqItem).not.toBeVisible();

    await expect(reqTab).not.toBeVisible();
    
    const mainContent = page.locator('main');
    await expect(mainContent).toContainText('Select a request from the sidebar to get started.');
  });

  test('should support tab context menu actions: Close Others and Close All', async ({ page }) => {
    await page.goto('/');

    await page.getByTitle('New Collection').click();
    let dialog = page.getByRole('dialog');
    let input = dialog.getByPlaceholder('Collection name');
    await input.fill('Menu Test Collection');
    await input.press('Enter');

    const sidebar = page.locator('aside');
    const collectionItem = sidebar.getByText('Menu Test Collection');
    await expect(collectionItem).toBeVisible();
    await collectionItem.click();

    const createReq = async (name: string) => {
      await collectionItem.click({ button: 'right' });
      await page.getByRole('menu').getByText('New Request').click();
      dialog = page.getByRole('dialog');
      input = dialog.getByPlaceholder('Request name');
      await input.fill(name);
      await input.press('Enter');
    };

    await createReq('Req 1');
    await createReq('Req 2');
    await createReq('Req 3');

    const tabBar = page.locator('.flex.items-center.h-9.border-b');
    const tab1 = tabBar.getByText('Req 1');
    const tab2 = tabBar.getByText('Req 2');
    const tab3 = tabBar.getByText('Req 3');

    await expect(tab1).toBeVisible();
    await expect(tab2).toBeVisible();
    await expect(tab3).toBeVisible();

    await tab2.click({ button: 'right' });
    let contextMenu = page.getByRole('menu');
    await expect(contextMenu).toBeVisible();
    await contextMenu.getByText('Close Others').click();

    await expect(tab1).not.toBeVisible();
    await expect(tab3).not.toBeVisible();
    await expect(tab2).toBeVisible();

    await createReq('Req 4');
    const tab4 = tabBar.getByText('Req 4');
    await expect(tab2).toBeVisible();
    await expect(tab4).toBeVisible();

    await tab4.click({ button: 'right' });
    contextMenu = page.getByRole('menu');
    await expect(contextMenu).toBeVisible();
    await contextMenu.getByText('Close All').click();

    await expect(tab2).not.toBeVisible();
    await expect(tab4).not.toBeVisible();
    
    const mainContent = page.locator('main');
    await expect(mainContent).toContainText('Select a request from the sidebar to get started.');
  });
});
