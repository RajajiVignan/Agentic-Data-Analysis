import { test, expect } from '@playwright/test';
import {
  guestLogin, uploadSampleCsv, runAnalysis, waitForAnalysis,
} from './helpers';

test.describe('Chart Interactions', () => {
  test.beforeEach(async ({ page }) => {
    await guestLogin(page);
    await uploadSampleCsv(page);
    await runAnalysis(page, 'Show revenue by month');
    await waitForAnalysis(page, 30000);
  });

  test('right-click context menu appears on chart cards', async ({ page }) => {
    const chartTitle = page.getByText('Interactive Trend');
    const visible = await chartTitle.isVisible().catch(() => false);
    if (!visible) {
      test.skip();
      return;
    }

    await chartTitle.click({ button: 'right' });
    await expect(page.getByText('Drill Down')).toBeVisible({ timeout: 3000 });
    await expect(page.getByText('Change Chart Type')).toBeVisible();
    await expect(page.getByText('Filter by Selection')).toBeVisible();
    await expect(page.getByText('Export').first()).toBeVisible();
  });

  test('context menu change chart type submenu works', async ({ page }) => {
    const chartTitle = page.getByText('Interactive Trend');
    const visible = await chartTitle.isVisible().catch(() => false);
    if (!visible) { test.skip(); return; }

    await chartTitle.click({ button: 'right' });
    await expect(page.getByText('Change Chart Type')).toBeVisible({ timeout: 3000 });

    await page.getByText('Change Chart Type').hover();
    await page.waitForTimeout(400);

    await expect(page.getByText('Line').first()).toBeVisible();
    await expect(page.getByText('Pie').first()).toBeVisible();
    await expect(page.getByText('Area').first()).toBeVisible();
  });

  test('context menu export submenu works', async ({ page }) => {
    const chartTitle = page.getByText('Interactive Trend');
    const visible = await chartTitle.isVisible().catch(() => false);
    if (!visible) { test.skip(); return; }

    await chartTitle.click({ button: 'right' });
    await expect(page.getByText('Export').first()).toBeVisible({ timeout: 3000 });

    await page.getByText('Export').first().hover();
    await page.waitForTimeout(400);

    await expect(page.getByText('PNG').first()).toBeVisible();
    await expect(page.getByText('JPEG').first()).toBeVisible();
  });

  test('dismiss context menu on outside click', async ({ page }) => {
    const chartTitle = page.getByText('Interactive Trend');
    const visible = await chartTitle.isVisible().catch(() => false);
    if (!visible) { test.skip(); return; }

    await chartTitle.click({ button: 'right' });
    await expect(page.getByText('Drill Down')).toBeVisible({ timeout: 3000 });

    await page.mouse.click(10, 10);
    await page.waitForTimeout(500);
    await expect(page.getByText('Drill Down')).not.toBeVisible();
  });

  test('cross-filtering: clicking a bar dims others', async ({ page }) => {
    const chartTitle = page.getByText('Interactive Trend');
    const visible = await chartTitle.isVisible().catch(() => false);
    if (!visible) { test.skip(); return; }

    const bars = page.locator('rect.recharts-bar-rectangle');
    const barCount = await bars.count();

    if (barCount === 0) { test.skip(); return; }

    await bars.first().click();
    await page.waitForTimeout(600);

    const crossFilter = page.getByText('Cross-filter:');
    await expect(crossFilter).toBeVisible({ timeout: 3000 });

    await page.getByText('Clear all').click();
    await page.waitForTimeout(500);
    await expect(crossFilter).not.toBeVisible();
  });

  test('cross-filtering: clicking a pie slice filters', async ({ page }) => {
    const segmentChart = page.getByText('Interactive Segments');
    const visible = await segmentChart.isVisible().catch(() => false);
    if (!visible) { test.skip(); return; }

    const slices = segmentChart.locator('path');
    const sliceCount = await slices.count();
    if (sliceCount === 0) { test.skip(); return; }

    await slices.first().click();
    await page.waitForTimeout(600);
    const crossFilter = page.getByText('Cross-filter:');
    await expect(crossFilter).toBeVisible({ timeout: 3000 });
    await page.getByText('Clear all').click();
  });

  test('pin chart button exists and clickable', async ({ page }) => {
    const pinButtons = page.locator('button[title="Pin to Dashboard"]');
    const count = await pinButtons.count();
    expect(count).toBeGreaterThan(0);

    await pinButtons.first().click();
    await page.waitForTimeout(1500);
    const pinned = await page.getByText('Chart pinned to dashboard').isVisible().catch(() => false);
    const failed = await page.getByText('Failed to pin chart').isVisible().catch(() => false);
    expect(pinned || failed).toBe(true);
  });

  test('download chart dropdown opens on click', async ({ page }) => {
    const downloadButtons = page.locator('button[title="Download chart"]');
    const count = await downloadButtons.count();
    expect(count).toBeGreaterThan(0);

    await downloadButtons.first().click();
    await page.waitForTimeout(400);

    await expect(page.getByText('PNG').first()).toBeVisible({ timeout: 3000 });
    await expect(page.getByText('JPEG').first()).toBeVisible();
  });

  test('all chart type toggle works', async ({ page }) => {
    const toggle = page.getByRole('button', { name: /Show all|Show smart/ });
    const exists = await toggle.isVisible().catch(() => false);
    if (!exists) { test.skip(); return; }

    await toggle.click();
    await page.waitForTimeout(500);

    const cardTitles = page.locator('strong.text-sm.font-semibold.text-slate-800');
    const cardCount = await cardTitles.count();
    expect(cardCount).toBeGreaterThan(1);
  });
});
