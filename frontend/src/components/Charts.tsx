import React from 'react';
import Image from 'next/image';
import { Pin } from 'lucide-react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from 'recharts';

type KpiProps = {
  label: string;
  value: string;
  change: string;
};

const COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6'];

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
