"use client";

import React, { useState } from "react";
import { X, Plus, Edit3, Trash2, Check, X as XIcon } from "lucide-react";
import type { PinnedChart, Dashboard } from "@/lib/api";

type PinnedDashboardProps = {
  charts: PinnedChart[];
  dashboards: Dashboard[];
  activeDashboardId: string | null;
  onSelectDashboard: (id: string | null) => void;
  onCreateDashboard: (name: string) => Promise<void>;
  onRenameDashboard: (id: string, name: string) => Promise<void>;
  onDeleteDashboard: (id: string) => Promise<void>;
  onUnpin: (id: string) => void;
};

export function PinnedDashboard({
  charts,
  dashboards,
  activeDashboardId,
  onSelectDashboard,
  onCreateDashboard,
  onRenameDashboard,
  onDeleteDashboard,
  onUnpin,
}: PinnedDashboardProps) {
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");

  const activeDashboard = activeDashboardId
    ? dashboards.find((d) => d.id === activeDashboardId)
    : null;

  function renderChartData(chart: PinnedChart) {
    if (chart.chart_type === "kpi" && typeof chart.data === "object" && chart.data !== null && !Array.isArray(chart.data)) {
      const d = chart.data as Record<string, string>;
      return (
        <div className="mt-2 p-3 bg-indigo-50 rounded-lg">
          <div className="text-2xl font-bold text-slate-900">{d.value ?? "—"}</div>
          <div className="text-xs text-slate-500">{d.change ?? ""}</div>
        </div>
      );
    }
    if (chart.chart_type === "trend" && Array.isArray(chart.data)) {
      const pts = chart.data as { label: string; value: number }[];
      return (
        <div className="mt-2 space-y-1">
          {pts.slice(0, 5).map((pt, i) => (
            <div key={i} className="flex justify-between text-xs text-slate-600">
              <span>{pt.label}</span>
              <span className="font-medium">{pt.value}</span>
            </div>
          ))}
        </div>
      );
    }
    if (chart.chart_type === "segment" && Array.isArray(chart.data)) {
      const pts = chart.data as { label: string; value: number }[];
      return (
        <div className="mt-2 space-y-1">
          {pts.map((pt, i) => (
            <div key={i} className="flex justify-between text-xs text-slate-600">
              <span>{pt.label}</span>
              <span className="font-medium">{pt.value}</span>
            </div>
          ))}
        </div>
      );
    }
    return null;
  }

  const filteredCharts = activeDashboardId
    ? charts.filter((c) => activeDashboard?.chartIds?.includes(c.id))
    : charts;

  async function handleCreate() {
    if (!newName.trim()) return;
    await onCreateDashboard(newName.trim());
    setNewName("");
    setCreating(false);
  }

  async function handleRename(id: string) {
    if (!editName.trim()) return;
    await onRenameDashboard(id, editName.trim());
    setEditingId(null);
    setEditName("");
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-bold text-slate-900">Dashboards</h2>
        <button
          onClick={() => setCreating(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors"
        >
          <Plus size={14} />
          New Dashboard
        </button>
      </div>

      {creating && (
        <div className="flex items-center gap-2 p-3 bg-white rounded-xl border border-slate-200 shadow-sm">
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleCreate()}
            placeholder="Dashboard name..."
            className="flex-1 text-sm px-3 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <button
            onClick={handleCreate}
            className="p-1.5 text-emerald-600 hover:bg-emerald-50 rounded-md"
          >
            <Check size={16} />
          </button>
          <button
            onClick={() => setCreating(false)}
            className="p-1.5 text-slate-400 hover:bg-slate-100 rounded-md"
          >
            <XIcon size={16} />
          </button>
        </div>
      )}

      {/* Dashboard tabs */}
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => onSelectDashboard(null)}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
            activeDashboardId === null
              ? "bg-indigo-100 text-indigo-700"
              : "bg-white text-slate-600 border border-slate-200 hover:border-indigo-300"
          }`}
        >
          All Charts
        </button>
        {dashboards.map((d) => (
          <div key={d.id} className="flex items-center gap-1">
            {editingId === d.id ? (
              <div className="flex items-center gap-1 bg-white rounded-lg border border-slate-200 px-2 py-1">
                <input
                  autoFocus
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleRename(d.id)}
                  className="w-24 text-xs px-1 py-0.5 border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                <button onClick={() => handleRename(d.id)} className="p-0.5 text-emerald-600 hover:bg-emerald-50 rounded">
                  <Check size={12} />
                </button>
                <button onClick={() => setEditingId(null)} className="p-0.5 text-slate-400 hover:bg-slate-100 rounded">
                  <XIcon size={12} />
                </button>
              </div>
            ) : (
              <button
                onClick={() => onSelectDashboard(d.id)}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors flex items-center gap-1.5 ${
                  activeDashboardId === d.id
                    ? "bg-indigo-100 text-indigo-700"
                    : "bg-white text-slate-600 border border-slate-200 hover:border-indigo-300"
                }`}
              >
                {d.name}
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setEditingId(d.id);
                    setEditName(d.name);
                  }}
                  className="p-0.5 text-slate-400 hover:text-indigo-600"
                >
                  <Edit3 size={10} />
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    if (confirm(`Delete dashboard "${d.name}"?`)) onDeleteDashboard(d.id);
                  }}
                  className="p-0.5 text-slate-400 hover:text-red-500"
                >
                  <Trash2 size={10} />
                </button>
              </button>
            )}
          </div>
        ))}
      </div>

      {/* Charts grid */}
      <div>
        <h3 className="text-sm font-semibold text-slate-700 mb-3">
          {activeDashboard ? activeDashboard.name : "All Charts"}
          <span className="text-xs font-normal text-slate-400 ml-2">
            {filteredCharts.length} chart{filteredCharts.length !== 1 ? "s" : ""}
          </span>
        </h3>
        {filteredCharts.length === 0 ? (
          <p className="text-sm text-slate-500">
            {activeDashboard
              ? "No charts pinned to this dashboard yet. Run an analysis and pin charts from the Explore tab."
              : "No pinned charts yet. Run an analysis and pin charts from the Explore tab."}
          </p>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {filteredCharts.map((chart) => (
              <div key={chart.id} className="bg-white p-4 rounded-2xl border border-slate-200 shadow-sm relative">
                <button
                  onClick={() => onUnpin(chart.id)}
                  className="absolute top-3 right-3 text-slate-400 hover:text-red-500"
                  title="Unpin"
                >
                  <X size={14} />
                </button>
                <div className="text-xs font-bold text-slate-500 mb-2">{chart.label}</div>
                <div className="text-xs text-slate-400 capitalize mb-2">{chart.chart_type}</div>
                {chart.url && (
                  <img src={chart.url} alt={chart.label} className="mt-2 rounded-lg border border-slate-100" />
                )}
                {renderChartData(chart)}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
