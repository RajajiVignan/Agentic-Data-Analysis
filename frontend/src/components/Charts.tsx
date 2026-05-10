import React from 'react';
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
  Cell
} from 'recharts';

type KpiProps = {
  label: string;
  value: string;
  change: string;
};

type ChartPoint = {
  label: string;
  value: number;
};

type TrendChartProps = {
  data: ChartPoint[];
};

type SegmentChartProps = {
  data: ChartPoint[];
  recommendations: string[];
};

export function MetricTile({ label, value, change }: KpiProps) {
  return (
    <div className="p-4 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-1">
      <span className="text-xs font-medium text-slate-500">{label}</span>
      <strong className="block text-2xl font-bold text-slate-900">{value}</strong>
      <small className={`text-xs ${change.includes('+') ? 'text-emerald-600' : 'text-slate-500'}`}>
        {change}
      </small>
    </div>
  );
}

export function TrendChart({ data }: TrendChartProps) {
  return (
    <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4">
      <div className="flex items-center justify-between">
        <strong className="text-sm font-semibold text-slate-800">Revenue Trend</strong>
        <span className="text-xs text-slate-400">Last 8 periods</span>
      </div>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
            <XAxis dataKey="label" fontSize={12} tickLine={false} axisLine={false} tick={{fill: '#94a3b8'}} />
            <YAxis hide />
            <Tooltip 
              contentStyle={{ borderRadius: '12px', border: 'none', boxShadow: '0 4px 12px rgba(0,0,0,0.1)' }}
              cursor={{ fill: '#f8fafc' }}
            />
            <Bar dataKey="value" fill="#6366f1" radius={[4, 4, 0, 0]} barSize={32} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export function SegmentChart({ data, recommendations }: SegmentChartProps) {
  const COLORS = ['#6366f1', '#8b5cf6', '#ec4899', '#f97316', '#ef4444'];

  return (
    <div className="grid grid-cols-2 gap-6">
      <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4">
        <strong className="text-sm font-semibold text-slate-800">Segment Mix</strong>
        <div className="h-64 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={data}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={80}
                paddingAngle={5}
                dataKey="value"
              >
                {data.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>
        <div className="flex flex-wrap gap-3 justify-center">
          {data.map((entry, index) => (
            <div key={entry.label} className="flex items-center gap-1.5">
              <div className="w-2 h-2 rounded-full" style={{ backgroundColor: COLORS[index % COLORS.length] }} />
              <span className="text-[10px] text-slate-500 font-medium">{entry.label}</span>
            </div>
          ))}
        </div>
      </div>
      <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4">
        <strong className="text-sm font-semibold text-slate-800">Agent Recommendations</strong>
        <div className="space-y-3">
          {recommendations.map((rec, i) => (
            <div key={i} className="p-3 rounded-xl bg-slate-50 border border-slate-100 text-xs text-slate-600 leading-relaxed">
              {rec}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
