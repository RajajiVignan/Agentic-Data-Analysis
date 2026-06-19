"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Filter,
  Pencil,
  Trash2,
  Beaker,
  Merge,
  ArrowUpDown,
  Undo2,
  Redo2,
  RotateCcw,
  Eye,
  CheckCircle2,
  Loader2,
  Table2,
  FileDown,
} from "lucide-react";
import type { TransformStep, TransformHistory, Column } from "@/lib/api";
import {
  transformPreview,
  transformApply,
  transformUndo,
  transformRedo,
  transformHistory,
  transformReset,
} from "@/lib/api";

type Tab = "filter" | "rename" | "drop" | "nulls" | "derive" | "aggregate" | "sort";

type Props = {
  datasetId: string;
  columns: Column[];
  onTransformed: () => void;
  onExportCsv?: () => void;
};

export function TransformationPanel({ datasetId, columns, onTransformed, onExportCsv }: Props) {
  const [activeTab, setActiveTab] = useState<Tab>("filter");
  const [history, setHistory] = useState<TransformHistory | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewRows, setPreviewRows] = useState<Record<string, string>[] | null>(null);
  const [showPreview, setShowPreview] = useState(false);

  // Filter params
  const [filterCol, setFilterCol] = useState(columns[0]?.name ?? "");
  const [filterOp, setFilterOp] = useState("eq");
  const [filterVal, setFilterVal] = useState("");

  // Rename params
  const [renameOld, setRenameOld] = useState(columns[0]?.name ?? "");
  const [renameNew, setRenameNew] = useState("");

  // Drop params
  const [dropCols, setDropCols] = useState<string[]>([]);

  // Null handle params
  const [nullCol, setNullCol] = useState(columns[0]?.name ?? "");
  const [nullStrategy, setNullStrategy] = useState("fill");
  const [nullFillVal, setNullFillVal] = useState("0");

  // Derive params
  const [deriveName, setDeriveName] = useState("");
  const [deriveExpr, setDeriveExpr] = useState("");

  // Aggregate params
  const [aggGroupBy, setAggGroupBy] = useState(columns[0]?.name ?? "");
  const [aggCol, setAggCol] = useState("");
  const [aggFn, setAggFn] = useState("sum");
  const [aggNewName, setAggNewName] = useState("");

  // Sort params
  const [sortCol, setSortCol] = useState(columns[0]?.name ?? "");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");

  const loadHistory = useCallback(async () => {
    try {
      const h = await transformHistory(datasetId);
      setHistory(h);
    } catch {
      // ignore
    }
  }, [datasetId]);

  useEffect(() => {
    loadHistory();
  }, [loadHistory]);

  async function handlePreview(step: TransformStep) {
    setLoading(true);
    setError(null);
    try {
      const res = await transformPreview(datasetId, step);
      setPreviewRows(res.rows ?? []);
      setShowPreview(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Preview failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleApply(step: TransformStep) {
    setLoading(true);
    setError(null);
    try {
      await transformApply(datasetId, step);
      setShowPreview(false);
      setPreviewRows(null);
      await loadHistory();
      onTransformed();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Apply failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleUndo() {
    setLoading(true);
    setError(null);
    try {
      await transformUndo(datasetId);
      await loadHistory();
      onTransformed();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Undo failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleRedo() {
    setLoading(true);
    setError(null);
    try {
      await transformRedo(datasetId);
      await loadHistory();
      onTransformed();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Redo failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleReset() {
    setLoading(true);
    setError(null);
    try {
      await transformReset(datasetId);
      setHistory(null);
      setShowPreview(false);
      setPreviewRows(null);
      onTransformed();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Reset failed");
    } finally {
      setLoading(false);
    }
  }

  function buildStep(): TransformStep | null {
    switch (activeTab) {
      case "filter":
        if (!filterCol || !filterOp) return null;
        return { type: "filter", params: { column: filterCol, operator: filterOp, value: filterVal }, description: `Filter ${filterCol} ${filterOp} ${filterVal}` };
      case "rename":
        if (!renameOld || !renameNew) return null;
        return { type: "rename", params: { mappings: { [renameOld]: renameNew } }, description: `Rename ${renameOld} → ${renameNew}` };
      case "drop":
        if (dropCols.length === 0) return null;
        return { type: "drop", params: { columns: dropCols }, description: `Drop ${dropCols.join(", ")}` };
      case "nulls":
        return { type: "null_handle", params: { column: nullCol, strategy: nullStrategy, fillValue: nullFillVal }, description: `${nullStrategy} nulls in ${nullCol}` };
      case "derive":
        if (!deriveName || !deriveExpr) return null;
        return { type: "derive", params: { newColumn: deriveName, expression: deriveExpr }, description: `Derive ${deriveName} = ${deriveExpr}` };
      case "aggregate":
        if (!aggGroupBy || !aggCol) return null;
        return { type: "aggregate", params: { groupBy: aggGroupBy, aggregateColumn: aggCol, function: aggFn, newColumnName: aggNewName || `${aggFn}_${aggCol}` }, description: `${aggFn} ${aggCol} by ${aggGroupBy}` };
      case "sort":
        if (!sortCol) return null;
        return { type: "sort", params: { column: sortCol, order: sortOrder }, description: `Sort by ${sortCol} (${sortOrder})` };
      default:
        return null;
    }
  }

  const step = buildStep();

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: "filter", label: "Filter", icon: <Filter size={14} /> },
    { id: "rename", label: "Rename", icon: <Pencil size={14} /> },
    { id: "drop", label: "Drop", icon: <Trash2 size={14} /> },
    { id: "nulls", label: "Nulls", icon: <Beaker size={14} /> },
    { id: "derive", label: "Derive", icon: <Merge size={14} /> },
    { id: "aggregate", label: "Aggregate", icon: <Table2 size={14} /> },
    { id: "sort", label: "Sort", icon: <ArrowUpDown size={14} /> },
  ];

  return (
    <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
      {/* Header with undo/redo */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-slate-50">
        <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
          <Filter size={16} className="text-indigo-500" />
          Data Transformation Pipeline
        </div>
        <div className="flex items-center gap-1">
          {onExportCsv && (
            <button
              onClick={onExportCsv}
              className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-slate-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-all mr-1"
              title="Export cleaned CSV"
            >
              <FileDown size={13} />
              CSV
            </button>
          )}
          <span className="text-xs text-slate-400 mr-2">{history?.steps?.length ?? 0} steps</span>
          <button
            onClick={handleUndo}
            disabled={!history?.canUndo || loading}
            className="p-1.5 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg disabled:opacity-30 transition-colors"
            title="Undo"
          >
            <Undo2 size={14} />
          </button>
          <button
            onClick={handleRedo}
            disabled={!history?.canRedo || loading}
            className="p-1.5 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg disabled:opacity-30 transition-colors"
            title="Redo"
          >
            <Redo2 size={14} />
          </button>
          <button
            onClick={handleReset}
            disabled={loading}
            className="p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg disabled:opacity-30 transition-colors"
            title="Reset all"
          >
            <RotateCcw size={14} />
          </button>
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex gap-0.5 px-3 pt-3 pb-1 overflow-x-auto bg-slate-50/50">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-t-lg text-xs font-medium transition-colors whitespace-nowrap ${
              activeTab === tab.id
                ? "bg-white text-indigo-700 border border-b-white border-slate-200 shadow-sm"
                : "text-slate-500 hover:text-slate-700 hover:bg-slate-100"
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="p-4 space-y-3 border-t border-slate-200">
        {/* Filter */}
        {activeTab === "filter" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1 min-w-[120px]">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Column</label>
              <select value={filterCol} onChange={(e) => setFilterCol(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                {columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="w-24">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Operator</label>
              <select value={filterOp} onChange={(e) => setFilterOp(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="eq">=</option>
                <option value="neq">≠</option>
                <option value="gt">&gt;</option>
                <option value="gte">≥</option>
                <option value="lt">&lt;</option>
                <option value="lte">≤</option>
                <option value="contains">contains</option>
                <option value="starts_with">starts with</option>
                <option value="ends_with">ends with</option>
                <option value="is_empty">is empty</option>
                <option value="is_not_empty">not empty</option>
              </select>
            </div>
            {filterOp !== "is_empty" && filterOp !== "is_not_empty" && (
              <div className="flex-1 min-w-[120px]">
                <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Value</label>
                <input value={filterVal} onChange={(e) => setFilterVal(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg" placeholder="value..." />
              </div>
            )}
          </div>
        )}

        {/* Rename */}
        {activeTab === "rename" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">From</label>
              <select value={renameOld} onChange={(e) => setRenameOld(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                {columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">To</label>
              <input value={renameNew} onChange={(e) => setRenameNew(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg" placeholder="new name..." />
            </div>
          </div>
        )}

        {/* Drop */}
        {activeTab === "drop" && (
          <div>
            <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Columns to drop</label>
            <div className="flex flex-wrap gap-2">
              {columns.map((c) => (
                <label key={c.name} className={`px-2.5 py-1 rounded-lg text-xs border cursor-pointer transition-colors ${dropCols.includes(c.name) ? "bg-red-50 border-red-300 text-red-700" : "border-slate-200 text-slate-600 hover:border-slate-300"}`}>
                  <input type="checkbox" className="mr-1.5" checked={dropCols.includes(c.name)} onChange={() => setDropCols((prev) => prev.includes(c.name) ? prev.filter((x) => x !== c.name) : [...prev, c.name])} />
                  {c.name}
                </label>
              ))}
            </div>
          </div>
        )}

        {/* Nulls */}
        {activeTab === "nulls" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Column</label>
              <select value={nullCol} onChange={(e) => setNullCol(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="">All columns</option>
                {columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="w-28">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Strategy</label>
              <select value={nullStrategy} onChange={(e) => setNullStrategy(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="fill">Fill value</option>
                <option value="fill_forward">Fill forward</option>
                <option value="fill_backward">Fill backward</option>
                <option value="fill_mean">Fill mean (numeric)</option>
                <option value="drop_row">Drop rows</option>
              </select>
            </div>
            {nullStrategy === "fill" && (
              <div className="flex-1">
                <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Fill value</label>
                <input value={nullFillVal} onChange={(e) => setNullFillVal(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg" placeholder="value..." />
              </div>
            )}
          </div>
        )}

        {/* Derive */}
        {activeTab === "derive" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">New column name</label>
              <input value={deriveName} onChange={(e) => setDeriveName(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg" placeholder="e.g. profit" />
            </div>
            <div className="flex-[2]">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Expression</label>
              <input value={deriveExpr} onChange={(e) => setDeriveExpr(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg font-mono" placeholder="e.g. revenue - cost" />
            </div>
          </div>
        )}

        {/* Aggregate */}
        {activeTab === "aggregate" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Group by</label>
              <select value={aggGroupBy} onChange={(e) => setAggGroupBy(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                {columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Aggregate column</label>
              <select value={aggCol} onChange={(e) => setAggCol(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="">Select...</option>
                {columns.filter((c) => c.type === "number").map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="w-20">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Function</label>
              <select value={aggFn} onChange={(e) => setAggFn(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="sum">SUM</option>
                <option value="avg">AVG</option>
                <option value="count">COUNT</option>
                <option value="min">MIN</option>
                <option value="max">MAX</option>
              </select>
            </div>
          </div>
        )}

        {/* Sort */}
        {activeTab === "sort" && (
          <div className="flex items-end gap-2 flex-wrap">
            <div className="flex-1">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Column</label>
              <select value={sortCol} onChange={(e) => setSortCol(e.target.value)} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                {columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div className="w-20">
              <label className="text-[10px] font-medium text-slate-500 uppercase mb-1 block">Order</label>
              <select value={sortOrder} onChange={(e) => setSortOrder(e.target.value as "asc" | "desc")} className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white">
                <option value="asc">ASC</option>
                <option value="desc">DESC</option>
              </select>
            </div>
          </div>
        )}

        {/* Action buttons */}
        <div className="flex items-center gap-2 pt-2">
          <button
            onClick={() => step && handlePreview(step)}
            disabled={!step || loading}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-slate-600 border border-slate-200 rounded-lg hover:bg-slate-50 disabled:opacity-40 transition-colors"
          >
            {loading ? <Loader2 size={12} className="animate-spin" /> : <Eye size={12} />}
            Preview
          </button>
          <button
            onClick={() => step && handleApply(step)}
            disabled={!step || loading}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 disabled:opacity-40 transition-colors"
          >
            {loading ? <Loader2 size={12} className="animate-spin" /> : <CheckCircle2 size={12} />}
            Apply
          </button>
          {error && <span className="text-xs text-red-500">{error}</span>}
        </div>

        {/* Step history */}
        {history && history.steps && history.steps.length > 0 && (
          <div className="pt-2 border-t border-slate-100">
            <label className="text-[10px] font-medium text-slate-500 uppercase mb-1.5 block">Applied steps ({history.steps.length})</label>
            <div className="space-y-1 max-h-32 overflow-y-auto">
              {history.steps.map((s, i) => (
                <div key={i} className="flex items-center gap-2 text-xs text-slate-600 px-2 py-1 bg-slate-50 rounded-md">
                  <span className="text-indigo-400 font-mono text-[10px]">{i + 1}.</span>
                  <span className="font-medium">{s.type}</span>
                  <span className="text-slate-400 truncate">{s.description}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Preview table */}
      {showPreview && previewRows && previewRows.length > 0 && (
        <div className="border-t border-slate-200">
          <div className="px-4 py-2 bg-slate-50 border-b border-slate-100 flex items-center justify-between">
            <span className="text-[10px] font-medium text-slate-500 uppercase">Preview ({Math.min(previewRows.length, 50)} rows)</span>
            <button onClick={() => setShowPreview(false)} className="text-xs text-slate-400 hover:text-slate-600">&times;</button>
          </div>
          <div className="overflow-x-auto max-h-48 overflow-y-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="bg-slate-50 text-slate-500">
                  {Object.keys(previewRows[0]).map((col) => (
                    <th key={col} className="px-3 py-1.5 text-left font-medium whitespace-nowrap">{col}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {previewRows.slice(0, 10).map((row, i) => (
                  <tr key={i} className="border-t border-slate-100 hover:bg-slate-50">
                    {Object.keys(previewRows[0]).map((col) => (
                      <td key={col} className="px-3 py-1 text-slate-600 max-w-[200px] truncate">{row[col]}</td>
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
