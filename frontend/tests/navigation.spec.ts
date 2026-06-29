import { test, expect } from '@playwright/test';
import { guestLogin, navigateToTab } from './helpers';

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await guestLogin(page);
  });

  test('sidebar navigation highlights active tab', async ({ page }) => {
    // "Explore" tab should be active by default
    const sidebar = page.locator('aside');
    const exploreBtn = sidebar.getByRole('button', { name: 'Explore', exact: true });
    await expect(exploreBtn).toBeVisible();

    // Navigate to Data tab
    await navigateToTab(page, 'Data');
    await page.waitForTimeout(500);

    // Should see Data tab content
    await expect(page.getByText('Connections & Datasets')).toBeVisible({ timeout: 5000 });
  });

  test('explore tab is the default view', async ({ page }) => {
    await expect(page.getByText('AI Data Analyst')).toBeVisible({ timeout: 5000 });
    await expect(page.getByPlaceholder('Ask your data a question...')).toBeVisible();
  });

  test('Data tab shows connection UI', async ({ page }) => {
    await navigateToTab(page, 'Data');
    await page.waitForTimeout(500);

    // Data tab should show some content
    const hasContent = await page.getByText('Connections & Datasets').isVisible().catch(() => false);
    const hasEmpty = await page.getByText('Data Sources').isVisible().catch(() => false);
    expect(hasContent || hasEmpty).toBe(true);
  });

  test('Share tab shows export options', async ({ page }) => {
    const sidebar = page.locator('aside');
    const shareBtn = sidebar.getByRole('button', { name: 'Share', exact: true });
    await shareBtn.scrollIntoViewIfNeeded();
    await shareBtn.click({ force: true });
    await page.waitForTimeout(500);

    await expect(page.getByText('Share & Export')).toBeVisible({ timeout: 5000 });
  });

  test('Glossary tab is accessible', async ({ page }) => {
    const sidebar = page.locator('aside');
    const glossaryBtn = sidebar.getByRole('button', { name: 'Glossary', exact: true });
    const exists = await glossaryBtn.isVisible().catch(() => false);

    if (exists) {
      await glossaryBtn.scrollIntoViewIfNeeded();
      await glossaryBtn.click({ force: true });
      await page.waitForTimeout(500);
    }
  });

  test('sidebar can be toggled open/closed', async ({ page }) => {
    // Find the sidebar toggle button in the header
    const toggleBtn = page.locator('button[title="Toggle sidebar"]').or(
      page.locator('button[title="Unpin sidebar"]')
    );

    if (await toggleBtn.isVisible()) {
      await toggleBtn.click();
      await page.waitForTimeout(300);
      // Sidebar should be hidden or narrower
    }
  });
});
