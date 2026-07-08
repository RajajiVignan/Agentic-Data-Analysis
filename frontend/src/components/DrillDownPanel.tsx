
import { useState } from "react";
import {
  Filter,
  X,
  ChevronDown,
  ChevronRight,
  Search,
  Layers,
  ArrowRight,
  RotateCcw,
} from "lucide-react";
import { useDashboardFilter } from "@/components/DashboardFilterContext";
import type { AnalysisResult, ChartPoint } from "@/lib/api";

type DrillDownPanelProps = {
  result: AnalysisResult;
};

type DrillDownDetail = {
  column: string;
  value: string;
  matchingRows: ChartPoint[];
  totalValue: number;
  percentage: number;
};

export function DrillDownPanel({ result }: DrillDownPanelProps) {
  const { filters, removeFilter, clearFilters } = useDashboardFilter();
  const [expandedFilter, setExpandedFilter] = useState<number | null>(null);
  const [searchTerm, setSearchTerm] = useState("");

  if (filters.length === 0) return null;

  const buildDrillDownDetails = (): DrillDownDetail[] => {
    return filters.map((f) => {
      const sourceData =
        f.column === "label"
          ? [...(result.dashboard.trend || []), ...(result.dashboard.segments || [])]
          : result.dashboard.trend || [];

      const matchingRows = sourceData.filter(
        (d) => d.label === f.value || d.label?.toLowerCase().includes(f.value.toLowerCase())
      );
      const totalValue = matchingRows.reduce((sum, r) => sum + (r.value || 0), 0);
      const allValues = sourceData.reduce((sum, r) => sum + (r.value || 0), 0);
      const percentage = allValues > 0 ? (totalValue / allValues) * 100 : 0;

      return {
        column: f.column,
        value: f.value,
        matchingRows,
        totalValue,
        percentage,
      };
    });
  };

  const details = buildDrillDownDetails();
  const filteredDetails = details.filter(
    (d) =>
      !searchTerm ||
      d.value.toLowerCase().includes(searchTerm.toLowerCase()) ||
      d.column.toLowerCase().includes(searchTerm.toLowerCase())
  );

  return (
    <div className="bg-white rounded-2xl border border-indigo-200/80 shadow-sm overflow-hidden animate-slide-up">
      {/* Header */}
      <div className="px-5 py-3 bg-gradient-to-r from-indigo-50 to-purple-50 border-b border-indigo-100 flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <div className="p-1.5 bg-indigo-100 rounded-lg">
            <Layers size={14} className="text-indigo-600" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-slate-800">Drill-Down View</h3>
            <p className="text-[10px] text-slate-500">
              {filters.length} active filter{filters.length !== 1 ? "s" : ""} applied
            </p>
          </div>
        </div>
        <button
          onClick={clearFilters}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-600 hover:text-red-700 hover:bg-red-50 rounded-lg transition-colors"
        >
          <RotateCcw size={12} />
          Reset All
        </button>
      </div>

      {/* Search */}
      <div className="px-5 py-2 border-b border-slate-100">
        <div className="relative">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="Search filters..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-1 focus:ring-indigo-300 focus:border-indigo-300"
          />
        </div>
      </div>

      {/* Filter details */}
      <div className="divide-y divide-slate-100">
        {filteredDetails.map((detail, i) => {
          const isExpanded = expandedFilter === i;
          return (
            <div key={i} className="px-5">
              {/* Filter chip row */}
              <div className="flex items-center justify-between py-3">
                <div className="flex items-center gap-3 min-w-0">
                  <button
                    onClick={() => setExpandedFilter(isExpanded ? null : i)}
                    className="flex items-center gap-2 min-w-0"
                  >
                    {isExpanded ? (
                      <ChevronDown size={14} className="text-indigo-500 shrink-0" />
                    ) : (
                      <ChevronRight size={14} className="text-slate-400 shrink-0" />
                    )}
                    <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium bg-indigo-50 border border-indigo-200 rounded-full text-indigo-700">
                      <Filter size={10} className="text-indigo-400" />
                      <span className="opacity-60">{detail.column}:</span> {detail.value}
                    </span>
                  </button>
                  <div className="flex items-center gap-2 shrink-0">
                    <div className="w-24 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-indigo-500 rounded-full transition-all duration-500"
                        style={{ width: `${Math.min(detail.percentage, 100)}%` }}
                      />
                    </div>
                    <span className="text-[10px] font-medium text-slate-500 w-10 text-right">
                      {detail.percentage.toFixed(1)}%
                    </span>
                  </div>
                </div>
                <button
                  onClick={() => removeFilter(detail.column)}
                  className="p-1 rounded-md text-slate-400 hover:text-red-500 hover:bg-red-50 transition-colors shrink-0"
                  title="Remove filter"
                >
                  <X size={14} />
                </button>
              </div>

              {/* Expanded detail */}
              {isExpanded && (
                <div className="pb-4 pl-6 animate-slide-up">
                  <div className="bg-slate-50 rounded-xl p-4 space-y-3">
                    <div className="grid grid-cols-3 gap-3 text-center">
                      <div className="p-2 bg-white rounded-lg border border-slate-200">
                        <p className="text-[10px] text-slate-400 uppercase">Matched</p>
                        <p className="text-lg font-bold text-slate-800">
                          {detail.matchingRows.length}
                        </p>
                      </div>
                      <div className="p-2 bg-white rounded-lg border border-slate-200">
                        <p className="text-[10px] text-slate-400 uppercase">Value</p>
                        <p className="text-lg font-bold text-indigo-600">
                          {detail.totalValue.toLocaleString()}
                        </p>
                      </div>
                      <div className="p-2 bg-white rounded-lg border border-slate-200">
                        <p className="text-[10px] text-slate-400 uppercase">Share</p>
                        <p className="text-lg font-bold text-emerald-600">
                          {detail.percentage.toFixed(1)}%
                        </p>
                      </div>
                    </div>

                    {detail.matchingRows.length > 0 && (
                      <div className="space-y-1.5">
                        <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
                          Matching Data Points
                        </p>
                        <div className="max-h-40 overflow-y-auto space-y-1">
                          {detail.matchingRows.map((row, j) => (
                            <div
                              key={j}
                              className="flex items-center justify-between px-3 py-2 bg-white rounded-lg border border-slate-200 text-xs"
                            >
                              <span className="text-slate-700 font-medium truncate">{row.label}</span>
                              <span className="text-indigo-600 font-semibold ml-2">
                                {row.value.toLocaleString()}
                              </span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    <p className="text-[10px] text-slate-400 flex items-center gap-1">
                      <ArrowRight size={10} />
                      Click other chart elements to add more cross-filters
                    </p>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Footer hint */}
      <div className="px-5 py-2.5 bg-slate-50 border-t border-slate-100">
        <p className="text-[10px] text-slate-400 text-center">
          Click any bar or pie segment to drill down. All charts update simultaneously.
        </p>
      </div>
    </div>
  );
}
