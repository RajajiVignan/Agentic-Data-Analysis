"use client";
import React, { useState, useEffect, useRef } from 'react';
import {
  Sparkles,
  Loader2,
  FileText,
  CheckCircle2,
  Download,
  FileDown,
  FileType2,
  Pin,
  LayoutDashboard,
  X
} from 'lucide-react';
import { Sidebar } from '../components/Sidebar';
import { MetricTile, TrendChart, SegmentChart, PythonPlot } from '../components/Charts';

const API_BASE = (process.env.NEXT_PUBLIC_API_BASE ?? "http://127.0.0.1:3000/api").replace(/\/$/, "");

type NotebookStep = {
  title: string;
  body?: string;
  code?: string;
};

type Kpi = {
  label: string;
  value: string;
  change: string;
};

type ChartPoint = {
  label: string;
  value: number;
};

type AnalysisResult = {
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

type Dataset = {
  id: string;
  filename: string;
};

type DatasetApiItem = Dataset & {
  profile?: unknown;
};

type BackendStatus = "checking" | "online" | "offline";

type PinnedChart = {
  id: string;
  chart_type: 'kpi' | 'trend' | 'segment' | 'python_plot';
  label: string;
  data: unknown;
  url?: string;
};

function AnalysisSkeleton() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-28 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
        ))}
      </div>
      <div className="h-64 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="h-64 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
        <div className="h-64 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
      </div>
    </div>
  );
}

export default function Workspace() {
  const dashboardRef = useRef<HTMLDivElement | null>(null);
  const [availableDatasets, setAvailableDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetIds, setSelectedDatasetIds] = useState<string[]>([]);
  const [prompt, setPrompt] = useState("");
  const [uploadLoading, setUploadLoading] = useState(false);
  const [analyzeLoading, setAnalyzeLoading] = useState(false);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [backendStatus, setBackendStatus] = useState<BackendStatus>("checking");
  const [error, setError] = useState<string | null>(null);

  const [pinnedCharts, setPinnedCharts] = useState<PinnedChart[]>([]);
  const [showPinned, setShowPinned] = useState(false);
  const [activeNav, setActiveNav] = useState('explore');
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    checkBackend();
    fetchDatasets();
    fetchPinnedCharts();
  }, []);

  async function checkBackend() {
    try {
      const res = await fetch(`${API_BASE}/health`);
      if (res.ok) setBackendStatus("online");
      else setBackendStatus("offline");
    } catch {
      setBackendStatus("offline");
    }
  }

  async function fetchDatasets() {
    try {
      const res = await fetch(`${API_BASE}/datasets`);
      const data = await res.json();
      if (res.ok && data.datasets) {
        setAvailableDatasets((data.datasets as DatasetApiItem[]).map((d) => ({ id: d.id, filename: d.filename })));
      }
    } catch (e) {
      console.error("Failed to fetch datasets", e);
    }
  }

  async function fetchPinnedCharts() {
    try {
      const res = await fetch(`${API_BASE}/pinned-charts`);
      const data = await res.json();
      if (res.ok) setPinnedCharts(data.pinnedCharts);
    } catch (e) {
      console.error("Failed to fetch pinned charts", e);
    }
  }

  async function pinChart(type: PinnedChart['chart_type'], label: string, data: unknown, url?: string) {
    try {
      const res = await fetch(`${API_BASE}/pin-chart`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: '', chart_type: type, label, data, url }),
      });
      if (res.ok) {
        const pinned = await res.json();
        setPinnedCharts(prev => [...prev, pinned]);
      }
    } catch (e) {
      console.error("Pinning failed", e);
    }
  }

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    const formData = new FormData();
    formData.append("file", file);

    setUploadLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/upload`, { method: "POST", body: formData });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);

      await fetchDatasets();
      setSelectedDatasetIds([data.datasetId]);
      setPrompt("");
      setResult(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploadLoading(false);
    }
  }

  async function runAnalysis(inputValue: string, ids = selectedDatasetIds) {
    if (ids.length === 0) {
      setError("Select at least one dataset before running analysis.");
      return;
    }

    setAnalyzeLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/analyze`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ datasetIds: ids, prompt: inputValue }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Analysis failed");
    } finally {
      setAnalyzeLoading(false);
    }
  }

  const toggleDataset = (id: string) => {
    setSelectedDatasetIds(prev =>
      prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
    );
  };

  async function handleConnectSource() {
    try {
      setError(null);
      const res = await fetch(`${API_BASE}/connect-source`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source: "sample" }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to connect source");
      await fetchDatasets();
      if (data.datasetId) {
        setSelectedDatasetIds(prev => [...prev, data.datasetId]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to connect data source");
    }
  }

  function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  }

  function getExportPlotNodes() {
    return Array.from(dashboardRef.current?.querySelectorAll<HTMLElement>("[data-export-plot]") ?? []);
  }

  function exportPlotsAsSvg() {
    const plotNodes = getExportPlotNodes();
    const plotItems = plotNodes
      .map((node) => ({
        title: node.dataset?.exportPlot || "Plot",
        svg: node.querySelector("svg"),
        image: node.querySelector("img"),
      }))
      .filter((item) => Boolean(item.svg || item.image));

    if (plotItems.length === 0) {
      setError("No plots are available to export yet.");
      return;
    }

    if (!mounted) return;

    const serializer = new XMLSerializer();
    const width = 900;
    const sectionHeight = 360;
    const body = plotItems.map((item, index) => {
      const y = index * sectionHeight + 48;
      let visual = "";

      if (item.svg) {
        const clone = item.svg.cloneNode(true) as SVGSVGElement;
        const rect = item.svg.getBoundingClientRect();
        const chartWidth = Math.max(1, Math.round(rect.width || 760));
        const chartHeight = Math.max(1, Math.round(rect.height || 260));
        clone.setAttribute("width", String(chartWidth));
        clone.setAttribute("height", String(chartHeight));
        clone.setAttribute("x", "0");
        clone.setAttribute("y", "0");
        visual = serializer.serializeToString(clone);
      } else if (item.image) {
        const rect = item.image.getBoundingClientRect();
        const imageWidth = Math.max(1, Math.round(rect.width || 760));
        const imageHeight = Math.max(1, Math.round(rect.height || 260));
        visual = `<image href="${escapeXml(item.image.src)}" width="${imageWidth}" height="${imageHeight}" preserveAspectRatio="xMidYMid meet" />`;
      }

      return `
        <text x="24" y="${index * sectionHeight + 28}" font-family="Arial, sans-serif" font-size="18" font-weight="700" fill="#0f172a">${escapeXml(item.title)}</text>
        <g transform="translate(24 ${y})">${visual}</g>
      `;
    }).join("");

    const combinedSvg = `
      <svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${plotItems.length * sectionHeight}" viewBox="0 0 ${width} ${plotItems.length * sectionHeight}">
        <rect width="100%" height="100%" fill="#ffffff"/>
        ${body}
      </svg>
    `.trim();

    downloadBlob(new Blob([combinedSvg], { type: "image/svg+xml;charset=utf-8" }), "insightpilot-plots.svg");
  }

  function exportPlotsAsPdf() {
    const plotNodes = getExportPlotNodes();
    if (plotNodes.length === 0) {
      setError("No plots are available to export yet.");
      return;
    }

    const sectionHeight = 360;
    const sections = plotNodes.map((node) => {
      const title = node.dataset?.exportPlot || "Plot";
      const svg = node.querySelector("svg");
      const image = node.querySelector("img");
      const visual = svg?.outerHTML || (image ? `<img src="${image.src}" alt="${escapeHtml(title)}" />` : "");
      return `
        <section>
          <h2>${escapeHtml(title)}</h2>
          <div className="plot">${visual}</div>
        </section>
      `;
    }).join("");

    const printWindow = window.open("", "_blank");
    if (!printWindow) {
      setError("Could not open the PDF export window. Allow popups and try again.");
      return;
    }

    printWindow.document.write(`
      <!doctype html>
      <html>
        <head>
          <title>InsightPilot Plots</title>
          <style>
            body { margin: 32px; color: #0f172a; font-family: Arial, sans-serif; }
            header { margin-bottom: 24px; }
            h1 { margin: 0 0 6px; font-size: 24px; }
            p { margin: 0; color: #64748b; }
            section { break-inside: avoid; page-break-inside: avoid; margin-bottom: 28px; }
            h2 { font-size: 16px; margin: 0 0 12px; }
            .plot { border: 1px solid #e2e8f0; border-radius: 12px; padding: 18px; }
            svg, img { display: block; max-width: 100%; height: auto; margin: 0 auto; }
            @page { margin: 18mm; }
          </style>
        </head>
        <body>
          <header>
            <h1>InsightPilot Plot Export</h1>
            <p>${new Date().toLocaleString()}</p>
          </header>
          ${sections}
          <script>
            window.onload = () => {
              window.focus();
              window.print();
            };
          </script>
        </body>
      </html>
    `);
    printWindow.document.close();
  }

  async function exportCleanedCsv() {
    if (selectedDatasetIds.length === 0) {
      setError("Select at least one dataset before exporting CSV.");
      return;
    }

    try {
      setError(null);
      const query = encodeURIComponent(selectedDatasetIds.join(","));
      const res = await fetch(`${API_BASE}/export/cleaned-csv?datasetIds=${query}`);
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || "CSV export failed");
      }
      const blob = await res.blob();
      downloadBlob(blob, selectedDatasetIds.length === 1 ? "cleaned-data.csv" : "cleaned-datasets.csv");
    } catch (err) {
      setError(err instanceof Error ? err.message : "CSV export failed");
    }
  }

  function escapeXml(value: string) {
    return value.replace(/[<>&"']/g, (char) => ({
      "<": "&lt;",
      ">": "&gt;",
      "&": "&amp;",
      '"': "&quot;",
      "'": "&apos;",
    })[char] || char);
  }

  function escapeHtml(value: string) {
    return value.replace(/[<>&"']/g, (char) => ({
      "<": "&lt;",
      ">": "&gt;",
      "&": "&amp;",
      '"': "&quot;",
      "'": "&#039;",
    })[char] || char);
  }

  async function unpinChart(id: string) {
    try {
      const res = await fetch(`${API_BASE}/unpin-chart?id=${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      if (res.ok) {
        setPinnedCharts(prev => prev.filter(c => c.id !== id));
      }
    } catch (e) {
      console.error("Unpin failed", e);
    }
  }

  return (
    <div className="flex h-screen bg-slate-50 text-slate-900">
      <Sidebar
        datasets={availableDatasets}
        selectedDatasetIds={selectedDatasetIds}
        onToggleDataset={toggleDataset}
        onConnectSource={handleConnectSource}
        activeNav={activeNav}
        onNavChange={setActiveNav}
      />

      <main className="flex-1 overflow-y-auto">
        <header className="px-8 py-6 flex justify-between items-center border-b border-slate-200 bg-white">
          <div className="flex items-center gap-4">
            <div>
              <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">Explore Workspace</p>
              <h1 className="text-2xl font-bold text-slate-900">AI Data Analyst</h1>
            </div>
            <button
              onClick={() => { setShowPinned(!showPinned); setActiveNav(showPinned ? 'explore' : 'dashboards'); }}
              className={`ml-4 px-3 py-1 rounded-full text-xs font-bold uppercase flex items-center gap-1.5 transition-all ${showPinned ? 'bg-indigo-600 text-white shadow-md' : 'bg-slate-100 text-slate-500 hover:bg-slate-200'}`}
            >
              <LayoutDashboard size={12} />
              {showPinned ? 'Exit Dashboard' : 'Pinned Dashboard'}
            </button>
          </div>
          <div className="flex items-center gap-3">
            <div className={`px-2 py-1 rounded-full text-[10px] font-bold uppercase flex items-center gap-1.5 ${backendStatus === 'online' ? 'bg-emerald-100 text-emerald-600' : 'bg-red-100 text-red-600'}`}>
              <span className={`w-1.5 h-1.5 rounded-full ${backendStatus === 'online' ? 'bg-emerald-500' : 'bg-red-500'}`} />
              Backend {backendStatus}
            </div>
          </div>
        </header>

        <div className="p-8 max-w-6xl mx-auto space-y-8">
          {!showPinned ? (
            <>
              {/* Dataset Selection Area */}
              <div className="bg-white p-6 rounded-2xl border border-slate-200 shadow-sm space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="font-bold text-slate-800 flex items-center gap-2">
                    <FileText size={18} className="text-indigo-500" />
                    Active Datasets
                  </h3>
                  <label className="cursor-pointer bg-indigo-600 text-white px-4 py-2 rounded-lg text-xs font-medium hover:bg-indigo-700 transition-colors">
                    Upload New
                    <input type="file" className="hidden" accept=".csv,.json" onChange={handleFileUpload} />
                  </label>
                </div>

                <div className="flex flex-wrap gap-3">
                  {availableDatasets.length === 0 ? (
                    <p className="text-sm text-slate-400 italic">No datasets available. Please upload a file.</p>
                  ) : (
                    availableDatasets.map(ds => (
                      <div
                        key={ds.id}
                        onClick={() => toggleDataset(ds.id)}
                        className={`px-3 py-2 rounded-xl border cursor-pointer transition-all flex items-center gap-2 text-xs font-medium ${
                          selectedDatasetIds.includes(ds.id)
                            ? 'bg-indigo-50 border-indigo-500 text-indigo-700 shadow-sm'
                            : 'bg-white border-slate-200 text-slate-600 hover:border-slate-300'
                        }`}
                      >
                        {selectedDatasetIds.includes(ds.id) ? <CheckCircle2 size={14} /> : <div className="w-3.5 h-3.5 rounded-full border-2 border-slate-300" />}
                        {ds.filename}
                      </div>
                    ))
                  )}
                </div>
              </div>

              {/* Prompt Area */}
              <div className="relative group">
                <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl blur opacity-25 group-hover:opacity-50 transition duration-1000"></div>
                <div className="relative bg-white p-2 rounded-2xl border border-slate-200 shadow-sm flex items-center gap-2">
                  <div className="p-3 text-indigo-500">
                    <Sparkles size={24} />
                  </div>
                  <input
                    className="flex-1 py-3 px-2 outline-none text-slate-700 placeholder:text-slate-400"
                    placeholder="Ask your data a question... (e.g., 'Compare revenue between these files')"
                    value={prompt}
                    onChange={(e) => setPrompt(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter' && !analyzeLoading) runAnalysis(prompt); }}
                  />
                  <button
                    onClick={() => runAnalysis(prompt)}
                    disabled={analyzeLoading || selectedDatasetIds.length === 0 || !mounted}
                    className="bg-indigo-600 hover:bg-indigo-700 text-white px-6 py-2 rounded-xl font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                  >
                    {analyzeLoading ? <Loader2 className="animate-spin" size={18} /> : "Run Analysis"}
                  </button>
                </div>
                {error && (
                  <p className="px-5 pb-4 text-sm text-red-600">
                    {error}
                  </p>
                )}
              </div>

              {analyzeLoading && !result && <AnalysisSkeleton />}

              {result && !analyzeLoading && (
                <div className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {result.dashboard.kpis.map((kpi, i) => (
                      <MetricTile key={i} {...kpi} onPin={() => pinChart('kpi', kpi.label, { value: kpi.value, change: kpi.change })} />
                    ))}
                  </div>

                  {result.dashboard.plotUrl && (
                    <PythonPlot url={result.dashboard.plotUrl} onPin={() => pinChart('python_plot', 'Python Plot', { url: result.dashboard.plotUrl }, result.dashboard.plotUrl)} />
                  )}

                  <TrendChart data={result.dashboard.trend} onPin={() => pinChart('trend', 'Revenue Trend', result.dashboard.trend)} />

                  <SegmentChart data={result.dashboard.segments} recommendations={result.dashboard.recommendations} onPin={() => pinChart('segment', 'Segment Mix', result.dashboard.segments)} />

                  {result.warnings && result.warnings.length > 0 && (
                    <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-700 space-y-1">
                      {result.warnings.map((w, i) => (
                        <div key={i}>{w}</div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </>
          ) : (
            <div className="space-y-4">
              <h2 className="text-lg font-bold text-slate-900">Pinned Dashboard</h2>
              {pinnedCharts.length === 0 ? (
                <p className="text-sm text-slate-500">No pinned charts yet.</p>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {pinnedCharts.map(chart => (
                    <div key={chart.id} className="bg-white p-4 rounded-2xl border border-slate-200 shadow-sm relative">
                      <button
                        onClick={() => unpinChart(chart.id)}
                        className="absolute top-3 right-3 text-slate-400 hover:text-red-500"
                        title="Unpin"
                      >
                        <X size={14} />
                      </button>
                      <div className="text-xs font-bold text-slate-500 mb-2">{chart.label}</div>
                      <div className="text-xs text-slate-400 capitalize">{chart.chart_type}</div>
                      {chart.url && <img src={chart.url} alt={chart.label} className="mt-2 rounded-lg border border-slate-100" />}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
