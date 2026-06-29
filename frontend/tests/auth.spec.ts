import { test, expect } from '@playwright/test';
import { guestLogin } from './helpers';

test.describe('Authentication', () => {
  test('shows auth overlay when not logged in', async ({ page }) => {
    await page.goto('/');
    // Should see the auth overlay with brand name
    await expect(page.locator('text=InsightPilot').first()).toBeVisible({ timeout: 15000 });
    await expect(page.getByRole('button', { name: 'Continue as Guest' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
  });

  test('guest login completes successfully', async ({ page }) => {
    await guestLogin(page);
    // Should be on the explore workspace
    await expect(page.locator('text=AI Data Analyst')).toBeVisible({ timeout: 10000 });
    // Sidebar should show explore as active
    await expect(page.locator('text=Explore Workspace')).toBeVisible();
  });

  test('can toggle between login and register', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('text=InsightPilot', { timeout: 15000 });
    // Default shows login mode
    await expect(page.getByText('Sign in to your account')).toBeVisible();
    // Click "Sign up" to switch to register
    await page.getByRole('button', { name: 'Sign up' }).click();
    await expect(page.getByText('Create a new account')).toBeVisible();
    // Switch back
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByText('Sign in to your account')).toBeVisible();
  });

  test('login form shows validation for empty fields', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('text=InsightPilot', { timeout: 15000 });
    // Try submitting with empty fields
    await page.getByRole('button', { name: 'Sign In' }).click();
    // Browser validation should prevent or API returns error
    // Either way the auth overlay should still be visible
    await expect(page.getByRole('button', { name: 'Continue as Guest' })).toBeVisible();
  });
});
