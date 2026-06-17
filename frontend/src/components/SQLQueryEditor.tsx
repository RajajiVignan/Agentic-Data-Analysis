"use client";

import React, { useState, useCallback } from "react";
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
} from "lucide-react";
import { runSQLQuery, fetchQuerySchema, visualizeQuery } from "@/lib/api";
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
import type { Dataset, SchemaInfo, QueryResult } from "@/lib/api";

const COLORS = ["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4"];

type Props = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
};

export function SQLQueryEditor({ datasets, selectedDatasetIds }: Props) {
  const [sql, setSql] = useState("");
  const [result, setResult] = useState<QueryResult | null>(null);
  const [schemas, setSchemas] = useState<SchemaInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSchema, setShowSchema] = useState(true);
  const [chartType, setChartType] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const loadSchema = useCallback(async () => {
    if (selectedDatasetIds.length === 0) return;
    try {
      const data = await fetchQuerySchema(selectedDatasetIds);
      setSchemas(data.schemas);
    } catch {
      // ignore
    }
  }, [selectedDatasetIds]);

  React.useEffect(() => {
    loadSchema();
  }, [loadSchema]);

  async function handleRun() {
    if (!sql.trim() || selectedDatasetIds.length === 0) return;
    setLoading(true);
    setError(null);
    setChartType(null);
    try {
      const data = await runSQLQuery(selectedDatasetIds, sql, page);
      setResult(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Query failed");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }

  async function handleVisualize(type: string) {
    if (!sql.trim() || selectedDatasetIds.length === 0) return;
    setLoading(true);
    setError(null);
    try {
      const data = await visualizeQuery(selectedDatasetIds[0], sql, type);
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

      {/* SQL Editor */}
      <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
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
              onClick={handleRun}
              disabled={loading || !sql.trim() || selectedDatasetIds.length === 0}
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
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            placeholder="SELECT * FROM data WHERE ..."
            className="w-full p-4 font-mono text-sm bg-slate-900 text-green-400 rounded-lg resize-none focus:outline-none min-h-[120px] placeholder:text-slate-500"
            spellCheck={false}
          />
        </div>
      </div>

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-xs text-red-600">
          {error}
        </div>
      )}

      {/* Results grid */}
      {result && result.rows.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-5 py-3 border-b border-slate-100">
            <span className="text-xs font-medium text-slate-500">
              {result.totalRows} rows returned
              {result.hasMore && " (truncated, use LIMIT)"}
            </span>
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
