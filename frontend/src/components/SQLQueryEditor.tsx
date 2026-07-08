
import { useState, useEffect } from "react";
import {
  Play,
  Code,
  Table2,
  ChevronDown,
  ChevronRight,
  BarChart3,
  LineChart,
  PieChart,
  Loader2,
  Bookmark,
  Plus,
  X,
  Save,
  History,
  Download,
  Trash2,
} from "lucide-react";
import {
  runSQLQuery,
  fetchQuerySchema,
  visualizeQuery,
  fetchQueryHistory,
  fetchSavedQueries,
  createSavedQuery,
  updateSavedQuery,
  deleteSavedQuery,
  runSQLQueryAsync,
  pollQueryJob,
  downloadText,
  queryResultToCSV,
} from "@/lib/api";
import {
  BarChart,
  Bar,
  LineChart as ReLineChart,
  Line,
  PieChart as RePieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import type {
  Dataset,
  SchemaInfo,
  QueryResult,
  QueryHistoryEntry,
  SavedQuery,
  AsyncJobResult,
} from "@/lib/api";

const COLORS = ["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4"];

type Tab = {
  id: string;
  name: string;
  sql: string;
  savedId?: string;
};

let tabCounter = 0;
function newTabId(): string {
  tabCounter += 1;
  return `tab-${tabCounter}-${Date.now().toString(36)}`;
}

type Props = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
};

export function SQLQueryEditor({ datasets, selectedDatasetIds }: Props) {
  const [tabs, setTabs] = useState<Tab[]>(() => [
    { id: newTabId(), name: "Query 1", sql: "" },
  ]);
  const [activeTabId, setActiveTabId] = useState<string>(() => tabs[0].id);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [schemas, setSchemas] = useState<SchemaInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSchema, setShowSchema] = useState(true);
  const [chartType, setChartType] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const [showSaved, setShowSaved] = useState(false);
  const [savedQueries, setSavedQueries] = useState<SavedQuery[]>([]);
  const [savedLoading, setSavedLoading] = useState(false);

  const [showHistory, setShowHistory] = useState(false);
  const [history, setHistory] = useState<QueryHistoryEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  const [asyncRunning, setAsyncRunning] = useState(false);
  const [asyncJobId, setAsyncJobId] = useState<string | null>(null);

  const activeTab = tabs.find((t) => t.id === activeTabId) ?? tabs[0];
  const activeSql = activeTab ? activeTab.sql : "";

  function setActiveSql(value: string) {
    setTabs((ts) => ts.map((t) => (t.id === activeTabId ? { ...t, sql: value } : t)));
  }

  function setActiveSavedId(savedId: string | undefined) {
    setTabs((ts) => ts.map((t) => (t.id === activeTabId ? { ...t, savedId } : t)));
  }

  useEffect(() => {
    if (selectedDatasetIds.length === 0) return;
    let ignore = false;
    (async () => {
      try {
        const data = await fetchQuerySchema(selectedDatasetIds);
        if (!ignore) setSchemas(data.schemas);
      } catch {
        // ignore
      }
    })();
    return () => {
      ignore = true;
    };
  }, [selectedDatasetIds]);

  function addTab() {
    setTabs((ts) => {
      const tab: Tab = {
        id: newTabId(),
        name: `Query ${ts.length + 1}`,
        sql: "",
      };
      setActiveTabId(tab.id);
      return [...ts, tab];
    });
  }

  function closeTab(id: string) {
    setTabs((ts) => {
      if (ts.length <= 1) {
        const fresh: Tab = { id: newTabId(), name: "Query 1", sql: "" };
        setActiveTabId(fresh.id);
        return [fresh];
      }
      const next = ts.filter((t) => t.id !== id);
      if (id === activeTabId) setActiveTabId(next[0].id);
      return next;
    });
  }

  async function handleRun() {
    if (!activeSql.trim() || selectedDatasetIds.length === 0) return;
    setLoading(true);
    setError(null);
    setChartType(null);
    try {
      const data = await runSQLQuery(selectedDatasetIds, activeSql, page);
      setResult(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Query failed");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }

  async function handleVisualize(type: string) {
    if (!activeSql.trim() || selectedDatasetIds.length === 0) return;
    setLoading(true);
    setError(null);
    try {
      const data = await visualizeQuery(selectedDatasetIds[0], activeSql, type);
      setResult({
        columns: data.columns,
        rows: data.rows,
        page: 1,
        pageSize: data.rowCount,
        hasMore: false,
        totalRows: data.rowCount,
      });
      setChartType(type);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Visualize failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleRunAsync() {
    if (!activeSql.trim() || selectedDatasetIds.length === 0) return;
    setLoading(true);
    setError(null);
    setChartType(null);
    try {
      const { jobId } = await runSQLQueryAsync(selectedDatasetIds, activeSql, page);
      setAsyncJobId(jobId);
      setAsyncRunning(true);

      const poll = async () => {
        try {
          const job: AsyncJobResult = await pollQueryJob(jobId);
          if (job.status === "done") {
            setAsyncRunning(false);
            setAsyncJobId(null);
            setResult({
              columns: job.columns,
              rows: job.rows,
              page: job.page,
              pageSize: job.pageSize,
              hasMore: job.hasMore,
              totalRows: job.totalRows,
            });
            setLoading(false);
          } else if (job.status === "error") {
            setAsyncRunning(false);
            setAsyncJobId(null);
            setError(job.error ?? "Async query failed");
            setResult(null);
            setLoading(false);
          } else {
            setTimeout(poll, 1000);
          }
        } catch (e) {
          setAsyncRunning(false);
          setAsyncJobId(null);
          setError(e instanceof Error ? e.message : "Async query failed");
          setResult(null);
          setLoading(false);
        }
      };
      setTimeout(poll, 1000);
    } catch (e) {
      setAsyncRunning(false);
      setAsyncJobId(null);
      setError(e instanceof Error ? e.message : "Async query failed");
      setLoading(false);
    }
  }

  async function loadSaved() {
    setSavedLoading(true);
    try {
      const data = await fetchSavedQueries();
      setSavedQueries(data);
    } catch {
      // ignore
    } finally {
      setSavedLoading(false);
    }
  }

  async function handleToggleSaved() {
    const next = !showSaved;
    setShowSaved(next);
    if (next && savedQueries.length === 0) await loadSaved();
  }

  async function handleDeleteSaved(id: string) {
    try {
      await deleteSavedQuery(id);
      await loadSaved();
    } catch {
      // ignore
    }
  }

  function handleLoadSaved(item: SavedQuery) {
    setActiveSql(item.sql);
    setActiveSavedId(item.id);
  }

  async function handleSaveQuery() {
    if (!activeSql.trim()) return;
    const name = window.prompt(
      "Name this query:",
      activeTab?.savedId ? activeTab.name : "My Query"
    );
    if (name == null) return;
    setTabs((ts) =>
      ts.map((t) => (t.id === activeTabId ? { ...t, name: name || t.name } : t))
    );
    try {
      if (activeTab?.savedId) {
        await updateSavedQuery(activeTab.savedId, name, selectedDatasetIds, activeSql);
      } else {
        const created = await createSavedQuery(name, selectedDatasetIds, activeSql);
        setActiveSavedId(created.id);
      }
      if (showSaved) await loadSaved();
    } catch {
      // ignore
    }
  }

  async function loadHistory() {
    setHistoryLoading(true);
    try {
      const data = await fetchQueryHistory();
      setHistory(data);
    } catch {
      // ignore
    } finally {
      setHistoryLoading(false);
    }
  }

  async function handleToggleHistory() {
    const next = !showHistory;
    setShowHistory(next);
    if (next && history.length === 0) await loadHistory();
  }

  function handleLoadHistory(item: QueryHistoryEntry) {
    setActiveSql(item.sql);
  }

  function handleExportCSV() {
    if (!result) return;
    const csv = queryResultToCSV(result.columns, result.rows);
    downloadText(csv, "query-result.csv", "text/csv");
  }

  function handleExportJSON() {
    if (!result) return;
    downloadText(JSON.stringify(result, null, 2), "query-result.json", "application/json");
  }

  const numericColumns = result?.columns?.filter((c) =>
    result.rows.some((r) => r[c] && !isNaN(Number(r[c])))
  ) ?? [];
  const labelColumns = result?.columns?.filter((c) =>
    result.rows.some((r) => r[c] && isNaN(Number(r[c])))
  ) ?? [];
  const defaultLabel = labelColumns[0] ?? result?.columns?.[0] ?? "";
  const defaultMetric = numericColumns[0] ?? result?.columns?.[1] ?? "";

  const chartData = result?.rows?.map((r) => ({
    label: r[defaultLabel] ?? "",
    value: Number(r[defaultMetric]) || 0,
  })) ?? [];

  return (
    <div className="space-y-4">
      {/* Schema explorer */}
      {schemas.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <button
            onClick={() => setShowSchema(!showSchema)}
            className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
          >
            <div className="flex items-center gap-2">
              <Table2 size={16} className="text-indigo-500" />
              Table Schema
            </div>
            {showSchema ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          </button>
          {showSchema && (
            <div className="px-5 pb-4 space-y-3">
              {schemas.map((s) => (
                <div key={s.datasetId}>
                  <div className="text-xs font-medium text-slate-600 mb-1">
                    {s.filename} <span className="text-slate-400">(as {s.tableAlias})</span>
                  </div>
                  <div className="grid gap-1">
                    {s.columns.map((c) => (
                      <div
                        key={c.name}
                        className="flex items-center gap-2 px-2 py-1 bg-slate-50 rounded text-xs"
                      >
                        <span className="font-mono text-slate-700">{c.name}</span>
                        <span
                          className={`px-1 rounded text-[10px] font-medium ${
                            c.type === "number"
                              ? "bg-blue-100 text-blue-600"
                              : c.type === "date"
                              ? "bg-green-100 text-green-600"
                              : "bg-slate-100 text-slate-500"
                          }`}
                        >
                          {c.type}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Saved queries panel */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        <button
          onClick={handleToggleSaved}
          className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
        >
          <div className="flex items-center gap-2">
            <Bookmark size={16} className="text-indigo-500" />
            Saved Queries
          </div>
          {showSaved ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </button>
        {showSaved && (
          <div className="px-5 pb-4 space-y-2">
            {savedLoading ? (
              <div className="text-xs text-slate-400">Loading…</div>
            ) : savedQueries.length === 0 ? (
              <div className="text-xs text-slate-400">No saved queries yet.</div>
            ) : (
              savedQueries.map((q) => (
                <div
                  key={q.id}
                  className="flex items-center justify-between gap-2 px-3 py-2 bg-slate-50 rounded-lg"
                >
                  <button
                    onClick={() => handleLoadSaved(q)}
                    className="flex-1 text-left truncate text-xs font-medium text-slate-700 hover:text-indigo-600"
                    title={q.name}
                  >
                    {q.name}
                  </button>
                  <button
                    onClick={() => handleDeleteSaved(q.id)}
                    className="p-1 hover:bg-slate-200 rounded text-slate-400 hover:text-red-600"
                    title="Delete saved query"
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              ))
            )}
          </div>
        )}
      </div>

      {/* History panel */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        <button
          onClick={handleToggleHistory}
          className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
        >
          <div className="flex items-center gap-2">
            <History size={16} className="text-indigo-500" />
            Query History
          </div>
          {showHistory ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </button>
        {showHistory && (
          <div className="px-5 pb-4 space-y-2">
            {historyLoading ? (
              <div className="text-xs text-slate-400">Loading…</div>
            ) : history.length === 0 ? (
              <div className="text-xs text-slate-400">No history yet.</div>
            ) : (
              history.map((h) => (
                <button
                  key={h.id}
                  onClick={() => handleLoadHistory(h)}
                  className="w-full text-left px-3 py-2 bg-slate-50 rounded-lg hover:bg-slate-100"
                  title="Load SQL into editor"
                >
                  <div className="truncate font-mono text-xs text-slate-700">
                    {h.sql}
                  </div>
                  <div className="flex items-center gap-2 mt-1 text-[10px] text-slate-400">
                    <span>{new Date(h.executedAt).toLocaleString()}</span>
                    <span>· {h.durationMs}ms</span>
                    {h.error ? (
                      <span className="text-red-500">· error</span>
                    ) : (
                      <span>· {h.rowCount} rows</span>
                    )}
                  </div>
                </button>
              ))
            )}
          </div>
        )}
      </div>

      {/* SQL Editor */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {/* Tab bar */}
        <div className="flex items-center gap-1 px-3 py-2 border-b border-slate-100 bg-slate-50 overflow-x-auto">
          {tabs.map((t) => (
            <div
              key={t.id}
              className={`flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium cursor-pointer whitespace-nowrap ${
                t.id === activeTabId
                  ? "bg-white text-indigo-600 shadow-sm"
                  : "text-slate-500 hover:text-slate-700"
              }`}
              onClick={() => setActiveTabId(t.id)}
            >
              <span>{t.name}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  closeTab(t.id);
                }}
                className="p-0.5 hover:bg-slate-200 rounded text-slate-400 hover:text-slate-600"
                title="Close tab"
              >
                <X size={12} />
              </button>
            </div>
          ))}
          <button
            onClick={addTab}
            className="p-1.5 hover:bg-slate-200 rounded-lg text-slate-400 hover:text-indigo-600"
            title="New tab"
          >
            <Plus size={14} />
          </button>
        </div>

        <div className="flex items-center justify-between px-5 py-3 border-b border-slate-100">
          <div className="flex items-center gap-2">
            <Code size={16} className="text-indigo-500" />
            <span className="text-sm font-semibold text-slate-700">SQL Query</span>
          </div>
          <div className="flex items-center gap-2">
            {result && result.rows.length > 0 && (
              <div className="flex items-center gap-1">
                <button
                  onClick={() => handleVisualize("bar")}
                  className="p-1.5 hover:bg-slate-100 rounded text-slate-400 hover:text-indigo-600"
                  title="Bar Chart"
                >
                  <BarChart3 size={14} />
                </button>
                <button
                  onClick={() => handleVisualize("line")}
                  className="p-1.5 hover:bg-slate-100 rounded text-slate-400 hover:text-indigo-600"
                  title="Line Chart"
                >
                  <LineChart size={14} />
                </button>
                <button
                  onClick={() => handleVisualize("pie")}
                  className="p-1.5 hover:bg-slate-100 rounded text-slate-400 hover:text-indigo-600"
                  title="Pie Chart"
                >
                  <PieChart size={14} />
                </button>
              </div>
            )}
            <button
              onClick={handleSaveQuery}
              disabled={!activeSql.trim()}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-white border border-slate-200 text-slate-600 text-xs font-medium rounded-lg hover:bg-slate-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              title="Save query"
            >
              <Save size={12} />
              Save
            </button>
            <button
              onClick={handleRunAsync}
              disabled={loading || asyncRunning || !activeSql.trim() || selectedDatasetIds.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-amber-500 text-white text-xs font-medium rounded-lg hover:bg-amber-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              title="Run asynchronously"
            >
              {asyncRunning ? (
                <Loader2 size={12} className="animate-spin" />
              ) : (
                <Play size={12} />
              )}
              Run async
            </button>
            <button
              onClick={handleRun}
              disabled={loading || asyncRunning || !activeSql.trim() || selectedDatasetIds.length === 0}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 text-white text-xs font-medium rounded-lg hover:bg-indigo-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? (
                <Loader2 size={12} className="animate-spin" />
              ) : (
                <Play size={12} />
              )}
              Run
            </button>
          </div>
        </div>
        <div className="p-1">
          <textarea
            value={activeSql}
            onChange={(e) => setActiveSql(e.target.value)}
            placeholder="SELECT * FROM data WHERE ..."
            className="w-full p-4 font-mono text-sm bg-slate-900 text-green-400 rounded-lg resize-none focus:outline-none min-h-[120px] placeholder:text-slate-500"
            spellCheck={false}
          />
        </div>
      </div>

      {asyncRunning && (
        <div className="flex items-center gap-2 text-xs text-amber-600">
          <Loader2 size={12} className="animate-spin" />
          Running query asynchronously… (job {asyncJobId})
        </div>
      )}

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-xs text-red-600">
          {error}
        </div>
      )}

      {/* Results grid */}
      {result && result.rows.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-5 py-3 border-b border-slate-100 flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500">
              {result.totalRows} rows returned
              {result.hasMore && " (truncated, use LIMIT)"}
            </span>
            <div className="flex items-center gap-1">
              <button
                onClick={handleExportCSV}
                className="flex items-center gap-1 px-2 py-1 text-xs text-slate-500 hover:text-indigo-600 hover:bg-slate-50 rounded"
                title="Export CSV"
              >
                <Download size={12} />
                Export CSV
              </button>
              <button
                onClick={handleExportJSON}
                className="flex items-center gap-1 px-2 py-1 text-xs text-slate-500 hover:text-indigo-600 hover:bg-slate-50 rounded"
                title="Export JSON"
              >
                <Download size={12} />
                Export JSON
              </button>
            </div>
          </div>

          {/* Chart visualization */}
          {chartType && chartData.length > 0 && (
            <div className="p-5 border-b border-slate-100">
              <ResponsiveContainer width="100%" height={280}>
                {chartType === "bar" ? (
                  <BarChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                    <XAxis dataKey="label" tick={{ fontSize: 11, fill: "#64748b" }} />
                    <YAxis tick={{ fontSize: 11, fill: "#64748b" }} />
                    <Tooltip />
                    <Bar dataKey="value" fill="#6366f1" radius={[4, 4, 0, 0]} />
                  </BarChart>
                ) : chartType === "line" ? (
                  <ReLineChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                    <XAxis dataKey="label" tick={{ fontSize: 11, fill: "#64748b" }} />
                    <YAxis tick={{ fontSize: 11, fill: "#64748b" }} />
                    <Tooltip />
                    <Line type="monotone" dataKey="value" stroke="#6366f1" strokeWidth={2} dot={{ fill: "#6366f1", r: 4 }} />
                  </ReLineChart>
                ) : (
                  <RePieChart>
                    <Pie
                      data={chartData}
                      dataKey="value"
                      nameKey="label"
                      cx="50%"
                      cy="50%"
                      outerRadius={100}
                      label={({ name, percent }) => `${name ?? ""} ${((percent ?? 0) * 100).toFixed(0)}%`}
                    >
                      {chartData.map((_, i) => (
                        <Cell key={i} fill={COLORS[i % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: "11px" }} />
                  </RePieChart>
                )}
              </ResponsiveContainer>
            </div>
          )}

          {/* Table results */}
          <div className="overflow-x-auto max-h-80 overflow-y-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="bg-slate-50">
                  {result.columns.map((col) => (
                    <th
                      key={col}
                      className="px-4 py-2 text-left font-semibold text-slate-600 border-b border-slate-200 whitespace-nowrap"
                    >
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.rows.map((row, i) => (
                  <tr key={i} className="hover:bg-slate-50 even:bg-slate-50/50">
                    {result.columns.map((col) => (
                      <td
                        key={col}
                        className="px-4 py-1.5 text-slate-700 border-b border-slate-100 whitespace-nowrap font-mono"
                      >
                        {row[col] ?? ""}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
