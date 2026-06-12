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
} from "@/lib/api";
import { exportPlotsAsSvg, exportPlotsAsPdf } from "@/lib/export";
import type {
  AnalysisResult,
  Dataset,
  BackendStatus,
  PinnedChart,
} from "@/lib/api";

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
  const [showPinned, setShowPinned] = useState(false);
  const [activeNav, setActiveNav] = useState("explore");

  // --- Initialization ---

  useEffect(() => {
    setMounted(true);
    init();
  }, []);

  async function init() {
    setBackendStatus(await checkBackend());
    await Promise.all([loadDatasets(), loadPinnedCharts()]);
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

  // --- Render ---

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
              onClick={() => {
                setShowPinned(!showPinned);
                setActiveNav(showPinned ? "explore" : "dashboards");
              }}
              className={`ml-4 px-3 py-1 rounded-full text-xs font-bold uppercase flex items-center gap-1.5 transition-all ${
                showPinned
                  ? "bg-indigo-600 text-white shadow-md"
                  : "bg-slate-100 text-slate-500 hover:bg-slate-200"
              }`}
            >
              <LayoutDashboard size={12} />
              {showPinned ? "Exit Dashboard" : "Pinned Dashboard"}
            </button>
          </div>
          <div className="flex items-center gap-3">
            {!showPinned && (
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
          {!showPinned ? (
            <>
              <UploadArea
                datasets={availableDatasets}
                selectedDatasetIds={selectedDatasetIds}
                onToggleDataset={toggleDataset}
                onFileUpload={handleFileUpload}
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
          ) : (
            <PinnedDashboard charts={pinnedCharts} onUnpin={handleUnpinChart} />
          )}
        </div>
      </main>
    </div>
  );
}
