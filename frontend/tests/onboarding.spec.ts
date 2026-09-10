import { test, expect } from '@playwright/test';

test.describe('Onboarding welcome', () => {
  // Opt out of the global seeded storageState — the welcome only shows up on a
  // profile that has never made the choice.
  test.use({ storageState: { cookies: [], origins: [] } });

  test('first launch offers the local/account choice', async ({ page }) => {
    await page.goto('/');

    const modal = page.getByTestId('onboarding-modal');
    await expect(modal).toBeVisible();
    await expect(modal.getByTestId('onboarding-choice-local')).toBeVisible();
    await expect(modal.getByTestId('onboarding-choice-account')).toBeVisible();
    await expect(modal.getByTestId('onboarding-tour-link')).toBeVisible();
  });

  test('working locally dismisses the welcome for good', async ({ page }) => {
    await page.goto('/');
    await page.getByTestId('onboarding-choice-local').click();
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);

    await page.reload();
    await expect(page.locator('main')).toContainText('Select a request from the sidebar to get started.');
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);
  });

  test('connecting an account opens sync ready to create one', async ({ page }) => {
    await page.goto('/');
    await page.getByTestId('onboarding-choice-account').click();
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId('signin-browser-register')).toHaveClass(/bg-primary/);
  });

  test('the tour walks both ways and hands back to the choice', async ({ page }) => {
    await page.goto('/');
    await page.getByTestId('onboarding-tour-link').click();

    const tour = page.getByTestId('onboarding-tour');
    await expect(tour).toBeVisible();
    await expect(page.getByTestId('onboarding-modal')).toHaveCount(0);
    await expect(tour.getByTestId('onboarding-tour-counter')).toHaveText('1 / 4');
    await expect(tour.getByTestId('onboarding-tour-back')).toBeDisabled();

    await tour.getByTestId('onboarding-tour-next').click();
    await tour.getByTestId('onboarding-tour-next').click();
    await expect(tour.getByTestId('onboarding-tour-counter')).toHaveText('3 / 4');

    await tour.getByTestId('onboarding-tour-back').click();
    await expect(tour.getByTestId('onboarding-tour-counter')).toHaveText('2 / 4');

    await tour.getByTestId('onboarding-tour-next').click();
    await tour.getByTestId('onboarding-tour-next').click();
    await expect(tour.getByTestId('onboarding-tour-counter')).toHaveText('4 / 4');

    await tour.getByTestId('onboarding-tour-next').click();
    await expect(page.getByTestId('onboarding-tour')).toHaveCount(0);
    await expect(page.getByTestId('onboarding-modal')).toBeVisible();
  });

  test('the clip can be paused and resumed', async ({ page }) => {
    await page.goto('/');
    await page.getByTestId('onboarding-tour-link').click();

    const playback = page.getByTestId('onboarding-tour-playback');
    await expect(playback).toBeVisible();
    await expect(playback).toHaveAttribute('aria-label', 'Pause');

    await playback.click();
    await expect(playback).toHaveAttribute('aria-label', 'Play');
    expect(await page.evaluate(
      () => document.querySelector('[data-testid="onboarding-tour"] video')?.paused,
    )).toBe(true);

    await playback.click();
    await expect(playback).toHaveAttribute('aria-label', 'Pause');
  });
});

test.describe('Email confirmation', () => {
  test('an unconfirmed address waits, then connects once GetMe reports it verified', async ({ page }) => {
    await page.goto('/?mock=verify-pending');
    await page.getByRole('button', { name: 'Sync', exact: true }).click();

    const dialog = page.getByRole('dialog');
    // The cloud registers through the browser now; the in-app gate lives on a
    // server that cannot complete a browser sign-in.
    await dialog.getByRole('button', { name: 'Use custom server' }).click();
    await dialog.locator('#server-url').fill('sync.corp.local');
    await dialog.getByRole('tab', { name: 'Register' }).click();
    await dialog.locator('#reg-name').fill('Test User');
    await dialog.locator('#reg-email').fill('pending@example.com');
    await dialog.locator('#reg-password').fill('secret123');
    await dialog.getByRole('button', { name: 'Register', exact: true }).click();

    const waiting = dialog.getByTestId('sync-verify-waiting');
    await expect(waiting).toBeVisible();
    await expect(waiting).toContainText('pending@example.com');
    // The mail has just gone out, so the resend cooldown is already running.
    await expect(waiting.getByTestId('sync-verify-resend')).toBeDisabled();

    // The mock says "not yet" for two GetMe calls, and the poll consumes them
    // alongside the button — drive it until the connected state lands.
    await expect(async () => {
      const confirmed = dialog.getByTestId('sync-verify-confirmed');
      if (await confirmed.isVisible()) await confirmed.click();
      await expect(dialog.getByRole('button', { name: 'Disconnect' })).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 20000 });

    await expect(dialog.getByTestId('sync-verify-waiting')).toHaveCount(0);
    await expect(dialog).toContainText('pending@example.com');
  });
});
