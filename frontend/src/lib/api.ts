// All API calls use relative paths since Go serves both the SPA and the API
// on the same host:port. No NEXT_PUBLIC_API_BASE needed.
const API_BASE = "/api";

// --- Types ---

export type NotebookStep = {
  title: string;
  body?: string;
  code?: string;
};

export type Kpi = {
  label: string;
  value: string;
  change: string;
};

export type ChartPoint = {
  label: string;
  value: number;
};

export type ConversationTurn = {
  prompt: string;
  response: AnalysisResult;
};

export type Explanation = {
  chart: string;
  columns: string;
  sql?: string;
  warning?: string;
  grouping?: string;
};

export type AnalysisResult = {
  notebook: NotebookStep[];
  dashboard: {
    kpis: Kpi[];
    trend: ChartPoint[];
    segments: ChartPoint[];
    recommendations: string[];
    narrative?: string;
    plotUrl?: string | null;
    chartType?: string;
    chartTypes?: string[];
    explanations?: Explanation[];
  };
  assumptions: string[];
  warnings: string[];
  used_deterministic?: boolean;
  sqlQueries?: string[];
  sessionId?: string;
};

export type Dataset = {
  id: string;
  filename: string;
  liveDb?: boolean;
  tableName?: string;
  profile?: {
    rowCount: number;
    columns: Column[];
  };
};

export type PinnedChart = {
  id: string;
  chart_type: 'kpi' | 'trend' | 'segment' | 'python_plot';
  label: string;
  data: unknown;
  url?: string;
};

export type ConnectionConfig = {
  id: string;
  provider: string;
  host?: string;
  port?: string;
  database: string;
  username?: string;
  password?: string;
  projectId?: string;
  accountId?: string;
  warehouse?: string;
  role?: string;
  region?: string;
  useSsl?: boolean;
  connected: boolean;
  datasetId?: string;
  filename?: string;
  connectedAt?: string;
};

export type BackendStatus = "checking" | "online" | "offline";

export type Dashboard = {
  id: string;
  name: string;
  chartIds: string[];
  created_at: string;
};

// --- Error helper ---

async function parseError(res: Response): Promise<string> {
  try {
    const data = await res.json();
    return data.error ?? `Request failed (${res.status})`;
  } catch {
    return `Request failed (${res.status})`;
  }
}

// --- Auth API ---

export type AuthUser = {
  id: string;
  email: string;
  name: string;
  created_at: string;
};

async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> || {}),
  };
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });
  return res;
}

export async function register(email: string, password: string, name: string): Promise<{ user: AuthUser; token: string }> {
  const res = await apiFetch("/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password, name }),
  });
  if (!res.ok) throw new Error((await res.json()).error ?? "Registration failed");
  return res.json();
}

export async function login(email: string, password: string): Promise<{ user: AuthUser; token: string }> {
  const res = await apiFetch("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error((await res.json()).error ?? "Login failed");
  return res.json();
}

export async function guestLogin(): Promise<{ user: AuthUser; token: string }> {
  const res = await apiFetch("/auth/guest", { method: "POST" });
  if (!res.ok) throw new Error((await res.json()).error ?? "Guest login failed");
  return res.json();
}

export async function logout(): Promise<void> {
  try {
    await apiFetch("/auth/logout", { method: "POST" });
  } catch {
    // ignore
  }
}

export async function fetchMe(): Promise<AuthUser | null> {
  const res = await apiFetch("/auth/me");
  if (!res.ok) {
    return null;
  }
  return res.json();
}

// Use auth-aware fetch for protected endpoints
export async function checkBackend(): Promise<BackendStatus> {
  try {
    const res = await apiFetch("/health");
    return res.ok ? "online" : "offline";
  } catch {
    return "offline";
  }
}

export async function fetchDatasets(): Promise<Dataset[]> {
  const res = await apiFetch("/datasets");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return (data.datasets as Dataset[]).map((d) => ({ id: d.id, filename: d.filename, liveDb: d.liveDb, profile: d.profile }));
}

export async function uploadFile(file: File): Promise<{ datasetId: string; filename: string }> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${API_BASE}/upload`, { method: "POST", body: formData, credentials: "include" });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Upload failed");
  return { datasetId: data.datasetId, filename: data.filename };
}

export async function runAnalysis(datasetIds: string[], prompt: string, sessionId?: string): Promise<AnalysisResult> {
  const res = await apiFetch("/analyze", {
    method: "POST",
    body: JSON.stringify({ datasetIds, prompt, sessionId }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Analysis failed");
  return {
    notebook: data.notebook ?? [],
    dashboard: {
      kpis: data.dashboard?.kpis ?? [],
      trend: data.dashboard?.trend ?? [],
      segments: data.dashboard?.segments ?? [],
      recommendations: data.dashboard?.recommendations ?? [],
      narrative: data.dashboard?.narrative ?? "",
      plotUrl: data.plotUrl ?? null,
      chartType: data.dashboard?.chartType ?? "",
      chartTypes: data.dashboard?.chartTypes ?? [],
      explanations: data.dashboard?.explanations ?? [],
    },
    assumptions: data.assumptions ?? [],
    warnings: data.warnings ?? [],
    used_deterministic: data.used_deterministic,
    sqlQueries: data.sqlQueries ?? [],
    sessionId: data.sessionId,
  };
}

export async function clearSession(sessionId: string): Promise<void> {
  const res = await apiFetch("/session/clear", {
    method: "POST",
    body: JSON.stringify({ sessionId }),
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function connectSource(source: string = "sample"): Promise<{ datasetId: string; filename: string }> {
  const res = await apiFetch("/connect-source", {
    method: "POST",
    body: JSON.stringify({ source }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Failed to connect source");
  return { datasetId: data.datasetId, filename: data.filename };
}

export async function exportCleanedCsv(datasetIds: string[], apiBase: string = API_BASE): Promise<Blob> {
  const query = encodeURIComponent(datasetIds.join(","));
  const res = await apiFetch(`/export/cleaned-csv?datasetIds=${query}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.blob();
}

export async function fetchPinnedCharts(): Promise<PinnedChart[]> {
  const res = await apiFetch("/pinned-charts");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.pinnedCharts as PinnedChart[];
}

export async function pinChart(chart: Omit<PinnedChart, "id">): Promise<PinnedChart> {
  const res = await apiFetch("/pin-chart", {
    method: "POST",
    body: JSON.stringify({ id: "", chart_type: chart.chart_type, label: chart.label, data: chart.data, url: chart.url }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function unpinChart(id: string): Promise<void> {
  const res = await apiFetch(`/unpin-chart?id=${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function fetchConnections(): Promise<ConnectionConfig[]> {
  const res = await apiFetch("/connections");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.connections as ConnectionConfig[];
}

export async function testConnection(cfg: Record<string, unknown>): Promise<{ ok: boolean; error?: string }> {
  const res = await apiFetch("/connections/test", {
    method: "POST",
    body: JSON.stringify(cfg),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function createConnection(cfg: Record<string, unknown>): Promise<{ connection: ConnectionConfig; datasetId: string; filename: string }> {
  const res = await apiFetch("/connections", {
    method: "POST",
    body: JSON.stringify(cfg),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Failed to create connection");
  return data;
}

export async function deleteConnection(id: string): Promise<void> {
  const res = await apiFetch(`/connections?id=${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function fetchDashboards(): Promise<Dashboard[]> {
  const res = await apiFetch("/dashboards");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.dashboards as Dashboard[];
}

export async function createDashboard(name: string): Promise<Dashboard> {
  const res = await apiFetch("/dashboards", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function renameDashboard(id: string, name: string): Promise<void> {
  const res = await apiFetch(`/dashboards/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function deleteDashboard(id: string): Promise<void> {
  const res = await apiFetch(`/dashboards/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function addChartToDashboard(dashboardId: string, chartId: string): Promise<void> {
  const res = await apiFetch(`/dashboards/${encodeURIComponent(dashboardId)}/charts`, {
    method: "POST",
    body: JSON.stringify({ chartId }),
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function refreshDataset(id: string): Promise<{ ok: boolean; rowCount: number }> {
  const res = await apiFetch(`/refresh-dataset?id=${encodeURIComponent(id)}`, {
    method: "POST",
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

// --- Feature 4: Cross-Dataset Joins ---

export type JoinRequest = {
  leftDatasetId: string;
  rightDatasetId: string;
  leftKey: string;
  rightKey: string;
  joinType: 'inner' | 'left' | 'right' | 'outer';
};

export async function joinDatasets(req: JoinRequest): Promise<{ datasetId: string; filename: string; profile: unknown; rowCount: number }> {
  const res = await apiFetch("/join", {
    method: "POST",
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

// --- Feature 5: Custom SQL Query Mode ---

export type QueryResult = {
  columns: string[];
  rows: Record<string, string>[];
  page: number;
  pageSize: number;
  hasMore: boolean;
  totalRows: number;
};

export type SchemaInfo = {
  datasetId: string;
  filename: string;
  rowCount: number;
  columns: { name: string; type: string; nonEmpty: number }[];
  tableAlias: string;
};

export async function runSQLQuery(datasetIds: string[], sql: string, page = 1, pageSize = 100): Promise<QueryResult> {
  const res = await apiFetch("/query", {
    method: "POST",
    body: JSON.stringify({ datasetIds, sql, page, pageSize }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function fetchQuerySchema(datasetIds: string[]): Promise<{ schemas: SchemaInfo[] }> {
  const res = await apiFetch(`/query/schema?datasetIds=${encodeURIComponent(datasetIds.join(","))}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function visualizeQuery(datasetId: string, sql: string, chartType?: string): Promise<{ columns: string[]; rows: Record<string, string>[]; chartType: string; rowCount: number }> {
  const res = await apiFetch("/query/visualize", {
    method: "POST",
    body: JSON.stringify({ datasetId, sql, chartType }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

// --- Feature 6: Scheduled Reports & Alerts ---

export type ScheduledReport = {
  id: string;
  name: string;
  datasetIds: string[];
  chartIds: string[];
  frequency: 'daily' | 'weekly' | 'monthly';
  dayOfWeek: number;
  dayOfMonth: number;
  hour: number;
  emails: string[];
  slackWebhook?: string;
  teamsWebhook?: string;
  lastSent?: string;
  nextRun?: string;
  enabled: boolean;
  createdAt: string;
};

export type AlertRule = {
  id: string;
  name: string;
  datasetId: string;
  metricCol: string;
  condition: 'drop' | 'rise' | 'custom';
  threshold: number;
  period: string;
  emails: string[];
  slackHook?: string;
  enabled: boolean;
  lastChecked?: string;
  createdAt: string;
};

export async function fetchReports(): Promise<ScheduledReport[]> {
  const res = await apiFetch("/reports");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.reports as ScheduledReport[];
}

export async function createReport(rpt: Partial<ScheduledReport>): Promise<ScheduledReport> {
  const res = await apiFetch("/reports", {
    method: "POST",
    body: JSON.stringify(rpt),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function deleteReport(id: string): Promise<void> {
  const res = await apiFetch(`/reports?id=${encodeURIComponent(id)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function fetchAlerts(): Promise<AlertRule[]> {
  const res = await apiFetch("/alerts");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.alerts as AlertRule[];
}

export async function createAlert(alert: Partial<AlertRule>): Promise<AlertRule> {
  const res = await apiFetch("/alerts", {
    method: "POST",
    body: JSON.stringify(alert),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function deleteAlert(id: string): Promise<void> {
  const res = await apiFetch(`/alerts?id=${encodeURIComponent(id)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await parseError(res));
}

// --- Feature 7: Drag-and-Drop Dashboard Editor ---

export type DashboardTile = {
  id: string;
  type: 'chart' | 'text' | 'divider' | 'image' | 'metric';
  chartType?: string;
  title?: string;
  content?: string;
  imageUrl?: string;
  pinnedId?: string;
  w: number;
  h: number;
  x: number;
  y: number;
  data?: Record<string, unknown>;
};

export type DashboardLayout = {
  id: string;
  name: string;
  isDefault?: boolean;
  tiles: DashboardTile[];
  createdAt: string;
  updatedAt: string;
};

export async function fetchDashboardLayouts(): Promise<DashboardLayout[]> {
  const res = await apiFetch("/dashboard-layouts");
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.layouts as DashboardLayout[];
}

export async function getDashboardLayout(id: string): Promise<DashboardLayout> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function createDashboardLayout(name: string): Promise<DashboardLayout> {
  const res = await apiFetch("/dashboard-layouts", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function saveDashboardLayout(id: string, layout: DashboardLayout): Promise<void> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(layout),
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function deleteDashboardLayout(id: string): Promise<void> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(id)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function addTileToLayout(layoutId: string, tile: Partial<DashboardTile>): Promise<DashboardLayout> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(layoutId)}/tiles`, {
    method: "POST",
    body: JSON.stringify(tile),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function updateTileInLayout(layoutId: string, tileId: string, tile: Partial<DashboardTile>): Promise<void> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(layoutId)}/tiles/${encodeURIComponent(tileId)}`, {
    method: "PUT",
    body: JSON.stringify(tile),
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function removeTileFromLayout(layoutId: string, tileId: string): Promise<void> {
  const res = await apiFetch(`/dashboard-layouts/${encodeURIComponent(layoutId)}/tiles/${encodeURIComponent(tileId)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}

export async function removeChartFromDashboard(dashboardId: string, chartId: string): Promise<void> {
  const res = await apiFetch(
    `/dashboards/${encodeURIComponent(dashboardId)}/charts/${encodeURIComponent(chartId)}`,
    { method: "DELETE" }
  );
  if (!res.ok) throw new Error(await parseError(res));
}

// --- Share API ---

export type SharedDashboardData = {
  token: string;
  created_at: string;
  expires_at: string;
  charts: PinnedChart[];
  url: string;
};

export async function createShareLink(chartIds: string[]): Promise<SharedDashboardData> {
  const res = await apiFetch("/share", {
    method: "POST",
    body: JSON.stringify({ chartIds }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function getSharedDashboard(token: string): Promise<SharedDashboardData> {
  const res = await apiFetch(`/shared/${encodeURIComponent(token)}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

// --- Transformation Pipeline Types & API ---

export type Column = {
  name: string;
  type: string;
  nonEmpty: number;
  sample?: string[];
};

export type TransformStep = {
  type: string;
  params: Record<string, unknown>;
  description?: string;
};

export type TransformHistory = {
  steps: TransformStep[];
  undone: TransformStep[];
  canUndo: boolean;
  canRedo: boolean;
  rowCount?: number;
  columns?: Column[];
};

export type TransformPreviewResult = {
  rowCount: number;
  columns: Column[];
  rows: Record<string, string>[];
};

export async function transformPreview(datasetId: string, step: TransformStep): Promise<TransformPreviewResult> {
  const res = await apiFetch("/transform/preview", {
    method: "POST",
    body: JSON.stringify({ datasetId, step }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function transformApply(datasetId: string, step: TransformStep): Promise<Record<string, unknown>> {
  const res = await apiFetch("/transform/apply", {
    method: "POST",
    body: JSON.stringify({ datasetId, step }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function transformUndo(datasetId: string): Promise<Record<string, unknown>> {
  const res = await apiFetch("/transform/undo", {
    method: "POST",
    body: JSON.stringify({ datasetId }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function transformRedo(datasetId: string): Promise<Record<string, unknown>> {
  const res = await apiFetch("/transform/redo", {
    method: "POST",
    body: JSON.stringify({ datasetId }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function transformHistory(datasetId: string): Promise<TransformHistory> {
  const res = await apiFetch(`/transform/history?datasetId=${encodeURIComponent(datasetId)}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function transformReset(datasetId: string): Promise<Record<string, unknown>> {
  const res = await apiFetch("/transform/reset", {
    method: "POST",
    body: JSON.stringify({ datasetId }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

// --- Feature 12: Data Profiler ---

export type ColumnProfile = {
  name: string;
  type: string;
  nonEmpty: number;
  nullCount: number;
  uniqueCount: number;
  min?: number;
  max?: number;
  mean?: number;
  median?: number;
  stdDev?: number;
  sample?: string[];
};

export type HistogramBucket = {
  label: string;
  min: number;
  max: number;
  count: number;
};

export type HistogramResult = {
  column: string;
  buckets: HistogramBucket[];
};

export type CorrelationResult = {
  col1: string;
  col2: string;
  r: number;
};

export type DuplicateInfo = {
  totalRows: number;
  duplicateRows: number;
  duplicateKeys?: string[];
};

export type DatasetProfile = {
  datasetId: string;
  filename: string;
  rowCount: number;
  columnCount: number;
  columns: ColumnProfile[];
  correlations?: CorrelationResult[];
  duplicates: DuplicateInfo;
  histograms?: HistogramResult[];
};

export async function fetchDatasetProfile(datasetId: string): Promise<DatasetProfile> {
  const res = await apiFetch(`/dataset/profile?id=${encodeURIComponent(datasetId)}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}


