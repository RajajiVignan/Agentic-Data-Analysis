"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import {
  LayoutDashboard,
  Download,
  FileDown,
  FileType2,
} from "lucide-react";
import { Sidebar } from "@/components/Sidebar";
import { UploadArea } from "@/components/UploadArea";
import { AnalysisPrompt } from "@/components/AnalysisPrompt";
import { DashboardView, AnalysisSkeleton } from "@/components/DashboardView";
import { PinnedDashboard } from "@/components/PinnedDashboard";
import { DataConnections } from "@/components/DataConnections";
import {
  checkBackend,
  fetchDatasets,
  uploadFile,
  runAnalysis,
  connectSource,
  exportCleanedCsv,
  fetchPinnedCharts,
  pinChart as apiPinChart,
  unpinChart as apiUnpinChart,
  fetchConnections,
} from "@/lib/api";
import { exportPlotsAsSvg, exportPlotsAsPdf } from "@/lib/export";
import type {
  AnalysisResult,
  Dataset,
  BackendStatus,
  PinnedChart,
  ConnectionConfig,
} from "@/lib/api";

type NavTab = "explore" | "dashboards" | "data" | "context" | "share";

export default function Workspace() {
  const dashboardRef = useRef<HTMLDivElement | null>(null);
  const [mounted, setMounted] = useState(false);
  const [availableDatasets, setAvailableDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetIds, setSelectedDatasetIds] = useState<string[]>([]);
  const [prompt, setPrompt] = useState("");
  const [uploadLoading, setUploadLoading] = useState(false);
  const [analyzeLoading, setAnalyzeLoading] = useState(false);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [backendStatus, setBackendStatus] = useState<BackendStatus>("checking");
  const [error, setError] = useState<string | null>(null);
  const [pinnedCharts, setPinnedCharts] = useState<PinnedChart[]>([]);
  const [activeNav, setActiveNav] = useState<NavTab>("explore");
  const [connections, setConnections] = useState<ConnectionConfig[]>([]);

  // --- Initialization ---

  useEffect(() => {
    setMounted(true);
    init();
  }, []);

  async function init() {
    setBackendStatus(await checkBackend());
    await Promise.all([loadDatasets(), loadPinnedCharts(), loadConnections()]);
  }

  async function loadDatasets() {
    try {
      setAvailableDatasets(await fetchDatasets());
    } catch (e) {
      console.error("Failed to fetch datasets", e);
    }
  }

  async function loadPinnedCharts() {
    try {
      setPinnedCharts(await fetchPinnedCharts());
    } catch (e) {
      console.error("Failed to fetch pinned charts", e);
    }
  }

  async function loadConnections() {
    try {
      setConnections(await fetchConnections());
    } catch (e) {
      console.error("Failed to fetch connections", e);
    }
  }

  // --- Dataset selection ---

  const toggleDataset = useCallback((id: string) => {
    setSelectedDatasetIds((prev) =>
      prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]
    );
  }, []);

  // --- Actions ---

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadLoading(true);
    setError(null);
    try {
      const data = await uploadFile(file);
      await loadDatasets();
      setSelectedDatasetIds([data.datasetId]);
      setPrompt("");
      setResult(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploadLoading(false);
    }
  }

  async function handleRunAnalysis() {
    if (selectedDatasetIds.length === 0) {
      setError("Select at least one dataset before running analysis.");
      return;
    }
    setAnalyzeLoading(true);
    setError(null);
    try {
      const data = await runAnalysis(selectedDatasetIds, prompt);
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Analysis failed");
    } finally {
      setAnalyzeLoading(false);
    }
  }

  async function handleConnectSource() {
    try {
      setError(null);
      const data = await connectSource("sample");
      await loadDatasets();
      if (data.datasetId) {
        setSelectedDatasetIds((prev) => [...prev, data.datasetId]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to connect data source");
    }
  }

  async function handleExportCsv() {
    if (selectedDatasetIds.length === 0) {
      setError("Select at least one dataset before exporting CSV.");
      return;
    }
    try {
      setError(null);
      const blob = await exportCleanedCsv(selectedDatasetIds);
      downloadBlob(blob, selectedDatasetIds.length === 1 ? "cleaned-data.csv" : "cleaned-datasets.csv");
    } catch (err) {
      setError(err instanceof Error ? err.message : "CSV export failed");
    }
  }

  function handleExportSvg() {
    const err = exportPlotsAsSvg(dashboardRef.current, mounted);
    if (err) setError(err.message);
  }

  function handleExportPdf() {
    const err = exportPlotsAsPdf(dashboardRef.current);
    if (err) setError(err.message);
  }

  async function handlePinChart(type: PinnedChart["chart_type"], label: string, data: unknown, url?: string) {
    try {
      const saved = await apiPinChart({ chart_type: type, label, data, url });
      setPinnedCharts((prev) => [...prev, saved]);
    } catch (e) {
      console.error("Pinning failed", e);
    }
  }

  async function handleUnpinChart(id: string) {
    try {
      await apiUnpinChart(id);
      setPinnedCharts((prev) => prev.filter((c) => c.id !== id));
    } catch (e) {
      console.error("Unpin failed", e);
    }
  }

  function handleConnectionCreated(datasetId: string, filename: string) {
    loadDatasets();
    setSelectedDatasetIds((prev) => [...prev, datasetId]);
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

  // --- Nav-based page title ---

  const pageTitle: Record<NavTab, { subtitle: string; title: string }> = {
    explore: { subtitle: "Explore Workspace", title: "AI Data Analyst" },
    dashboards: { subtitle: "Dashboards", title: "Pinned Dashboards" },
    data: { subtitle: "Data Sources", title: "Connections & Datasets" },
    context: { subtitle: "Context", title: "Verified Context" },
    share: { subtitle: "Share", title: "Share & Export" },
  };

  // --- Render ---

  return (
    <div className="flex h-screen bg-slate-50 text-slate-900">
      <Sidebar
        datasets={availableDatasets}
        selectedDatasetIds={selectedDatasetIds}
        onToggleDataset={toggleDataset}
        onConnectSource={handleConnectSource}
        activeNav={activeNav}
        onNavChange={(nav) => setActiveNav(nav as NavTab)}
      />

      <main className="flex-1 overflow-y-auto">
        <header className="px-8 py-6 flex justify-between items-center border-b border-slate-200 bg-white">
          <div className="flex items-center gap-4">
            <div>
              <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">
                {pageTitle[activeNav].subtitle}
              </p>
              <h1 className="text-2xl font-bold text-slate-900">
                {pageTitle[activeNav].title}
              </h1>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {activeNav === "explore" && (
              <>
                <button
                  onClick={handleExportCsv}
                  className="p-2 text-slate-400 hover:text-indigo-600 transition-colors rounded-lg hover:bg-slate-100"
                  title="Export CSV"
                >
                  <FileDown size={18} />
                </button>
                <button
                  onClick={handleExportSvg}
                  className="p-2 text-slate-400 hover:text-indigo-600 transition-colors rounded-lg hover:bg-slate-100"
                  title="Export SVG"
                >
                  <FileType2 size={18} />
                </button>
                <button
                  onClick={handleExportPdf}
                  className="p-2 text-slate-400 hover:text-indigo-600 transition-colors rounded-lg hover:bg-slate-100"
                  title="Export PDF"
                >
                  <Download size={18} />
                </button>
              </>
            )}
            <div
              className={`px-2 py-1 rounded-full text-[10px] font-bold uppercase flex items-center gap-1.5 ${
                backendStatus === "online"
                  ? "bg-emerald-100 text-emerald-600"
                  : "bg-red-100 text-red-600"
              }`}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  backendStatus === "online" ? "bg-emerald-500" : "bg-red-500"
                }`}
              />
              Backend {backendStatus}
            </div>
          </div>
        </header>

        <div className="p-8 max-w-6xl mx-auto space-y-8">
          {/* --- EXPLORE TAB --- */}
          {activeNav === "explore" && (
            <>
              <UploadArea
                datasets={availableDatasets}
                selectedDatasetIds={selectedDatasetIds}
                onToggleDataset={toggleDataset}
                onFileUpload={handleFileUpload}
                onConnectDatabase={() => setActiveNav("data")}
                uploadLoading={uploadLoading}
              />

              <AnalysisPrompt
                prompt={prompt}
                onPromptChange={setPrompt}
                onRun={handleRunAnalysis}
                loading={analyzeLoading}
                disabled={analyzeLoading || selectedDatasetIds.length === 0 || !mounted}
                error={error}
              />

              {analyzeLoading && !result && <AnalysisSkeleton />}

              {result && !analyzeLoading && (
                <DashboardView
                  result={result}
                  dashboardRef={dashboardRef}
                  onPinChart={handlePinChart}
                />
              )}
            </>
          )}

          {/* --- DASHBOARDS TAB --- */}
          {activeNav === "dashboards" && (
            <PinnedDashboard charts={pinnedCharts} onUnpin={handleUnpinChart} />
          )}

          {/* --- DATA TAB --- */}
          {activeNav === "data" && (
            <DataConnections
              connections={connections}
              onRefreshConnections={loadConnections}
              onConnectionCreated={handleConnectionCreated}
            />
          )}

          {/* --- CONTEXT TAB --- */}
          {activeNav === "context" && (
            <div className="space-y-4">
              <h2 className="text-lg font-bold text-slate-900">Verified Context</h2>
              <div className="p-4 rounded-xl bg-slate-800/50 border border-slate-200 space-y-2">
                <div className="flex items-center justify-between text-sm font-semibold text-slate-700">
                  <span>Business Rules</span>
                  <span className="px-1.5 py-0.5 rounded-md bg-emerald-100 text-emerald-600 text-[10px]">Verified</span>
                </div>
                <ul className="text-sm space-y-1 text-slate-600 list-disc pl-4">
                  <li>Revenue excludes failed payments.</li>
                  <li>Enterprise is ARR &gt; $50k.</li>
                  <li>Churn risk is measured as a percentage.</li>
                </ul>
              </div>
            </div>
          )}

          {/* --- SHARE TAB --- */}
          {activeNav === "share" && (
            <div className="space-y-4">
              <h2 className="text-lg font-bold text-slate-900">Share & Export</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <button
                  onClick={handleExportCsv}
                  className="p-6 bg-white rounded-xl border border-slate-200 shadow-sm hover:border-indigo-300 hover:shadow-md transition-all text-left"
                >
                  <FileDown size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Export Cleaned CSV</div>
                  <div className="text-xs text-slate-400 mt-1">Download selected datasets as CSV</div>
                </button>
                <button
                  onClick={handleExportSvg}
                  className="p-6 bg-white rounded-xl border border-slate-200 shadow-sm hover:border-indigo-300 hover:shadow-md transition-all text-left"
                >
                  <FileType2 size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Export SVG</div>
                  <div className="text-xs text-slate-400 mt-1">Save dashboard plots as SVG</div>
                </button>
                <button
                  onClick={handleExportPdf}
                  className="p-6 bg-white rounded-xl border border-slate-200 shadow-sm hover:border-indigo-300 hover:shadow-md transition-all text-left"
                >
                  <Download size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Export PDF</div>
                  <div className="text-xs text-slate-400 mt-1">Save dashboard as PDF report</div>
                </button>
                <div className="p-6 bg-white rounded-xl border border-slate-200 shadow-sm opacity-50">
                  <LayoutDashboard size={24} className="text-slate-300 mb-2" />
                  <div className="text-sm font-semibold text-slate-400">Share Link</div>
                  <div className="text-xs text-slate-300 mt-1">Coming soon</div>
                </div>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
