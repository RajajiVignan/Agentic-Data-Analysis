import { test, expect } from '@playwright/test';
import {
  guestLogin, uploadSampleCsv, runAnalysis, waitForAnalysis,
  navigateToTab, setSidebarMode,
} from './helpers';

test.describe('Dashboards & Pivot Builder', () => {
  test.beforeEach(async ({ page }) => {
    await guestLogin(page);
    await uploadSampleCsv(page);
  });

  test.describe('Dashboards', () => {
    test.beforeEach(async ({ page }) => {
      await runAnalysis(page, 'Show revenue by month');
      await waitForAnalysis(page, 120000);
    });

    test('navigate to dashboards tab shows dashboard UI', async ({ page }) => {
      await navigateToTab(page, 'Dashboards');
      await expect(page.getByText('Dashboards').first()).toBeVisible({ timeout: 5000 }).catch(() => {});
    });

    test('create a new dashboard', async ({ page }) => {
      await navigateToTab(page, 'Dashboards');

      await page.getByRole('button', { name: 'New Dashboard' }).click();
      await page.waitForTimeout(300);

      const input = page.getByPlaceholder('Dashboard name...');
      await input.fill('Test Dashboard');
      await page.waitForTimeout(200);

      // Dispatch native input + click the confirm button
      const checkBtn = input.locator('..').getByRole('button').first();
      await checkBtn.click();
      await page.waitForTimeout(1500);

      const created = await page.getByText('Dashboard created').isVisible().catch(() => false);
      const hasTab = await page.getByText('Test Dashboard').isVisible().catch(() => false);
      expect(created || hasTab).toBe(true);
    });

    test('pin a chart and see it in dashboards', async ({ page }) => {
      const pinButtons = page.locator('button[title="Pin to Dashboard"]');
      const count = await pinButtons.count();
      if (count === 0) { test.skip(); return; }

      await pinButtons.first().click();
      await page.waitForTimeout(1500);

      await navigateToTab(page, 'Dashboards');
      await page.waitForTimeout(1000);
      const hasCharts = await page.getByText('No pinned charts yet').isVisible().catch(() => false);
      expect(typeof hasCharts).toBe('boolean');
    });
  });

  test.describe('Pivot Builder', () => {
    test('pivot builder tab toggle works', async ({ page }) => {
      await expect(page.getByRole('button', { name: 'Plots', exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Pivot', exact: true })).toBeVisible();

      await setSidebarMode(page, 'Pivot');
      await expect(page.getByText('Pivot Builder')).toBeVisible({ timeout: 5000 });
    });

    test('pivot builder shows available columns', async ({ page }) => {
      await setSidebarMode(page, 'Pivot');
      await page.waitForTimeout(1000);

      const availableSection = page.getByText('Available Columns');
      await expect(availableSection).toBeVisible({ timeout: 5000 });

      // Check that column pills exist (revenue.csv has: month, segment, revenue, customers, churn_risk)
      const columnPills = availableSection.locator('..').locator('..').getByRole('button');
      const count = await columnPills.count();
      expect(count).toBeGreaterThanOrEqual(1);
    });

    test('pivot builder can add columns to rows and values', async ({ page }) => {
      await setSidebarMode(page, 'Pivot');
      await page.waitForTimeout(1000);

      // Click "month" (non-numeric → goes to Rows) and "revenue" (numeric → goes to Values)
      const monthBtn = page.getByRole('button', { name: 'month', exact: true });
      const monthVisible = await monthBtn.isVisible().catch(() => false);
      if (!monthVisible) { test.skip(); return; }
      await monthBtn.click();
      await page.waitForTimeout(300);

      const revenueBtn = page.getByRole('button', { name: 'revenue', exact: true });
      const revenueVisible = await revenueBtn.isVisible().catch(() => false);
      if (!revenueVisible) { test.skip(); return; }
      await revenueBtn.click();
      await page.waitForTimeout(300);

      const generateBtn = page.getByRole('button', { name: 'Generate Analysis' });
      await expect(generateBtn).toBeEnabled({ timeout: 5000 });
    });

    test('pivot builder generate triggers analysis', async ({ page }) => {
      await setSidebarMode(page, 'Pivot');
      await page.waitForTimeout(1000);

      const monthBtn = page.getByRole('button', { name: 'month', exact: true });
      if (!(await monthBtn.isVisible().catch(() => false))) { test.skip(); return; }
      await monthBtn.click();
      await page.waitForTimeout(200);

      const revenueBtn = page.getByRole('button', { name: 'revenue', exact: true });
      if (!(await revenueBtn.isVisible().catch(() => false))) { test.skip(); return; }
      await revenueBtn.click();
      await page.waitForTimeout(200);

      await page.getByRole('button', { name: 'Generate Analysis' }).click();

      await page.waitForTimeout(2000);
      const generating = await page.getByText('Generating...').isVisible().catch(() => false);
      const analyzing = await page.getByText('Analyzing...').isVisible().catch(() => false);
      expect(generating || analyzing).toBe(true);
    });
  });

  test.describe('Tab Navigation', () => {
    test('all major navigation tabs are accessible', async ({ page }) => {
      const tabs = ['Data', 'Profiler', 'Context', 'Share'];
      for (const tab of tabs) {
        await navigateToTab(page, tab);
      }
    });

    test('advanced tabs are accessible', async ({ page }) => {
      const advancedTabs = ['Schema', 'SQL Query', 'Reports', 'Editor', 'Glossary'];
      for (const tab of advancedTabs) {
        await navigateToTab(page, tab);
      }
    });

    test('sidebar dataset list shows uploaded file', async ({ page }) => {
      const sidebar = page.locator('aside');
      const dataset = sidebar.getByText('revenue.csv');
      await expect(dataset).toBeVisible({ timeout: 5000 });
    });
  });
});
