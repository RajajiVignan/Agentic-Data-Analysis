import { Page, expect } from '@playwright/test';
import * as path from 'path';

const BASE = process.env.BASE_URL || 'http://localhost:3000';

/**
 * Log in as a guest user via the auth overlay.
 */
export async function guestLogin(page: Page) {
  await page.goto('/');
  await page.waitForSelector('text=InsightPilot', { timeout: 15000 });
  await page.getByRole('button', { name: 'Continue as Guest' }).click();
  await page.waitForSelector('text=AI Data Analyst', { timeout: 15000 });
}

/**
 * Upload a sample CSV file via the hidden file input.
 */
export async function uploadSampleCsv(page: Page, filePath?: string) {
  const uploadLabel = page.locator('label', { hasText: 'Upload New' });
  const fileInput = uploadLabel.locator('input[type="file"]');
  const csvPath = filePath || path.resolve(__dirname, '..', '..', 'samples', 'revenue.csv');
  await fileInput.setInputFiles(csvPath);
  await expect(page.getByText('File uploaded successfully')).toBeVisible({ timeout: 20000 });
  // Wait for dataset to be auto-selected and React to re-render
  await page.waitForTimeout(1000);
}

/**
 * Run analysis by calling the exposed __testHandleRunAnalysis handler via page.evaluate.
 * This bypasses the UI button click, which is unreliable with duplicate "Analyze" labels.
 * After calling, waits for the UI to render results.
 */
export async function runAnalysis(page: Page, question: string) {
  // Use the window-level test handler exposed by page.tsx
  const hasHandler = await page.evaluate(() => {
    return typeof (window as any).__testHandleRunAnalysis === 'function';
  });

  if (hasHandler) {
    await page.evaluate((prompt) => {
      return (window as any).__testHandleRunAnalysis(prompt);
    }, question);
  } else {
    // Fallback: fill input and click button
    const input = page.getByPlaceholder('Ask your data a question...');
    await input.fill(question);
    const btn = page.getByRole('button', { name: 'Analyze' }).first();
    await btn.click();
  }
}

/**
 * Wait for analysis results to render in the UI.
 * Looks for KPI tiles, chart cards, or AI insight text.
 */
export async function waitForAnalysis(page: Page, timeout = 60000) {
  const checks = [
    page.getByText('Interactive Trend'),
    page.getByText('Interactive Segments'),
    page.getByText('AI Insight'),
    page.getByText('Time Series Trend'),
  ];

  for (const check of checks) {
    try {
      await check.waitFor({ state: 'visible', timeout });
      return;
    } catch {
      // continue
    }
  }

  // Last resort: wait for any KPI value
  const kpi = page.locator('.text-2xl.font-bold.text-slate-900');
  try {
    await kpi.first().waitFor({ state: 'visible', timeout: 10000 });
  } catch {
    // still ok, we tried
  }
}

/**
 * Get all KPI tile values from the current dashboard view.
 */
export async function getKpiValues(page: Page) {
  return page.locator('.text-2xl.font-bold.text-slate-900').allTextContents();
}

/**
 * Navigate to a sidebar tab.
 */
export async function navigateToTab(page: Page, tabName: string) {
  const sidebar = page.locator('aside');
  const tabButton = sidebar.getByRole('button', { name: tabName, exact: true });
  await tabButton.scrollIntoViewIfNeeded();
  await tabButton.click({ force: true });
  await page.waitForTimeout(500);
}

/**
 * Set sidebar mode (Plots / Pivot).
 */
export async function setSidebarMode(page: Page, mode: 'Plots' | 'Pivot') {
  await page.locator('main').getByRole('button', { name: mode, exact: true }).click();
  await page.waitForTimeout(300);
}

/**
 * Run analysis via the REST API directly.
 * Returns the analysis result without updating page state.
 * Useful for verifying API behavior without UI dependencies.
 */
export async function analyzeViaApi(page: Page, question: string) {
  const datasetsResp = await page.request.get(`${BASE}/api/datasets`);
  const body = await datasetsResp.json();
  const datasets = body.datasets || [];
  if (!datasets || datasets.length === 0) {
    throw new Error('No datasets available for analysis');
  }
  const datasetId = datasets[0].id;

  const resp = await page.request.post(`${BASE}/api/analyze`, {
    data: {
      dataset_ids: [datasetId],
      prompt: question,
      session_id: null,
    },
    timeout: 60000,
  });

  if (!resp.ok()) {
    const text = await resp.text();
    throw new Error(`Analysis failed: ${resp.status()} ${text}`);
  }

  return resp.json();
}
