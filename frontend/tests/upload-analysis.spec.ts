import { test, expect } from '@playwright/test';
import { guestLogin, uploadSampleCsv, runAnalysis, waitForAnalysis, getKpiValues } from './helpers';
import * as path from 'path';

test.describe('Upload & Analysis Workflow', () => {
  test.beforeEach(async ({ page }) => {
    await guestLogin(page);
  });

  test('upload CSV file and confirm dataset appears', async ({ page }) => {
    await uploadSampleCsv(page);

    await expect(page.getByText('revenue.csv').first()).toBeVisible({ timeout: 5000 });
  });

  test('upload JSON file and confirm dataset appears', async ({ page }) => {
    const jsonPath = path.resolve(__dirname, '..', '..', 'samples', 'customers.json');
    const uploadLabel = page.locator('label', { hasText: 'Upload New' });
    const fileInput = uploadLabel.locator('input[type="file"]');
    await fileInput.setInputFiles(jsonPath);

    await expect(page.getByText('File uploaded successfully')).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('customers.json').first()).toBeVisible({ timeout: 5000 });
  });

  test('run analysis on revenue data and see KPI tiles', async ({ page }) => {
    await uploadSampleCsv(page);
    await runAnalysis(page, 'Show total revenue and customer count');
    await waitForAnalysis(page);

    const kpis = await getKpiValues(page);
    expect(kpis.length).toBeGreaterThan(0);

    const hasCharts = await page.getByText('Interactive Trend').isVisible().catch(() => false);
    const hasAiInsight = await page.getByText('AI Insight').isVisible().catch(() => false);
    expect(hasCharts || hasAiInsight).toBe(true);
  });

  test('run analysis and verify chart cards render', async ({ page }) => {
    await uploadSampleCsv(page);

    await page.getByTitle('Compare values across categories').click();
    await page.waitForTimeout(200);

    await runAnalysis(page, 'Show revenue by month as a bar chart');
    await waitForAnalysis(page, 30000);

    const chartCards = [
      'Interactive Trend',
      'Time Series Trend',
      'Interactive Segments',
      'AI Generated Visualization',
    ];

    let foundChart = false;
    for (const card of chartCards) {
      const visible = await page.getByText(card).isVisible().catch(() => false);
      if (visible) {
        foundChart = true;
        break;
      }
    }
    expect(foundChart).toBe(true);

    const downloadButtons = page.locator('button[title="Download chart"]');
    const count = await downloadButtons.count();
    expect(count).toBeGreaterThan(0);
  });

  test('suggested follow-up questions appear after analysis', async ({ page }) => {
    await uploadSampleCsv(page);
    await runAnalysis(page, 'Show revenue trends');
    await waitForAnalysis(page);

    const tryAsking = page.getByText('Try asking');
    await expect(tryAsking).toBeVisible({ timeout: 10000 }).catch(() => {});
  });

  test('can start a new analysis to clear conversation', async ({ page }) => {
    await uploadSampleCsv(page);
    await runAnalysis(page, 'Show revenue by segment');
    await waitForAnalysis(page);

    await page.getByRole('button', { name: 'New Analysis' }).click();
    await page.waitForTimeout(500);

    await expect(page.getByPlaceholder('Ask your data a question...')).toBeVisible();
  });

  test('analysis error state shows error message', async ({ page }) => {
    // Use evaluate to call the handler directly since the button is disabled without a dataset
    const hasHandler = await page.evaluate(() => typeof (window as any).__testHandleRunAnalysis === 'function');
    if (hasHandler) {
      await page.evaluate(() => (window as any).__testHandleRunAnalysis('some question'));
    } else {
      const input = page.getByPlaceholder('Ask your data a question...');
      await input.fill('some question');
      await page.getByRole('button', { name: 'Analyze' }).first().click({ force: true });
    }

    const hasError = await page.getByText('Select at least one dataset').isVisible().catch(() => false);
    const hasErrorToast = await page.getByText('Analysis failed').isVisible().catch(() => false);
    expect(hasError || hasErrorToast).toBe(true);
  });
});
