"use client";

import React, { useState } from "react";
import { ChevronDown, ChevronRight, Code } from "lucide-react";
import { MetricTile, PythonPlot, TrendChart, SegmentChart } from "@/components/Charts";
import type { AnalysisResult, PinnedChart } from "@/lib/api";

type DashboardViewProps = {
  result: AnalysisResult;
  dashboardRef: React.RefObject<HTMLDivElement | null>;
  onPinChart: (type: PinnedChart["chart_type"], label: string, data: unknown, url?: string) => void;
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
    </div>
  );
}

export function DashboardView({ result, dashboardRef, onPinChart }: DashboardViewProps) {
  const [showSql, setShowSql] = useState(false);

  return (
    <div ref={dashboardRef} className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {result.dashboard.kpis.map((kpi, i) => (
          <MetricTile
            key={i}
            {...kpi}
            onPin={() => onPinChart("kpi", kpi.label, { value: kpi.value, change: kpi.change })}
          />
        ))}
      </div>

      {result.dashboard.plotUrl && (
        <PythonPlot
          url={result.dashboard.plotUrl}
          onPin={() =>
            result.dashboard.plotUrl
              ? onPinChart("python_plot", "Python Plot", { url: result.dashboard.plotUrl }, result.dashboard.plotUrl)
              : undefined
          }
        />
      )}

      {result.dashboard.trend.length > 0 || result.dashboard.segments.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {result.dashboard.trend.length > 0 && (
            <TrendChart
              data={result.dashboard.trend}
              onPin={() => {
                const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                onPinChart("trend", `Trend: ${latest?.label ?? "overview"}`, result.dashboard.trend);
              }}
            />
          )}
          {result.dashboard.segments.length > 0 && (
            <SegmentChart
              data={result.dashboard.segments}
              onPin={() => onPinChart("segment", "Segment Breakdown", result.dashboard.segments)}
            />
          )}
        </div>
      ) : null}

      {result.warnings && result.warnings.length > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-700 space-y-1">
          {result.warnings.map((w, i) => (
            <div key={i}>{w}</div>
          ))}
        </div>
      )}

      {result.sqlQueries && result.sqlQueries.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <button
            onClick={() => setShowSql(!showSql)}
            className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50 transition-colors"
          >
            <div className="flex items-center gap-2">
              <Code size={16} className="text-indigo-500" />
              Generated SQL Queries
            </div>
            {showSql ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          </button>
          {showSql && (
            <div className="px-5 pb-4 space-y-3">
              {result.sqlQueries.map((sql, i) => (
                <pre
                  key={i}
                  className="p-3 bg-slate-900 text-slate-100 text-xs rounded-lg overflow-x-auto font-mono leading-relaxed"
                >
                  {sql}
                </pre>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export { AnalysisSkeleton };
