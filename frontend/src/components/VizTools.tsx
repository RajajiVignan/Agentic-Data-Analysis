"use client";

import React, { useState, useMemo } from "react";
import {
  BarChart3,
  LineChart,
  PieChart,
  AreaChart,
  ScatterChart,
  Table2,
  TrendingUp,
  LayoutDashboard,
  Lightbulb,
  ArrowRight,
  MousePointerClick,
  Sparkles,
  Eye,
  Palette,
  Type,
  Minus,
} from "lucide-react";
import type { Dataset, Column } from "@/lib/api";

type VizToolsProps = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
  onApplySuggestion: (prompt: string) => void;
  accentColor: string;
  chartScheme: string;
  fontFamily: string;
  fontSize: string;
  onAccentChange: (v: string) => void;
  onSchemeChange: (v: string) => void;
  onFontFamilyChange: (v: string) => void;
  onFontSizeChange: (v: string) => void;
};

type ChartTool = {
  id: string;
  label: string;
  icon: React.ReactNode;
  description: string;
  color: string;
};

type Suggestion = {
  prompt: string;
  label: string;
  description: string;
};

const CHART_TOOLS: ChartTool[] = [
  { id: "bar", label: "Bar", icon: <BarChart3 size={18} />, description: "Compare values across categories", color: "from-indigo-500 to-indigo-600" },
  { id: "line", label: "Line", icon: <LineChart size={18} />, description: "Show trends over time", color: "from-emerald-500 to-emerald-600" },
  { id: "pie", label: "Pie", icon: <PieChart size={18} />, description: "Show proportions of a whole", color: "from-amber-500 to-amber-600" },
  { id: "area", label: "Area", icon: <AreaChart size={18} />, description: "Emphasize magnitude of change", color: "from-violet-500 to-violet-600" },
  { id: "scatter", label: "Scatter", icon: <ScatterChart size={18} />, description: "Find correlations between metrics", color: "from-rose-500 to-rose-600" },
  { id: "combo", label: "Combo", icon: <LayoutDashboard size={18} />, description: "Bar + line combined chart", color: "from-cyan-500 to-cyan-600" },
  { id: "table", label: "Table", icon: <Table2 size={18} />, description: "Raw data in tabular format", color: "from-slate-500 to-slate-600" },
  { id: "kpi", label: "KPI", icon: <TrendingUp size={18} />, description: "Highlight a single key metric", color: "from-orange-500 to-orange-600" },
];

function generateSuggestions(columns: Column[], toolId: string): Suggestion[] {
  const numericCols = columns.filter(
    (c) => c.type === "numeric" || c.type === "integer" || c.type === "float" || c.type === "number" || c.type === "real" || c.type === "decimal"
  );
  const categoricalCols = columns.filter(
    (c) => c.type === "text" || c.type === "string" || c.type === "category" || c.type === "varchar" || c.type === "boolean"
  );
  const dateCols = columns.filter(
    (c) => c.type === "date" || c.type === "datetime" || c.type === "time" || c.type === "timestamp"
  );
  const dimCols = dateCols.length > 0 ? dateCols : categoricalCols;

  const suggestions: Suggestion[] = [];

  if (columns.length === 0) return suggestions;

  const topDims = dimCols.slice(0, 4);
  const topMetrics = numericCols.slice(0, 3);

  const addSuggestion = (metric: Column, dim: Column, promptTemplate: string, labelTemplate: string, descTemplate: string) => {
    suggestions.push({
      prompt: promptTemplate.replace("{metric}", metric.name).replace("{dim}", dim.name),
      label: labelTemplate.replace("{metric}", metric.name).replace("{dim}", dim.name),
      description: descTemplate.replace("{metric}", metric.name).replace("{dim}", dim.name),
    });
  };

  switch (toolId) {
    case "bar":
      for (const dim of topDims) {
        for (const metric of topMetrics) {
          addSuggestion(metric, dim, "Show {metric} by {dim} as a bar chart", "{metric} by {dim}", `Compare ${metric.name} across ${dim.name}`);
        }
      }
      if (topDims.length > 0 && topMetrics.length === 0) {
        for (const dim of topDims) {
          suggestions.push({
            prompt: `Show count of records grouped by ${dim.name} as a bar chart`,
            label: `Count by ${dim.name}`,
            description: `Bar chart showing record count by ${dim.name}`,
          });
        }
      }
      break;

    case "line":
      for (const dim of dateCols.length > 0 ? dateCols.slice(0, 2) : topDims) {
        for (const metric of topMetrics) {
          addSuggestion(metric, dim, "Show trend of {metric} over {dim} as a line chart", "{metric} over {dim}", `Line chart of ${metric.name} over ${dim.name}`);
        }
      }
      break;

    case "pie":
      for (const metric of topMetrics.slice(0, 1)) {
        for (const dim of topDims.slice(0, 3)) {
          addSuggestion(metric, dim, "Show {metric} breakdown by {dim} as a pie chart", "{metric} by {dim}", `Pie chart of ${metric.name} by ${dim.name}`);
        }
      }
      if (topMetrics.length === 0 && topDims.length > 0) {
        for (const dim of topDims.slice(0, 3)) {
          suggestions.push({
            prompt: `Show distribution of ${dim.name} as a pie chart`,
            label: `Distribution by ${dim.name}`,
            description: `Pie chart of ${dim.name} distribution`,
          });
        }
      }
      break;

    case "area":
      for (const dim of dateCols.length > 0 ? dateCols.slice(0, 2) : topDims) {
        for (const metric of topMetrics) {
          addSuggestion(metric, dim, "Show {metric} over {dim} as an area chart", "{metric} over {dim}", `Area chart of ${metric.name} over ${dim.name}`);
        }
      }
      break;

    case "scatter":
      for (let i = 0; i < topMetrics.length; i++) {
        for (let j = i + 1; j < topMetrics.length; j++) {
          suggestions.push({
            prompt: `Show scatter plot of ${topMetrics[i].name} vs ${topMetrics[j].name}`,
            label: `${topMetrics[i].name} vs ${topMetrics[j].name}`,
            description: `Scatter plot correlating ${topMetrics[i].name} and ${topMetrics[j].name}`,
          });
        }
      }
      if (topDims.length > 0 && topMetrics.length >= 2) {
        addSuggestion(topMetrics[1], topMetrics[0], "Show scatter plot of {metric} vs {dim} colored by category", "{metric} vs {dim}", `Scatter of ${topMetrics[1].name} vs ${topMetrics[0].name}`);
      }
      break;

    case "combo":
      if (topDims.length > 0 && topMetrics.length >= 2) {
        suggestions.push({
          prompt: `Show combo chart with ${topMetrics[0].name} as bars and ${topMetrics[1].name} as line over ${topDims[0].name}`,
          label: `${topMetrics[0].name} + ${topMetrics[1].name} over ${topDims[0].name}`,
          description: `Combo chart: ${topMetrics[0].name} (bar) and ${topMetrics[1].name} (line) by ${topDims[0].name}`,
        });
      }
      if (topDims.length > 0 && topMetrics.length >= 1) {
        suggestions.push({
          prompt: `Show ${topMetrics[0].name} by ${topDims[0].name} as a combo chart`,
          label: `${topMetrics[0].name} by ${topDims[0].name}`,
          description: `Combo chart of ${topMetrics[0].name} by ${topDims[0].name}`,
        });
      }
      break;

    case "table":
      if (columns.length > 0) {
        const colList = columns.slice(0, 6).map(c => c.name).join(", ");
        suggestions.push({
          prompt: `Show a table with columns: ${columns.slice(0, 6).map(c => c.name).join(", ")}`,
          label: `Full data table`,
          description: `Table showing ${colList}${columns.length > 6 ? ` and ${columns.length - 6} more` : ""}`,
        });
      }
      if (topDims.length > 0 && topMetrics.length > 0) {
        suggestions.push({
          prompt: `Show a summary table of ${topMetrics[0].name} grouped by ${topDims[0].name}`,
          label: `Summary: ${topMetrics[0].name} by ${topDims[0].name}`,
          description: `Summary table of ${topMetrics[0].name} grouped by ${topDims[0].name}`,
        });
      }
      break;

    case "kpi":
      for (const metric of topMetrics) {
        suggestions.push({
          prompt: `Show ${metric.name} as a KPI card with period-over-period change`,
          label: `${metric.name} KPI`,
          description: `KPI card showing ${metric.name} value and trend`,
        });
      }
      if (topMetrics.length >= 2) {
        suggestions.push({
          prompt: `Show KPIs for ${topMetrics.slice(0, 3).map(m => m.name).join(", ")}`,
          label: `Multi-KPI overview`,
          description: `Multiple KPI cards for key metrics`,
        });
      }
      break;

    default:
      break;
  }

  return suggestions.slice(0, 5);
}

export function VizTools({
  datasets, selectedDatasetIds, onApplySuggestion,
  accentColor, chartScheme, fontFamily, fontSize,
  onAccentChange, onSchemeChange, onFontFamilyChange, onFontSizeChange,
}: VizToolsProps) {
  const [selectedTool, setSelectedTool] = useState<string | null>(null);

  const selectedColumns = useMemo(() => {
    if (selectedDatasetIds.length === 0) return [];
    const cols: Column[] = [];
    for (const id of selectedDatasetIds) {
      const ds = datasets.find((d) => d.id === id);
      if (ds?.profile?.columns) {
        cols.push(...ds.profile.columns);
      }
    }
    return cols;
  }, [datasets, selectedDatasetIds]);

  const suggestions = useMemo(() => {
    if (!selectedTool || selectedColumns.length === 0) return [];
    return generateSuggestions(selectedColumns, selectedTool);
  }, [selectedTool, selectedColumns]);

  const handleToolClick = (toolId: string) => {
    setSelectedTool(selectedTool === toolId ? null : toolId);
  };

  const handleApply = (prompt: string) => {
    onApplySuggestion(prompt);
    setSelectedTool(null);
  };

  const activeTool = CHART_TOOLS.find((t) => t.id === selectedTool);

  return (
    <div className="w-full lg:w-72 shrink-0">
      <div className="p-4 bg-white rounded-2xl border border-slate-200/80 shadow-sm space-y-4 sticky top-24">
        {/* Header */}
        <div className="flex items-center gap-2 px-1">
          <div className="p-1.5 bg-indigo-100 rounded-lg">
            <MousePointerClick size={14} className="text-indigo-600" />
          </div>
          <span className="text-xs font-semibold text-slate-700 uppercase tracking-wider">Viz Tools</span>
        </div>

        {/* Tool buttons grid */}
        <div className="grid grid-cols-4 gap-1.5">
          {CHART_TOOLS.map((tool) => {
            const isActive = selectedTool === tool.id;
            const noData = selectedDatasetIds.length === 0;
            return (
              <button
                key={tool.id}
                onClick={() => handleToolClick(tool.id)}
                disabled={noData}
                className={`flex flex-col items-center gap-1 px-2 py-2 rounded-xl text-[10px] font-medium transition-all ${
                  isActive
                    ? "bg-gradient-to-br " + tool.color + " text-white shadow-sm scale-105"
                    : noData
                    ? "text-slate-300 cursor-not-allowed"
                    : "text-slate-500 hover:bg-slate-100 hover:text-slate-700"
                }`}
                title={tool.description}
              >
                {tool.icon}
                <span>{tool.label}</span>
              </button>
            );
          })}
        </div>

        {/* Suggestions panel */}
        {selectedTool && (
          <div className="animate-slide-up space-y-3 pt-2 border-t border-slate-100">
            <div className="flex items-center gap-2 px-1">
              <div className="p-1 bg-indigo-100 rounded-md">
                <Sparkles size={12} className="text-indigo-600" />
              </div>
              <span className="text-[11px] font-semibold text-indigo-600 uppercase tracking-wider">
                {activeTool?.label} Suggestions
              </span>
              <span className="text-[10px] text-slate-400 ml-auto">
                {suggestions.length} options
              </span>
            </div>

            {suggestions.length === 0 ? (
              <div className="p-3 text-center text-xs text-slate-400 bg-slate-50 rounded-xl">
                {selectedColumns.length === 0
                  ? "Select a dataset to get suggestions"
                  : "Not enough columns for this chart type. Try numeric + categorical data."}
              </div>
            ) : (
              <div className="space-y-1.5">
                {suggestions.map((s, i) => (
                  <button
                    key={i}
                    onClick={() => handleApply(s.prompt)}
                    className="w-full text-left p-3 rounded-xl bg-slate-50 hover:bg-indigo-50 border border-transparent hover:border-indigo-100 transition-all group"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="text-xs font-semibold text-slate-700 truncate">{s.label}</p>
                        <p className="text-[10px] text-slate-400 mt-0.5 line-clamp-2">{s.description}</p>
                      </div>
                      <div className="shrink-0 p-1 rounded-md bg-white border border-slate-200 text-slate-400 opacity-0 group-hover:opacity-100 transition-opacity">
                        <ArrowRight size={12} />
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            )}

            {/* Footer hint */}
            <div className="flex items-center gap-1.5 px-1 text-[10px] text-slate-400">
              <Eye size={10} />
              <span>Click a suggestion to run analysis</span>
            </div>
          </div>
        )}

        {/* Empty state when no tool selected */}
        {!selectedTool && selectedDatasetIds.length > 0 && (
          <div className="flex flex-col items-center gap-2 pt-2 pb-1 border-t border-slate-100">
            <Lightbulb size={16} className="text-slate-300" />
            <p className="text-[11px] text-slate-400 text-center leading-relaxed">
              Select a chart type above to see AI-powered suggestions based on your data
            </p>
          </div>
        )}

        {selectedDatasetIds.length === 0 && (
          <div className="flex flex-col items-center gap-2 pt-2 pb-1 border-t border-slate-100">
            <MousePointerClick size={16} className="text-slate-300" />
            <p className="text-[11px] text-slate-400 text-center leading-relaxed">
              Select a dataset from the sidebar to enable visualisation tools
            </p>
          </div>
        )}

        {/* --- Design Section --- */}
        <div className="pt-3 border-t border-slate-100 space-y-3">
          <div className="flex items-center gap-2 px-1">
            <div className="p-1.5 bg-indigo-100 rounded-lg">
              <Palette size={14} className="text-indigo-600" />
            </div>
            <span className="text-xs font-semibold text-slate-700 uppercase tracking-wider">Design</span>
          </div>

          {/* Accent color */}
          <div className="space-y-1.5">
            <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider px-1">Accent Color</label>
            <div className="flex gap-1.5 px-1">
              {[
                { id: "indigo", class: "bg-indigo-500" },
                { id: "blue", class: "bg-blue-500" },
                { id: "emerald", class: "bg-emerald-500" },
                { id: "amber", class: "bg-amber-500" },
                { id: "rose", class: "bg-rose-500" },
                { id: "violet", class: "bg-violet-500" },
              ].map((c) => (
                <button
                  key={c.id}
                  onClick={() => onAccentChange(c.id)}
                  className={`w-6 h-6 rounded-full ${c.class} transition-all ${
                    accentColor === c.id ? "ring-2 ring-offset-2 ring-offset-white ring-indigo-400 scale-110" : "hover:scale-110"
                  }`}
                  title={c.id.charAt(0).toUpperCase() + c.id.slice(1)}
                />
              ))}
            </div>
          </div>

          {/* Chart color scheme */}
          <div className="space-y-1.5">
            <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider px-1">Chart Scheme</label>
            <select
              value={chartScheme}
              onChange={(e) => onSchemeChange(e.target.value)}
              className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white text-slate-600"
            >
              <option value="default">Default</option>
              <option value="warm">Warm</option>
              <option value="cool">Cool</option>
              <option value="mono">Monochrome</option>
            </select>
          </div>

          {/* Font family */}
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 px-1">
              <Type size={11} className="text-slate-400" />
              <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider">Font</label>
            </div>
            <select
              value={fontFamily}
              onChange={(e) => onFontFamilyChange(e.target.value)}
              className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white text-slate-600"
            >
              <option value="system">System</option>
              <option value="inter">Inter</option>
              <option value="georgia">Georgia</option>
              <option value="mono">Monospace</option>
            </select>
          </div>

          {/* Font size */}
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 px-1">
              <Minus size={11} className="text-slate-400" />
              <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider">Size</label>
            </div>
            <select
              value={fontSize}
              onChange={(e) => onFontSizeChange(e.target.value)}
              className="w-full px-2 py-1.5 text-xs border border-slate-200 rounded-lg bg-white text-slate-600"
            >
              <option value="small">Small</option>
              <option value="medium">Medium</option>
              <option value="large">Large</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  );
}
