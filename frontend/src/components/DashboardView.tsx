"use client";

import React from "react";
import { MetricTile, TrendChart, SegmentChart, PythonPlot } from "@/components/Charts";
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
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="h-64 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
        <div className="h-64 rounded-2xl border border-slate-200 bg-slate-100 animate-pulse" />
      </div>
    </div>
  );
}

export function DashboardView({ result, dashboardRef, onPinChart }: DashboardViewProps) {
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

      <TrendChart
        data={result.dashboard.trend}
        onPin={() => onPinChart("trend", "Revenue Trend", result.dashboard.trend)}
      />

      <SegmentChart
        data={result.dashboard.segments}
        recommendations={result.dashboard.recommendations}
        onPin={() => onPinChart("segment", "Segment Mix", result.dashboard.segments)}
      />

      {result.warnings && result.warnings.length > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-700 space-y-1">
          {result.warnings.map((w, i) => (
            <div key={i}>{w}</div>
          ))}
        </div>
      )}
    </div>
  );
}

export { AnalysisSkeleton };
