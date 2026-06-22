"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import {
  LayoutDashboard,
  Download,
  FileDown,
  GitMerge,
  Terminal,
  CalendarClock,
  Layout,
  Sun,
  Moon,
  BarChart3,
  Menu,
} from "lucide-react";
import { Sidebar } from "@/components/Sidebar";
import { AuthOverlay } from "@/components/AuthOverlay";
import { UploadArea } from "@/components/UploadArea";
import { AnalysisPrompt } from "@/components/AnalysisPrompt";
import { DashboardView, AnalysisSkeleton } from "@/components/DashboardView";
import { PinnedDashboard } from "@/components/PinnedDashboard";
import { DataConnections } from "@/components/DataConnections";
import { TransformationPanel } from "@/components/TransformationPanel";
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
  fetchDashboards,
  createDashboard as apiCreateDashboard,
  renameDashboard as apiRenameDashboard,
  deleteDashboard as apiDeleteDashboard,
  addChartToDashboard as apiAddChartToDashboard,
  removeChartFromDashboard as apiRemoveChartFromDashboard,
  refreshDataset as apiRefreshDataset,
  createShareLink,
  fetchMe,
  logout as apiLogout,
  clearSession as apiClearSession,
  regeneratePlot,
} from "@/lib/api";
import { JoinConfigurator } from "@/components/JoinConfigurator";
import { SchemaDesigner } from "@/components/SchemaDesigner";
import { SQLQueryEditor } from "@/components/SQLQueryEditor";
import { ScheduleManager } from "@/components/ScheduleManager";
import { DashboardEditor } from "@/components/DashboardEditor";
import { DataProfiler } from "@/components/DataProfiler";
import { VizWidget } from "@/components/VizWidget";
import { CommandPalette } from "@/components/CommandPalette";
import { useToast } from "@/components/ToastProvider";
import type { AuthUser } from "@/lib/api";
import { exportPlotsAsPdf } from "@/lib/export";
import type {
  AnalysisResult,
  Dataset,
  BackendStatus,
  PinnedChart,
  ConnectionConfig,
  Dashboard,
  SharedDashboardData,
} from "@/lib/api";

type NavTab = "explore" | "dashboards" | "data" | "context" | "share" | "schema" | "query" | "reports" | "editor" | "profiler";

export default function Workspace() {
  const dashboardRef = useRef<HTMLDivElement | null>(null);
  const [mounted] = useState(true);
  const [availableDatasets, setAvailableDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetIds, setSelectedDatasetIds] = useState<string[]>([]);
  const [prompt, setPrompt] = useState("");
  const [uploadLoading, setUploadLoading] = useState(false);
  const [analyzeLoading, setAnalyzeLoading] = useState(false);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [conversationTurns, setConversationTurns] = useState<{prompt: string; result: AnalysisResult}[]>([]);
  const [backendStatus, setBackendStatus] = useState<BackendStatus>("checking");
  const [error, setError] = useState<string | null>(null);
  const [pinnedCharts, setPinnedCharts] = useState<PinnedChart[]>([]);
  const [activeNav, setActiveNav] = useState<NavTab>("explore");
  const [connections, setConnections] = useState<ConnectionConfig[]>([]);
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [activeDashboardId, setActiveDashboardId] = useState<string | null>(null);
  const [refreshingId, setRefreshingId] = useState<string | null>(null);
  const [shareLink, setShareLink] = useState<string | null>(null);
  const [shareLoading, setShareLoading] = useState(false);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [theme, setTheme] = useState<"light" | "dark">(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-theme") as "light" | "dark" | null : null) ?? "light"
  );
  const [accentColor, setAccentColor] = useState<string>(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-accent") : null) ?? "indigo"
  );
  const [chartScheme, setChartScheme] = useState<string>(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-chart-scheme") : null) ?? "default"
  );
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem("insightpilot-sidebar-pinned") === "true";
  });
  const [sidebarPinned, setSidebarPinned] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem("insightpilot-sidebar-pinned") === "true";
  });
  const [fontFamily, setFontFamily] = useState<string>(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-font-family") : null) ?? "system"
  );
  const [fontSize, setFontSize] = useState<string>(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-font-size") : null) ?? "medium"
  );
  const [vizType, setVizType] = useState<string>(
    () => (typeof window !== "undefined" ? localStorage.getItem("insightpilot-viz-type") : null) ?? "matplotlib"
  );
  const [selectedChartType, setSelectedChartType] = useState<string | null>(null);
  const { addToast } = useToast();

  // --- Initialization ---

  useEffect(() => {
    init();
  }, []);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("insightpilot-theme", theme);
  }, [theme]);

  useEffect(() => {
    // Map accent color names to CSS variable values
    const accentMap: Record<string, { base: string; hover: string; light: string }> = {
      indigo: { base: "#6366f1", hover: "#4f46e5", light: "rgba(99,102,241,0.15)" },
      blue: { base: "#3b82f6", hover: "#2563eb", light: "rgba(59,130,246,0.15)" },
      emerald: { base: "#10b981", hover: "#059669", light: "rgba(16,185,129,0.15)" },
      amber: { base: "#f59e0b", hover: "#d97706", light: "rgba(245,158,11,0.15)" },
      rose: { base: "#f43f5e", hover: "#e11d48", light: "rgba(244,63,94,0.15)" },
      violet: { base: "#8b5cf6", hover: "#7c3aed", light: "rgba(139,92,246,0.15)" },
    };
    const a = accentMap[accentColor] ?? accentMap.indigo;
    document.documentElement.style.setProperty("--accent", a.base);
    document.documentElement.style.setProperty("--accent-hover", a.hover);
    localStorage.setItem("insightpilot-accent", accentColor);
  }, [accentColor]);

  useEffect(() => {
    const schemes: Record<string, string> = {
      default: "#6366f1, #f59e0b, #10b981, #ef4444, #8b5cf6, #06b6d4, #a855f7",
      warm: "#f97316, #ef4444, #eab308, #f43f5e, #d97706, #fb923c, #a16207",
      cool: "#3b82f6, #06b6d4, #6366f1, #14b8a6, #0ea5e9, #8b5cf6, #2dd4bf",
      mono: "#64748b, #94a3b8, #475569, #cbd5e1, #334155, #e2e8f0, #1e293b",
    };
    document.documentElement.style.setProperty("--chart-colors", schemes[chartScheme] ?? schemes.default);
    localStorage.setItem("insightpilot-chart-scheme", chartScheme);
  }, [chartScheme]);

  useEffect(() => {
    const fontMap: Record<string, string> = {
      system: "Arial, Helvetica, sans-serif",
      inter: "Inter, system-ui, sans-serif",
      georgia: "Georgia, Times, serif",
      mono: "'Courier New', Consolas, monospace",
    };
    document.documentElement.style.setProperty("--font-family", fontMap[fontFamily] ?? fontMap.system);
    localStorage.setItem("insightpilot-font-family", fontFamily);
  }, [fontFamily]);

  useEffect(() => {
    document.documentElement.style.setProperty("--base-font-size", fontSize);
    localStorage.setItem("insightpilot-font-size", fontSize);
  }, [fontSize]);

  useEffect(() => {
    localStorage.setItem("insightpilot-sidebar-pinned", String(sidebarPinned));
  }, [sidebarPinned]);

  function handleSidebarToggle() {
    if (sidebarPinned) {
      setSidebarPinned(false);
      setSidebarOpen(false);
    } else {
      setSidebarOpen((prev) => !prev);
    }
  }

  function handleSidebarClose() {
    if (!sidebarPinned) {
      setSidebarOpen(false);
    }
  }

  function handleTogglePin() {
    const next = !sidebarPinned;
    setSidebarPinned(next);
    if (next) {
      setSidebarOpen(true);
    }
  }

  async function init() {
    setBackendStatus(await checkBackend());
    const existingUser = await fetchMe();
    setUser(existingUser);
    setAuthLoading(false);
    if (existingUser) {
      await Promise.all([loadDatasets(), loadPinnedCharts(), loadConnections(), loadDashboards()]);
    }
  }

  function handleAuth(user: AuthUser) {
    setUser(user);
    Promise.all([loadDatasets(), loadPinnedCharts(), loadConnections(), loadDashboards()]);
  }

  async function handleLogout() {
    await apiLogout();
    setUser(null);
    setResult(null);
    setPinnedCharts([]);
    setAvailableDatasets([]);
    setDashboards([]);
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

  async function loadDashboards() {
    try {
      setDashboards(await fetchDashboards());
    } catch (e) {
      console.error("Failed to fetch dashboards", e);
    }
  }

  // --- Dataset selection ---

  const toggleDataset = useCallback((id: string) => {
    setSelectedDatasetIds((prev) => {
      const next = prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id];
      // Reset session if dataset selection changes
      if (JSON.stringify(next) !== JSON.stringify(prev)) {
        setSessionId(null);
        setConversationTurns([]);
        setResult(null);
      }
      return next;
    });
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
        setSessionId(null);
        setConversationTurns([]);
        addToast("File uploaded successfully", "success");
      } catch (err) {
        addToast(err instanceof Error ? err.message : "Upload failed", "error");
      } finally {
      setUploadLoading(false);
    }
  }

  async function handleRunAnalysis(suggestionPrompt?: string) {
    if (selectedDatasetIds.length === 0) {
      setError("Select at least one dataset before running analysis.");
      return;
    }
    setAnalyzeLoading(true);
    setError(null);
    const currentPrompt = suggestionPrompt ?? prompt;
    setPrompt("");
    try {
      const data = await runAnalysis(selectedDatasetIds, currentPrompt, sessionId ?? undefined, vizType, accentColor, chartScheme, fontFamily, fontSize);
      setResult(data);
      if (data.sessionId) {
        setSessionId(data.sessionId);
      }
      setConversationTurns((prev) => [
        ...prev,
        { prompt: currentPrompt, result: data },
      ]);
      addToast("Analysis complete", "success");
    } catch (err) {
      addToast(err instanceof Error ? err.message : "Analysis failed", "error");
    } finally {
      setAnalyzeLoading(false);
    }
  }

  function handleFillPrompt(suggestionPrompt: string) {
    setPrompt(suggestionPrompt);
  }

  async function handleNewAnalysis() {
    if (sessionId) {
      try {
        await apiClearSession(sessionId);
      } catch {
        // ignore
      }
    }
    setSessionId(null);
    setConversationTurns([]);
    setResult(null);
    setPrompt("");
    setError(null);
  }

  async function handleConnectSource() {
    try {
      setError(null);
      const data = await connectSource("sample");
      await loadDatasets();
      if (data.datasetId) {
        setSelectedDatasetIds((prev) => [...prev, data.datasetId]);
      }
      addToast("Sample data connected", "success");
    } catch (err) {
      addToast(err instanceof Error ? err.message : "Failed to connect data source", "error");
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
      addToast("CSV exported successfully", "success");
    } catch (err) {
      addToast(err instanceof Error ? err.message : "CSV export failed", "error");
    }
  }

  function handleExportPdf() {
    const err = exportPlotsAsPdf(dashboardRef.current);
    if (err) setError(err.message);
  }

  function handleVizTypeChange(newVizType: string) {
    setVizType(newVizType);
    localStorage.setItem("insightpilot-viz-type", newVizType);
  }

  async function handlePinChart(type: PinnedChart["chart_type"], label: string, data: unknown, url?: string) {
    try {
      const saved = await apiPinChart({ chart_type: type, label, data, url });
      setPinnedCharts((prev) => [...prev, saved]);
      if (activeDashboardId) {
        await apiAddChartToDashboard(activeDashboardId, saved.id);
        await loadDashboards();
      }
      addToast("Chart pinned to dashboard", "success");
    } catch (e) {
      addToast("Failed to pin chart", "error");
    }
  }

  async function handleUnpinChart(id: string) {
    try {
      await apiUnpinChart(id);
      setPinnedCharts((prev) => prev.filter((c) => c.id !== id));
      if (activeDashboardId) {
        await apiRemoveChartFromDashboard(activeDashboardId, id);
        await loadDashboards();
      }
      addToast("Chart unpinned", "success");
    } catch (e) {
      addToast("Failed to unpin chart", "error");
    }
  }

  async function handleCreateShareLink() {
    setShareLoading(true);
    setError(null);
    try {
      const chartIds = pinnedCharts.map((c) => c.id);
      if (chartIds.length === 0) {
        addToast("Pin some charts first before creating a share link.", "warning");
        setShareLoading(false);
        return;
      }
      const data = await createShareLink(chartIds);
      setShareLink(data.url);
      addToast("Share link created!", "success");
    } catch (err) {
      addToast(err instanceof Error ? err.message : "Failed to create share link", "error");
    } finally {
      setShareLoading(false);
    }
  }

  async function handleRefreshDataset(id: string) {
    setRefreshingId(id);
    try {
      await apiRefreshDataset(id);
      await loadDatasets();
    } catch (e) {
      console.error("Refresh failed", e);
      addToast(e instanceof Error ? e.message : "Refresh failed", "error");
    } finally {
      setRefreshingId(null);
    }
  }

  async function handleCreateDashboard(name: string) {
    try {
      const d = await apiCreateDashboard(name);
      setDashboards((prev) => [...prev, d]);
      setActiveDashboardId(d.id);
      addToast("Dashboard created", "success");
    } catch (e) {
      addToast("Failed to create dashboard", "error");
    }
  }

  async function handleRenameDashboard(id: string, name: string) {
    try {
      await apiRenameDashboard(id, name);
      setDashboards((prev) => prev.map((d) => (d.id === id ? { ...d, name } : d)));
    } catch (e) {
      console.error("Failed to rename dashboard", e);
    }
  }

  async function handleDeleteDashboard(id: string) {
    try {
      await apiDeleteDashboard(id);
      setDashboards((prev) => prev.filter((d) => d.id !== id));
      if (activeDashboardId === id) {
        setActiveDashboardId(null);
      }
    } catch (e) {
      console.error("Failed to delete dashboard", e);
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

  // Auto-regenerate Python plot when design settings change
  const lastDesignRef = useRef<string>("");
  useEffect(() => {
    if (!result?.dashboard?.plotUrl || selectedDatasetIds.length === 0) return;
    if (conversationTurns.length === 0) return;
    const designKey = `${accentColor}|${chartScheme}|${fontFamily}|${fontSize}|${vizType}`;
    if (designKey === lastDesignRef.current) return;
    lastDesignRef.current = designKey;
    const lastPrompt = conversationTurns[conversationTurns.length - 1]?.prompt ?? "";
    const dsId = selectedDatasetIds[0];
    let cancelled = false;
    (async () => {
      const newPlot = await regeneratePlot(dsId, lastPrompt, vizType, accentColor, chartScheme, fontFamily, fontSize);
      if (!cancelled && newPlot?.plotUrl && newPlot.plotUrl !== result?.dashboard?.plotUrl) {
        setResult((prev) => {
          if (!prev) return prev;
          return {
            ...prev,
            dashboard: {
              ...prev.dashboard,
              plotUrl: newPlot.plotUrl,
              plotType: newPlot.vizType,
            },
          };
        });
        setConversationTurns((prev) => {
          if (prev.length === 0) return prev;
          const updated = [...prev];
          const last = { ...updated[updated.length - 1] };
          last.result = {
            ...last.result,
            dashboard: {
              ...last.result.dashboard,
              plotUrl: newPlot.plotUrl,
              plotType: newPlot.vizType,
            },
          };
          updated[updated.length - 1] = last;
          return updated;
        });
      }
    })();
    return () => { cancelled = true; };
  }, [accentColor, chartScheme, fontFamily, fontSize, vizType, result?.dashboard?.plotUrl, selectedDatasetIds]);

  // --- Nav-based page title ---

  const pageTitle: Record<NavTab, { subtitle: string; title: string }> = {
    explore: { subtitle: "Explore Workspace", title: "AI Data Analyst" },
    dashboards: { subtitle: "Dashboards", title: "Pinned Dashboards" },
    data: { subtitle: "Data Sources", title: "Connections & Datasets" },
    context: { subtitle: "Context", title: "Verified Context" },
    share: { subtitle: "Share", title: "Share & Export" },
    schema: { subtitle: "Schema Designer", title: "Visual Schema Designer" },
    query: { subtitle: "SQL Mode", title: "Custom SQL Query" },
    reports: { subtitle: "Automation", title: "Scheduled Reports & Alerts" },
    editor: { subtitle: "Dashboard Editor", title: "Drag-and-Drop Editor" },
    profiler: { subtitle: "Data Profiler", title: "Explore Your Data" },
  };

  // --- Render ---

  if (authLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-50">
        <div className="w-6 h-6 border-2 border-indigo-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!user) {
    return <AuthOverlay onAuth={handleAuth} />;
  }

  return (
    <div className="flex min-h-dvh bg-slate-50 text-slate-900">
      <CommandPalette
        onNavigate={(tab) => setActiveNav(tab as NavTab)}
        onNewAnalysis={handleNewAnalysis}
        onToggleTheme={() => setTheme(theme === "light" ? "dark" : "light")}
        onExportCsv={handleExportCsv}
        onExportPdf={handleExportPdf}
        theme={theme}
        hasData={selectedDatasetIds.length > 0}
        hasResults={conversationTurns.length > 0}
      />
      <Sidebar
        datasets={availableDatasets}
        selectedDatasetIds={selectedDatasetIds}
        onToggleDataset={toggleDataset}
        onConnectSource={handleConnectSource}
        activeNav={activeNav}
        onNavChange={(nav) => setActiveNav(nav as NavTab)}
        isOpen={sidebarOpen}
        onClose={handleSidebarClose}
        isPinned={sidebarPinned}
        onTogglePin={handleTogglePin}
      />

      <main
        className={`flex-1 overflow-y-auto transition-all duration-300 ${sidebarPinned ? 'ml-64' : 'ml-0'}`}
        onClick={handleSidebarClose}
      >
        <header className="sticky top-0 z-20 px-4 sm:px-8 py-3 sm:py-4 flex justify-between items-center border-b border-slate-200 bg-white/80 backdrop-blur-lg">
          <div className="flex items-center gap-3 min-w-0">
            <button
              onClick={(e) => { e.stopPropagation(); handleSidebarToggle(); }}
              className={`p-2 rounded-xl shrink-0 transition-all ${
                sidebarOpen
                  ? 'bg-indigo-100 text-indigo-600 hover:bg-indigo-200'
                  : 'text-slate-400 hover:bg-slate-100 hover:text-slate-600'
              }`}
              title={sidebarPinned ? 'Unpin sidebar' : 'Toggle sidebar'}
            >
              <Menu size={20} />
            </button>
            <div className="min-w-0">
              <p className="text-[10px] sm:text-xs font-medium text-slate-400 uppercase tracking-wider truncate">
                {pageTitle[activeNav].subtitle}
              </p>
              <h1 className="text-lg sm:text-2xl font-bold text-slate-900 truncate">
                {pageTitle[activeNav].title}
              </h1>
            </div>
          </div>
          <div className="flex items-center gap-2 sm:gap-3">
            {user && (
              <div className="flex items-center gap-1 sm:gap-2 px-2 sm:px-3 py-1.5 bg-slate-100 rounded-full">
                <div className="w-5 h-5 bg-indigo-600 rounded-full flex items-center justify-center text-[10px] font-bold text-white shrink-0">
                  {user.name.charAt(0).toUpperCase()}
                </div>
                <span className="hidden sm:inline text-xs font-medium text-slate-600">{user.name}</span>
                <button
                  onClick={handleLogout}
                  className="text-[10px] text-slate-400 hover:text-red-500 transition-colors font-medium"
                >
                  Logout
                </button>
              </div>
            )}
            <button
              onClick={() => setTheme(theme === "light" ? "dark" : "light")}
              className="p-2 text-slate-400 hover:text-indigo-600 transition-colors rounded-lg hover:bg-slate-100"
              title={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
            >
              {theme === "light" ? <Moon size={16} /> : <Sun size={16} />}
            </button>
          </div>
        </header>

        <div className="p-4 sm:p-8 max-w-6xl mx-auto space-y-8">
          {/* --- EXPLORE TAB --- */}
          {activeNav === "explore" && (
            <div className="flex flex-col lg:flex-row gap-8">
              <div className="flex-1 min-w-0 space-y-8">
                <UploadArea
                  datasets={availableDatasets}
                  selectedDatasetIds={selectedDatasetIds}
                  onToggleDataset={toggleDataset}
                  onFileUpload={handleFileUpload}
                  onConnectDatabase={() => setActiveNav("data")}
                  onRefreshDataset={handleRefreshDataset}
                  refreshingId={refreshingId}
                  uploadLoading={uploadLoading}
                />

                {/* Transformation Pipeline */}
                {selectedDatasetIds.length === 1 && (() => {
                  const ds = availableDatasets.find((d) => d.id === selectedDatasetIds[0]);
                  return ds?.profile?.columns ? (
                  <TransformationPanel
                    key={ds.id}
                    datasetId={ds.id}
                    columns={ds.profile.columns}
                    onTransformed={loadDatasets}
                    onExportCsv={handleExportCsv}
                  />
                  ) : null;
                })()}

                {/* Conversation thread — each turn's question + full dashboard */}
                {conversationTurns.length > 0 && (
                  <div className="space-y-8">
                    {conversationTurns.map((turn, i) => (
                      <div key={i} className="bg-white rounded-2xl overflow-hidden card-modern">
                        <div className="px-5 py-3 bg-slate-50 border-b border-slate-100 flex items-center gap-3">
                          <div className="w-6 h-6 bg-indigo-100 rounded-full flex items-center justify-center flex-shrink-0">
                            <span className="text-[10px] font-bold text-indigo-600">Q</span>
                          </div>
                          <p className="text-sm font-medium text-slate-800">{turn.prompt}</p>
                        </div>
                        <div className="p-5">
                          <DashboardView
                            result={turn.result}
                            dashboardRef={dashboardRef}
                            onPinChart={handlePinChart}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {/* Loading skeleton for the latest analysis */}
                {analyzeLoading && <AnalysisSkeleton />}

                {/* Export PDF — all conversation visualizations */}
                {(conversationTurns.length > 0 || result) && (
                  <div className="flex items-center gap-2 p-3 bg-white rounded-2xl card-modern">
                    <span className="text-xs font-medium text-slate-400 mr-2">Export</span>
                    <button
                      onClick={handleExportPdf}
                      className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-slate-600 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-all"
                      title="Export all conversation visualizations as PDF"
                    >
                      <Download size={14} />
                      PDF (All Plots)
                    </button>
                  </div>
                )}

                <AnalysisPrompt
                  prompt={prompt}
                  onPromptChange={setPrompt}
                  onRun={handleRunAnalysis}
                  onNewAnalysis={handleNewAnalysis}
                  loading={analyzeLoading}
                  disabled={analyzeLoading || selectedDatasetIds.length === 0 || !mounted}
                  error={error}
                  hasActiveSession={sessionId !== null && conversationTurns.length > 0}
                />
              </div>

              <VizWidget
                datasets={availableDatasets}
                selectedDatasetIds={selectedDatasetIds}
                onFillPrompt={handleFillPrompt}
                onRunAnalysis={() => handleRunAnalysis()}
                accentColor={accentColor}
                chartScheme={chartScheme}
                fontFamily={fontFamily}
                fontSize={fontSize}
                vizType={vizType}
                onAccentChange={setAccentColor}
                onSchemeChange={setChartScheme}
                onFontFamilyChange={setFontFamily}
                onFontSizeChange={setFontSize}
                onVizTypeChange={handleVizTypeChange}
                selectedChartType={selectedChartType}
                onChartTypeSelect={setSelectedChartType}
                analyzeLoading={analyzeLoading}
                canAnalyze={!analyzeLoading && selectedDatasetIds.length > 0 && selectedChartType !== null}
              />
            </div>
          )}

          {/* --- DASHBOARDS TAB --- */}
          {activeNav === "dashboards" && (
            <PinnedDashboard
              charts={pinnedCharts}
              dashboards={dashboards}
              activeDashboardId={activeDashboardId}
              onSelectDashboard={setActiveDashboardId}
              onCreateDashboard={handleCreateDashboard}
              onRenameDashboard={handleRenameDashboard}
              onDeleteDashboard={handleDeleteDashboard}
              onUnpin={handleUnpinChart}
            />
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

          {/* --- SCHEMA TAB --- */}
          {activeNav === "schema" && (
            <SchemaDesigner
              datasets={availableDatasets}
              onSchemaApplied={(relationships) => {
                // Apply schema: for now just notify the user
                setError(`Schema applied with ${relationships.length} relationship(s). Use the query tab to run cross-table queries.`);
              }}
            />
          )}

          {/* --- QUERY TAB (Feature 5) --- */}
          {activeNav === "query" && (
            <SQLQueryEditor
              datasets={availableDatasets}
              selectedDatasetIds={selectedDatasetIds}
            />
          )}

          {/* --- REPORTS TAB (Feature 6) --- */}
          {activeNav === "reports" && (
            <ScheduleManager datasets={availableDatasets} />
          )}

          {/* --- EDITOR TAB (Feature 7) --- */}
          {activeNav === "editor" && (
            <DashboardEditor onRefreshCharts={loadPinnedCharts} />
          )}

          {/* --- PROFILER TAB (Feature 12) --- */}
          {activeNav === "profiler" && (
            <div className="space-y-4">
              {selectedDatasetIds.length === 0 ? (
                <div className="p-8 text-center text-sm text-slate-400 bg-white rounded-2xl card-modern">
                  <BarChart3 size={32} className="mx-auto mb-3 opacity-50" />
                  <p>Select a dataset from the sidebar to view its profile</p>
                </div>
              ) : (
                selectedDatasetIds.map((id) => (
                  <DataProfiler key={id} datasetId={id} />
                ))
              )}
            </div>
          )}

          {/* --- SHARE TAB --- */}
          {activeNav === "share" && (
            <div className="space-y-4">
              <h2 className="text-lg font-bold text-slate-900">Share & Export</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <button
                  onClick={handleExportCsv}
                  className="p-6 bg-white rounded-xl card-modern hover:shadow-md transition-all text-left hover-glow"
                >
                  <FileDown size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Export Cleaned CSV</div>
                  <div className="text-xs text-slate-400 mt-1">Download selected datasets as CSV</div>
                </button>
                <button
                  onClick={handleExportPdf}
                  className="p-6 bg-white rounded-xl card-modern hover:shadow-md transition-all text-left hover-glow"
                >
                  <Download size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Export PDF</div>
                  <div className="text-xs text-slate-400 mt-1">Save dashboard as PDF report</div>
                </button>
                <button
                  onClick={handleCreateShareLink}
                  disabled={shareLoading || pinnedCharts.length === 0}
                  className="p-6 bg-white rounded-xl card-modern hover:shadow-md transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed hover-glow"
                >
                  <LayoutDashboard size={24} className="text-indigo-500 mb-2" />
                  <div className="text-sm font-semibold text-slate-800">Share Dashboard</div>
                  <div className="text-xs text-slate-400 mt-1">
                    {pinnedCharts.length === 0
                      ? "Pin charts first to create a share link"
                      : "Generate a view-only share link"}
                  </div>
                </button>
              </div>
              {shareLink && (
                <div className="p-4 bg-indigo-50 border border-indigo-200 rounded-xl card-modern">
                  <div className="text-sm font-semibold text-indigo-800 mb-2">Share Link Ready</div>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      readOnly
                      value={shareLink}
                      className="flex-1 p-2 text-sm bg-white border border-indigo-200 rounded-lg text-slate-700"
                      onClick={(e) => e.currentTarget.select()}
                    />
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(shareLink);
                      }}
                      className="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
                    >
                      Copy
                    </button>
                  </div>
                  <p className="text-xs text-indigo-600 mt-2">
                    Link expires in 7 days. Anyone with this link can view the shared charts.
                  </p>
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
