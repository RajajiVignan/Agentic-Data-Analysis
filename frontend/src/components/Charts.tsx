import React, { useState } from 'react';
import Image from 'next/image';
import { Pin, Info, ChevronDown, ChevronRight } from 'lucide-react';
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  AreaChart,
  Area,
  ScatterChart,
  Scatter,
  ComposedChart,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
  ZAxis,
} from 'recharts';

type KpiProps = {
  label: string;
  value: string;
  change: string;
};

const COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#06b6d4', '#a855f7'];

type ChartPoint = {
  label: string;
  value: number;
};

export function MetricTile({ label, value, change, onPin }: KpiProps & { onPin?: () => void }) {
  return (
    <div className="p-4 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-1 relative group">
      <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
        <button 
          onClick={onPin}
          className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
          title="Pin to Dashboard"
        >
          <Pin size={12} />
        </button>
      </div>
      <span className="text-xs font-medium text-slate-500">{label}</span>
      <strong className="block text-2xl font-bold text-slate-900">{value}</strong>
      <small className={`text-xs ${change.includes('+') ? 'text-emerald-600' : 'text-slate-500'}`}>
        {change}
      </small>
    </div>
  );
}

export function PythonPlot({ url, onPin }: { url: string; onPin?: () => void }) {
  if (!url) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Python Plot">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">AI Generated Visualization</strong>
        <div className="flex items-center gap-2">
          <button 
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Python (Matplotlib/Seaborn)</span>
        </div>
      </div>
      <div className="flex justify-center bg-slate-50 rounded-xl overflow-hidden border border-slate-100">
        <Image
          src={url}
          alt="AI Visualization"
          width={600}
          height={400}
          className="max-w-full h-auto object-contain"
          unoptimized
        />
      </div>
    </div>
  );
}

export function TrendChart({ data, onPin }: { data: ChartPoint[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Trend Chart">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Interactive Trend</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Bar</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <BarChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Bar dataKey="value" fill="#6366f1" radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

export function SegmentChart({ data, onPin }: { data: ChartPoint[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Segment Chart">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Interactive Segments</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Pie</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <PieChart>
          <Pie
            data={data}
            dataKey="value"
            nameKey="label"
            cx="50%"
            cy="50%"
            outerRadius={100}
            label={({ name, percent }) => `${name ?? ""} ${((percent ?? 0) * 100).toFixed(0)}%`}
          >
            {data.map((_, i) => (
              <Cell key={i} fill={COLORS[i % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Legend wrapperStyle={{ fontSize: '11px' }} />
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
}

export function LineTrendChart({ data, onPin }: { data: ChartPoint[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Line Trend">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Time Series Trend</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Line</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Line type="monotone" dataKey="value" stroke="#6366f1" strokeWidth={2} dot={{ fill: '#6366f1', r: 4 }} activeDot={{ r: 6 }} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

export function AreaTrendChart({ data, onPin }: { data: ChartPoint[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Area Trend">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Cumulative Trend</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Area</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <AreaChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Area type="monotone" dataKey="value" stroke="#6366f1" fill="#6366f1" fillOpacity={0.15} strokeWidth={2} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

type ScatterPoint = {
  label: string;
  x: number;
  y: number;
  z?: number;
};

export function ScatterTrendChart({ data, onPin }: { data: ScatterPoint[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Scatter Plot">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Correlation Explorer</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Scatter</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <ScatterChart margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="x" name="x" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis dataKey="y" name="y" tick={{ fontSize: 11, fill: '#64748b' }} />
          <ZAxis dataKey="z" range={[60, 400]} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
            cursor={{ strokeDasharray: '3 3' }}
          />
          <Scatter data={data} fill="#6366f1" fillOpacity={0.6}>
            {data.map((point, i) => (
              <Cell key={i} fill={COLORS[i % COLORS.length]} />
            ))}
          </Scatter>
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
}

type ComboPoint = {
  label: string;
  bars: number;
  line: number;
};

export function ComboChart({ data, onPin, barKey = "bars", lineKey = "line" }: {
  data: ComboPoint[];
  onPin?: () => void;
  barKey?: string;
  lineKey?: string;
}) {
  if (!data || data.length === 0) return null;
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Combo Chart">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Bar + Line Combo</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Recharts Composed</span>
        </div>
      </div>
      <ResponsiveContainer width="100%" height={280}>
        <ComposedChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Bar dataKey={barKey} fill="#6366f1" radius={[4, 4, 0, 0]} />
          <Line type="monotone" dataKey={lineKey} stroke="#ef4444" strokeWidth={2} dot={{ fill: '#ef4444', r: 4 }} />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

type HeatmapCell = {
  x: string;
  y: string;
  value: number;
};

// SmartAutoViz picks the right chart based on chartType prop
export function SmartAutoViz({ data, chartType, onPin, title }: {
  data: ChartPoint[];
  chartType?: string;
  onPin?: () => void;
  title?: string;
}) {
  if (!data || data.length === 0) return null;

  // Scatter needs two numeric columns — guard with a check
  if (chartType === 'scatter') {
    const hasNumericLabels = data.every((d) => !isNaN(Number(d.label)));
    if (!hasNumericLabels) {
      // Fall back to bar if labels aren't numeric
      return <TrendChart data={data} onPin={onPin} />;
    }
    return <ScatterTrendChart data={data.map((d) => ({ label: d.label, x: Number(d.label), y: d.value }))} onPin={onPin} />;
  }

  switch (chartType) {
    case 'line':
      return <LineTrendChart data={data} onPin={onPin} />;
    case 'pie':
      return <SegmentChart data={data} onPin={onPin} />;
    case 'area':
      return <AreaTrendChart data={data} onPin={onPin} />;
    case 'bar':
    default:
      return <TrendChart data={data} onPin={onPin} />;
  }
}

// Histogram chart for data profiling
export function HistogramChart({ column, buckets }: { column: string; buckets: { label: string; count: number }[] }) {
  if (!buckets || buckets.length === 0) return null;
  return (
    <div className="p-4 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-3">
      <strong className="text-xs font-semibold text-slate-700">Distribution: {column}</strong>
      <div className="space-y-1">
        {buckets.map((b, i) => {
          const maxCount = Math.max(...buckets.map((x) => x.count));
          const pct = maxCount > 0 ? (b.count / maxCount) * 100 : 0;
          return (
            <div key={i} className="flex items-center gap-2 text-[11px]">
              <span className="w-24 text-right text-slate-500 truncate shrink-0">{b.label}</span>
              <div className="flex-1 h-4 bg-slate-100 rounded-full overflow-hidden">
                <div className="h-full bg-indigo-500 rounded-full transition-all" style={{ width: `${pct}%` }} />
              </div>
              <span className="w-8 text-right text-slate-600 font-medium">{b.count}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// Collapsible explanation section for Explainable AI
export function ExplainSection({ explanations }: { explanations: { chart: string; columns: string; sql?: string; warning?: string; grouping?: string }[] }) {
  const [open, setOpen] = useState(false);
  if (!explanations || explanations.length === 0) return null;
  return (
    <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Info size={16} className="text-indigo-500" />
          How This Analysis Works
        </div>
        {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
      </button>
      {open && (
        <div className="px-5 pb-4 space-y-3">
          {explanations.map((exp, i) => (
            <div key={i} className="p-3 bg-slate-50 rounded-xl text-xs space-y-1">
              <div className="flex items-center gap-2">
                <span className="px-1.5 py-0.5 bg-indigo-100 text-indigo-700 rounded text-[10px] font-medium uppercase">{exp.chart}</span>
                <span className="text-slate-500">Columns: {exp.columns}</span>
              </div>
              {exp.grouping && exp.grouping !== 'none' && (
                <p className="text-slate-400">Grouped by: {exp.grouping}</p>
              )}
              {exp.sql && (
                <pre className="mt-1 p-2 bg-slate-800 text-slate-200 rounded-lg overflow-x-auto text-[10px] leading-relaxed">{exp.sql}</pre>
              )}
              {exp.warning && (
                <p className="text-amber-600 flex items-center gap-1">
                  <span>⚠</span> {exp.warning}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function HeatmapChart({ data, onPin }: { data: HeatmapCell[]; onPin?: () => void }) {
  if (!data || data.length === 0) return null;
  const maxVal = Math.max(...data.map((d) => d.value));
  const minVal = Math.min(...data.map((d) => d.value));
  const range = maxVal - minVal || 1;
  const xLabels = [...new Set(data.map((d) => d.x))];
  const yLabels = [...new Set(data.map((d) => d.y))];

  const getColor = (val: number) => {
    const intensity = (val - minVal) / range;
    const r = Math.round(99 + (99 - 99) * intensity);
    const g = Math.round(102 + (241 - 102) * (1 - intensity));
    const b = Math.round(241 + (241 - 241) * intensity);
    return `rgb(${r}, ${g}, ${b})`;
  };

  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4 relative group" data-export-plot="Heatmap">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Density Heatmap</strong>
        <div className="flex items-center gap-2">
          <button
            onClick={onPin}
            className="p-1 hover:bg-slate-100 rounded-md text-slate-400 hover:text-indigo-600 transition-colors"
            title="Pin to Dashboard"
          >
            <Pin size={14} />
          </button>
          <span className="text-xs text-slate-400">Grid</span>
        </div>
      </div>
      <div className="overflow-x-auto">
        <div className="grid gap-0.5" style={{ gridTemplateColumns: `auto repeat(${xLabels.length}, 1fr)` }}>
          <div className="text-[10px] text-slate-400 p-1" />
          {xLabels.map((xl) => (
            <div key={xl} className="text-[10px] text-slate-500 font-medium p-1 text-center truncate">{xl}</div>
          ))}
          {yLabels.map((yl) => (
            <React.Fragment key={yl}>
              <div className="text-[10px] text-slate-500 font-medium p-1 text-right truncate">{yl}</div>
              {xLabels.map((xl) => {
                const cell = data.find((d) => d.x === xl && d.y === yl);
                const val = cell?.value ?? 0;
                return (
                  <div
                    key={`${xl}-${yl}`}
                    className="p-2 text-center text-[10px] font-medium rounded transition-transform hover:scale-110"
                    style={{
                      backgroundColor: getColor(val),
                      color: val > (minVal + range * 0.6) ? 'white' : '#475569',
                    }}
                    title={`${yl} / ${xl}: ${val.toFixed(1)}`}
                  >
                    {val.toFixed(0)}
                  </div>
                );
              })}
            </React.Fragment>
          ))}
        </div>
      </div>
    </div>
  );
}
