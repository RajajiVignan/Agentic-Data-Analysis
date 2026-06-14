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
};

export type Dataset = {
  id: string;
  filename: string;
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

// --- Error helper ---

async function parseError(res: Response): Promise<string> {
  try {
    const data = await res.json();
    return data.error ?? `Request failed (${res.status})`;
  } catch {
    return `Request failed (${res.status})`;
  }
}

// --- API functions ---

export async function checkBackend(): Promise<BackendStatus> {
  try {
    const res = await fetch(`${API_BASE}/health`);
    return res.ok ? "online" : "offline";
  } catch {
    return "offline";
  }
}

export async function fetchDatasets(): Promise<Dataset[]> {
  const res = await fetch(`${API_BASE}/datasets`);
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return (data.datasets as Dataset[]).map((d) => ({ id: d.id, filename: d.filename }));
}

export async function uploadFile(file: File): Promise<{ datasetId: string; filename: string }> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${API_BASE}/upload`, { method: "POST", body: formData });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Upload failed");
  return { datasetId: data.datasetId, filename: data.filename };
}

export async function runAnalysis(datasetIds: string[], prompt: string): Promise<AnalysisResult> {
  const res = await fetch(`${API_BASE}/analyze`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ datasetIds, prompt }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Analysis failed");
  // The backend returns the result at the top level; normalize to AnalysisResult
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
  };
}

export async function connectSource(source: string = "sample"): Promise<{ datasetId: string; filename: string }> {
  const res = await fetch(`${API_BASE}/connect-source`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ source }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Failed to connect source");
  return { datasetId: data.datasetId, filename: data.filename };
}

export async function exportCleanedCsv(datasetIds: string[], apiBase: string = API_BASE): Promise<Blob> {
  const query = encodeURIComponent(datasetIds.join(","));
  const res = await fetch(`${apiBase}/export/cleaned-csv?datasetIds=${query}`);
  if (!res.ok) throw new Error(await parseError(res));
  return res.blob();
}

export async function fetchPinnedCharts(): Promise<PinnedChart[]> {
  const res = await fetch(`${API_BASE}/pinned-charts`);
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.pinnedCharts as PinnedChart[];
}

export async function pinChart(chart: Omit<PinnedChart, "id">): Promise<PinnedChart> {
  const res = await fetch(`${API_BASE}/pin-chart`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id: "", chart_type: chart.chart_type, label: chart.label, data: chart.data, url: chart.url }),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function unpinChart(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/unpin-chart?id=${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}

// --- Connection API ---

export async function fetchConnections(): Promise<ConnectionConfig[]> {
  const res = await fetch(`${API_BASE}/connections`);
  if (!res.ok) throw new Error(await parseError(res));
  const data = await res.json();
  return data.connections as ConnectionConfig[];
}

export async function testConnection(cfg: Record<string, unknown>): Promise<{ ok: boolean; error?: string }> {
  const res = await fetch(`${API_BASE}/connections/test`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cfg),
  });
  if (!res.ok) throw new Error(await parseError(res));
  return res.json();
}

export async function createConnection(cfg: Record<string, unknown>): Promise<{ connection: ConnectionConfig; datasetId: string; filename: string }> {
  const res = await fetch(`${API_BASE}/connections`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cfg),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? "Failed to create connection");
  return data;
}

export async function deleteConnection(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/connections?id=${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await parseError(res));
}
