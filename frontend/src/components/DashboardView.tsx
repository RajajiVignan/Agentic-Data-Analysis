"use client";

import React, { useState } from "react";
import { ChevronDown, ChevronRight, Code, BarChart3, Sparkles } from "lucide-react";
import { MetricTile, PythonPlot, SmartAutoViz, ExplainSection, PlotLoading, DashboardFilterBar } from "@/components/Charts";
import { DashboardFilterProvider } from "@/components/DashboardFilterContext";
import type { AnalysisResult, PinnedChart } from "@/lib/api";

type DashboardViewProps = {
  result: AnalysisResult;
  dashboardRef: React.RefObject<HTMLDivElement | null>;
  onPinChart: (type: PinnedChart["chart_type"], label: string, data: unknown, url?: string) => void;
  onRunFollowUp?: (prompt: string) => void;
};

function AnalysisSkeleton() {
  return (
    <div className="space-y-4 animate-fade-in">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div
            key={i}
            className="h-28 rounded-2xl animate-shimmer rounded-xl"
            style={{ animationDelay: `${i * 100}ms` }}
          />
        ))}
      </div>
      <div className="card-modern rounded-2xl">
        <PlotLoading />
      </div>
    </div>
  );
}

export function DashboardView({ result, dashboardRef, onPinChart, onRunFollowUp }: DashboardViewProps) {
  const [showSql, setShowSql] = useState(false);
  const [chartMode, setChartMode] = useState<'auto' | 'all'>('auto');
  const hasTrend = result.dashboard.trend.length > 0;
  const hasSegments = result.dashboard.segments.length > 0;
  const hasNarrative = !!result.dashboard.narrative;
  const hasExplanations = result.dashboard.explanations && result.dashboard.explanations.length > 0;
  const suggestedChartType = result.dashboard.chartType ?? 'bar';

  return (
    <DashboardFilterProvider>
    <div ref={dashboardRef} className="space-y-6 animate-slide-up">
      {/* Narrative summary */}
      {hasNarrative && (
        <div className="p-5 bg-gradient-to-br from-indigo-50/80 to-purple-50/80 border border-indigo-100/80 rounded-2xl card-hover backdrop-blur-sm">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 p-1.5 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-lg shadow-sm">
              <Sparkles size={16} className="text-white" />
            </div>
            <div className="flex-1">
              <p className="text-[10px] font-semibold text-indigo-600 uppercase tracking-widest mb-1.5">AI Insight</p>
              <p className="text-sm text-slate-700 leading-relaxed">{result.dashboard.narrative}</p>
            </div>
          </div>
        </div>
      )}

      {/* Cross-filter bar */}
      <DashboardFilterBar />

      {/* KPI tiles */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 stagger-1">
        {result.dashboard.kpis.map((kpi, i) => (
          <div key={i} className="animate-slide-up" style={{ animationDelay: `${i * 80}ms` }}>
            <MetricTile
              {...kpi}
              onPin={() => onPinChart("kpi", kpi.label, { value: kpi.value, change: kpi.change })}
            />
          </div>
        ))}
      </div>

      {/* Python plot */}
      {result.dashboard.plotUrl && (
        <div className="animate-fade-in">
          <PythonPlot
            url={result.dashboard.plotUrl}
            plotType={result.dashboard.plotType}
            onPin={() =>
              result.dashboard.plotUrl
                ? onPinChart("python_plot", "Python Plot", { url: result.dashboard.plotUrl }, result.dashboard.plotUrl)
                : undefined
            }
          />
        </div>
      )}

      {/* Chart type toggle */}
      {hasTrend || hasSegments ? (
        <div className="flex items-center justify-end gap-2">
          <span className="text-xs text-slate-400">
            <BarChart3 size={14} className="inline mr-1" />
            {chartMode === 'auto' ? `Smart: ${suggestedChartType}` : 'All views'}
          </span>
          <button
            onClick={() => setChartMode(chartMode === 'auto' ? 'all' : 'auto')}
            className="text-xs text-indigo-600 hover:text-indigo-800 font-medium transition-colors"
          >
            Show {chartMode === 'auto' ? 'all' : 'smart'}
          </button>
        </div>
      ) : null}

      {/* Chart grid */}
      {hasTrend || hasSegments ? (
        chartMode === 'auto' ? (
          <div className={`grid grid-cols-1 ${hasTrend && hasSegments ? 'md:grid-cols-2' : ''} gap-6`}>
            {hasTrend && (
              <div className="animate-slide-up">
                <SmartAutoViz
                  data={result.dashboard.trend}
                  chartType={suggestedChartType}
                  onPin={() => {
                    const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                    onPinChart("trend", `${suggestedChartType}: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                  }}
                />
              </div>
            )}
            {hasSegments && (
              <div className="animate-slide-up" style={{ animationDelay: '100ms' }}>
                <SmartAutoViz
                  data={result.dashboard.segments}
                  chartType="pie"
                  onPin={() => onPinChart("segment", "Segment Breakdown", result.dashboard.segments)}
                />
              </div>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {hasTrend && (
              <>
                <div className="animate-slide-up">
                  <SmartAutoViz
                    data={result.dashboard.trend}
                    chartType="bar"
                    onPin={() => {
                      const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                      onPinChart("trend", `Bar: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                    }}
                  />
                </div>
                <div className="animate-slide-up" style={{ animationDelay: '80ms' }}>
                  <SmartAutoViz
                    data={result.dashboard.trend}
                    chartType="line"
                    onPin={() => {
                      const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                      onPinChart("trend", `Line: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                    }}
                  />
                </div>
                {!hasSegments && (
                  <div className="animate-slide-up" style={{ animationDelay: '160ms' }}>
                    <SmartAutoViz
                      data={result.dashboard.trend}
                      chartType="area"
                      onPin={() => {
                        const latest = result.dashboard.trend[result.dashboard.trend.length - 1];
                        onPinChart("trend", `Area: ${latest?.label ?? "overview"}`, result.dashboard.trend);
                      }}
                    />
                  </div>
                )}
              </>
            )}
            {hasSegments && (
              <div className="animate-slide-up">
                <SmartAutoViz
                  data={result.dashboard.segments}
                  chartType="pie"
                  onPin={() => onPinChart("segment", "Segment Breakdown", result.dashboard.segments)}
                />
              </div>
            )}
          </div>
        )
      ) : null}

      {/* Suggested follow-up questions */}
      {result.suggestedQuestions && result.suggestedQuestions.length > 0 && onRunFollowUp && (
        <div className="bg-white rounded-2xl p-4 card-hover card-modern space-y-2">
          <p className="text-[10px] font-semibold text-slate-400 uppercase tracking-widest">Try asking</p>
          <div className="flex flex-wrap gap-2">
            {result.suggestedQuestions.map((q, i) => (
              <button
                key={i}
                onClick={() => onRunFollowUp(q)}
                className="px-3 py-1.5 text-xs font-medium text-indigo-600 bg-indigo-50 hover:bg-indigo-100 rounded-full transition-all border border-indigo-100 hover:border-indigo-200"
              >
                {q}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Warnings */}
      {result.warnings && result.warnings.length > 0 && (
        <div className="bg-amber-50/80 border border-amber-200/80 rounded-2xl p-4 text-xs text-amber-700 space-y-1 backdrop-blur-sm animate-fade-in">
          {result.warnings.map((w, i) => (
            <div key={i} className="flex items-center gap-2">
              <span className="w-1 h-1 rounded-full bg-amber-500 shrink-0" />
              {w}
            </div>
          ))}
        </div>
      )}

      {/* Explainable AI */}
      {hasExplanations && <ExplainSection explanations={result.dashboard.explanations!} />}

      {/* SQL Queries */}
      {result.sqlQueries && result.sqlQueries.length > 0 && (
        <div className="bg-white rounded-2xl overflow-hidden card-hover card-modern">
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
            <div className="px-5 pb-4 space-y-3 animate-slide-up">
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
    </DashboardFilterProvider>
  );
}

export { AnalysisSkeleton };
