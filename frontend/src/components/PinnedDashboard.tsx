"use client";

import React from "react";
import { X } from "lucide-react";
import type { PinnedChart } from "@/lib/api";

type PinnedDashboardProps = {
  charts: PinnedChart[];
  onUnpin: (id: string) => void;
};

export function PinnedDashboard({ charts, onUnpin }: PinnedDashboardProps) {
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-bold text-slate-900">Pinned Dashboard</h2>
      {charts.length === 0 ? (
        <p className="text-sm text-slate-500">No pinned charts yet.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {charts.map((chart) => (
            <div key={chart.id} className="bg-white p-4 rounded-2xl border border-slate-200 shadow-sm relative">
              <button
                onClick={() => onUnpin(chart.id)}
                className="absolute top-3 right-3 text-slate-400 hover:text-red-500"
                title="Unpin"
              >
                <X size={14} />
              </button>
              <div className="text-xs font-bold text-slate-500 mb-2">{chart.label}</div>
              <div className="text-xs text-slate-400 capitalize">{chart.chart_type}</div>
              {chart.url && (
                <img src={chart.url} alt={chart.label} className="mt-2 rounded-lg border border-slate-100" />
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
