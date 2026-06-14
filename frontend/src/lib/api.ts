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

export type AnalysisResult = {
  notebook: NotebookStep[];
  dashboard: {
    kpis: Kpi[];
    trend: ChartPoint[];
    segments: ChartPoint[];
    recommendations: string[];
    plotUrl?: string | null;
  };
  assumptions: string[];
  warnings: string[];
  used_deterministic?: boolean;
  sqlQueries?: string[];
};

export type Dataset = {
  id: string;
  filename: string;
  liveDb?: boolean;
  tableName?: string;
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

let authToken: string | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
  if (token) {
    localStorage.setItem("auth_token", token);
  } else {
    localStorage.removeItem("auth_token");
  }
}

export function getAuthToken(): string | null {
  if (authToken) return authToken;
  const stored = localStorage.getItem("auth_token");
  if (stored) {
    authToken = stored;
    return stored;
  }
  return null;
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const headers = {
    "Content-Type": "application/json",
    ...authHeaders(),
    ...(options.headers || {}),
  };
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  return res;
}

export async function register(email: string, password: string, name: string): Promise<{ user: AuthUser; token: string }> {
  const res = await apiFetch("/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password, name }),
  });
  if (!res.ok) throw new Error((await res.json()).error ?? "Registration failed");
  const data = await res.json();
  setAuthToken(data.token);
  return data;
}

export async function login(email: string, password: string): Promise<{ user: AuthUser; token: string }> {
  const res = await apiFetch("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error((await res.json()).error ?? "Login failed");
  const data = await res.json();
  setAuthToken(data.token);
  return data;
}

export async function logout(): Promise<void> {
  try {
    await apiFetch("/auth/logout", { method: "POST" });
  } catch {
    // ignore
  }
  setAuthToken(null);
}

export async function fetchMe(): Promise<AuthUser | null> {
  const token = getAuthToken();
  if (!token) return null;
  const res = await apiFetch("/auth/me");
  if (!res.ok) {
    setAuthToken(null);
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
  return (data.datasets as Dataset[]).map((d) => ({ id: d.id, filename: d.filename }));
}

export async function uploadFile(file: File): Promise<{ datasetId: string; filename: string }> {
  const formData = new FormData();
  formData.append("file", file);
  const token = getAuthToken();
  const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {};
  const res = await fetch(`${API_BASE}/upload`, { method: "POST", body: formData, headers });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Upload failed");
  return { datasetId: data.datasetId, filename: data.filename };
}

export async function runAnalysis(datasetIds: string[], prompt: string): Promise<AnalysisResult> {
  const res = await apiFetch("/analyze", {
    method: "POST",
    body: JSON.stringify({ datasetIds, prompt }),
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
    plotUrl: data.plotUrl ?? null,
  },
  assumptions: data.assumptions ?? [],
  warnings: data.warnings ?? [],
  used_deterministic: data.used_deterministic,
  sqlQueries: data.sqlQueries ?? [],
  };
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


