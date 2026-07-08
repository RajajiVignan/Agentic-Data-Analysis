
import React, { useState, useEffect } from "react";
import { BarChart3, Table2, GitBranch, Copy, AlertTriangle } from "lucide-react";
import { fetchDatasetProfile } from "@/lib/api";
import { HistogramChart } from "@/components/Charts";
import type { DatasetProfile, ColumnProfile, CorrelationResult } from "@/lib/api";

type ProfilerProps = {
  datasetId: string;
};

type Tab = "stats" | "distributions" | "correlations" | "duplicates";

export function DataProfiler({ datasetId }: ProfilerProps) {
  const [profile, setProfile] = useState<DatasetProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>("stats");

  useEffect(() => {
    if (!datasetId) return;
    let ignore = false;
    (async () => {
      setLoading(true);
      setError(null);
      try {
        const p = await fetchDatasetProfile(datasetId);
        if (!ignore) setProfile(p);
      } catch (e) {
        if (!ignore) setError(e instanceof Error ? e.message : "Failed");
      } finally {
        if (!ignore) setLoading(false);
      }
    })();
    return () => { ignore = true; };
  }, [datasetId]);

  if (loading) {
    return (
      <div className="p-8 text-center">
        <div className="animate-spin w-6 h-6 border-2 border-indigo-500 border-t-transparent rounded-full mx-auto mb-2" />
        <p className="text-sm text-slate-500">Profiling dataset...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-2xl p-4 text-sm text-red-700">
        {error}
      </div>
    );
  }

  if (!profile) return null;

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: "stats", label: "Column Stats", icon: <Table2 size={14} /> },
    { key: "distributions", label: "Distributions", icon: <BarChart3 size={14} /> },
    { key: "correlations", label: "Correlations", icon: <GitBranch size={14} /> },
    { key: "duplicates", label: "Duplicates", icon: <Copy size={14} /> },
  ];

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-slate-800">{profile.filename}</h3>
          <p className="text-xs text-slate-500">
            {profile.rowCount.toLocaleString()} rows × {profile.columnCount} columns
          </p>
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 bg-slate-100 rounded-xl p-1">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
              activeTab === tab.key
                ? "bg-white text-slate-800 shadow-sm"
                : "text-slate-500 hover:text-slate-700"
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "stats" && <StatsTab columns={profile.columns} />}
      {activeTab === "distributions" && <DistributionsTab histograms={profile.histograms} />}
      {activeTab === "correlations" && <CorrelationsTab correlations={profile.correlations} />}
      {activeTab === "duplicates" && <DuplicatesTab duplicates={profile.duplicates} />}
    </div>
  );
}

function StatsTab({ columns }: { columns: ColumnProfile[] }) {
  return (
    <div className="overflow-x-auto rounded-2xl border border-slate-200">
      <table className="w-full text-xs">
        <thead>
          <tr className="bg-slate-50 text-slate-600">
            <th className="text-left px-4 py-2 font-medium">Column</th>
            <th className="text-left px-4 py-2 font-medium">Type</th>
            <th className="text-right px-4 py-2 font-medium">Non-Empty</th>
            <th className="text-right px-4 py-2 font-medium">Nulls</th>
            <th className="text-right px-4 py-2 font-medium">Unique</th>
            <th className="text-right px-4 py-2 font-medium">Min</th>
            <th className="text-right px-4 py-2 font-medium">Max</th>
            <th className="text-right px-4 py-2 font-medium">Mean</th>
            <th className="text-right px-4 py-2 font-medium">Median</th>
            <th className="text-right px-4 py-2 font-medium">StdDev</th>
          </tr>
        </thead>
        <tbody>
          {columns.map((col, i) => (
            <tr key={i} className={i % 2 === 0 ? "bg-white" : "bg-slate-50/50"}>
              <td className="px-4 py-2 font-medium text-slate-800">{col.name}</td>
              <td className="px-4 py-2">
                <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                  col.type === "number" ? "bg-blue-100 text-blue-700" :
                  col.type === "date" ? "bg-green-100 text-green-700" :
                  "bg-slate-100 text-slate-600"
                }`}>
                  {col.type}
                </span>
              </td>
              <td className="px-4 py-2 text-right">{col.nonEmpty}</td>
              <td className="px-4 py-2 text-right text-red-500">{col.nullCount}</td>
              <td className="px-4 py-2 text-right">{col.uniqueCount}</td>
              <td className="px-4 py-2 text-right font-mono">{col.min?.toLocaleString() ?? "—"}</td>
              <td className="px-4 py-2 text-right font-mono">{col.max?.toLocaleString() ?? "—"}</td>
              <td className="px-4 py-2 text-right font-mono">{col.mean?.toFixed(2) ?? "—"}</td>
              <td className="px-4 py-2 text-right font-mono">{col.median?.toFixed(2) ?? "—"}</td>
              <td className="px-4 py-2 text-right font-mono">{col.stdDev?.toFixed(2) ?? "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DistributionsTab({ histograms }: { histograms?: { column: string; buckets: { label: string; count: number }[] }[] }) {
  if (!histograms || histograms.length === 0) {
    return (
      <div className="p-8 text-center text-sm text-slate-400">
        <BarChart3 size={24} className="mx-auto mb-2 opacity-50" />
        No numeric columns to show distributions for.
      </div>
    );
  }
  return (
    <div className="space-y-4">
      {histograms.map((h, i) => (
        <HistogramChart key={i} column={h.column} buckets={h.buckets} />
      ))}
    </div>
  );
}

function CorrelationsTab({ correlations }: { correlations?: CorrelationResult[] }) {
  if (!correlations || correlations.length === 0) {
    return (
      <div className="p-8 text-center text-sm text-slate-400">
        <GitBranch size={24} className="mx-auto mb-2 opacity-50" />
        Need at least 2 numeric columns to compute correlations.
      </div>
    );
  }
  return (
    <div className="space-y-2">
      {correlations.map((c, i) => {
        const absR = Math.abs(c.r);
        const strength = absR > 0.7 ? "Strong" : absR > 0.4 ? "Moderate" : "Weak";
        const direction = c.r > 0 ? "positive" : "negative";
        const barPct = absR * 100;
        return (
          <div key={i} className="p-3 bg-white rounded-xl border border-slate-200 space-y-1">
            <div className="flex items-center justify-between text-xs">
              <span className="font-medium text-slate-700">{c.col1} ↔ {c.col2}</span>
              <span className="text-slate-500">r = {c.r.toFixed(3)}</span>
            </div>
            <div className="flex items-center gap-2 text-[11px] text-slate-500">
              <span className="px-1.5 py-0.5 bg-indigo-100 text-indigo-700 rounded text-[10px] font-medium">{strength}</span>
              <span>{direction}</span>
            </div>
            <div className="h-2 bg-slate-100 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${c.r > 0 ? "bg-emerald-500" : "bg-red-500"}`}
                style={{ width: `${barPct}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

function DuplicatesTab({ duplicates }: { duplicates: { totalRows: number; duplicateRows: number; duplicateKeys?: string[] } }) {
  const dupPct = duplicates.totalRows > 0 ? ((duplicates.duplicateRows / duplicates.totalRows) * 100).toFixed(1) : "0.0";
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="p-4 bg-white rounded-2xl border border-slate-200 shadow-sm">
          <span className="text-xs text-slate-500">Total Rows</span>
          <p className="text-2xl font-bold text-slate-900">{duplicates.totalRows.toLocaleString()}</p>
        </div>
        <div className="p-4 bg-white rounded-2xl border border-slate-200 shadow-sm">
          <span className="text-xs text-slate-500">Duplicate Rows</span>
          <p className={`text-2xl font-bold ${duplicates.duplicateRows > 0 ? "text-amber-600" : "text-emerald-600"}`}>
            {duplicates.duplicateRows.toLocaleString()}
            <span className="text-sm font-normal text-slate-400 ml-1">({dupPct}%)</span>
          </p>
        </div>
      </div>
      {duplicates.duplicateRows > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-700 space-y-2">
          <div className="flex items-center gap-1.5 font-medium">
            <AlertTriangle size={14} />
            Duplicate Rows Detected
          </div>
          <p>{duplicates.duplicateRows} of {duplicates.totalRows} rows are exact duplicates. Consider cleaning the dataset to avoid skewed analysis results.</p>
          {duplicates.duplicateKeys && duplicates.duplicateKeys.length > 0 && (
            <div className="mt-2">
              <p className="font-medium mb-1">Sample duplicate keys (first column value):</p>
              <ul className="list-disc list-inside space-y-0.5">
                {duplicates.duplicateKeys.map((k, i) => (
                  <li key={i} className="font-mono text-amber-800">{k}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
