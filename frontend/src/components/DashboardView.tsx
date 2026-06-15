"use client";

import React, { useState } from "react";
import { ChevronDown, ChevronRight, Code, BookOpen } from "lucide-react";
import { MetricTile, PythonPlot, TrendChart, SegmentChart, LineTrendChart, AreaTrendChart } from "@/components/Charts";
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
  const hasTrend = result.dashboard.trend.length > 0;
  const hasSegments = result.dashboard.segments.length > 0;
  const hasNarrative = !!result.dashboard.narrative;

  return (
    <div ref={dashboardRef} className="space-y-6">
      {/* Narrative summary */}
      {hasNarrative && (
        <div className="p-5 bg-gradient-to-br from-indigo-50 to-purple-50 border border-indigo-100 rounded-2xl shadow-sm">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 p-1.5 bg-indigo-100 rounded-lg">
              <BookOpen size={16} className="text-indigo-600" />
            </div>
            <div className="flex-1">
              <p className="text-xs font-semibold text-indigo-700 uppercase tracking-wider mb-1.5">AI Narrative</p>
              <p className="text-sm text-slate-700 leading-relaxed">{result.dashboard.narrative}</p>
            </div>
          </div>
        </div>
      )}

      {/* KPI tiles */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {result.dashboard.kpis.map((kpi, i) => (
          <MetricTile
            key={i}
            {...kpi}
            onPin={() => onPinChart("kpi", kpi.label, { value: kpi.value, change: kpi.change })}
          />
        ))}
      </div>

      {/* Python plot */}
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

      {/* Chart grid: trend (multiple views) + segments */}
      {hasTrend || hasSegments ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {hasTrend && (
            <>
              <TrendChart
                data={result.dashboard.trend}
                onPin={() => {
                  const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                  onPinChart("trend", `Bar: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                }}
              />
              <LineTrendChart
                data={result.dashboard.trend}
                onPin={() => {
                  const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                  onPinChart("trend", `Line: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                }}
              />
              {hasSegments ? null : (
                <AreaTrendChart
                  data={result.dashboard.trend}
                  onPin={() => {
                    const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                    onPinChart("trend", `Area: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                  }}
                />
              )}
            </>
          )}
          {hasSegments && (
            <SegmentChart
              data={result.dashboard.segments}
              onPin={() => onPinChart("segment", "Segment Breakdown", result.dashboard.segments)}
            />
          )}
        </div>
      ) : null}

      {/* Warnings */}
      {result.warnings && result.warnings.length > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 text-xs text-amber-700 space-y-1">
          {result.warnings.map((w, i) => (
            <div key={i}>{w}</div>
          ))}
        </div>
      )}

      {/* SQL Queries */}
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
