import React from 'react';
import Image from 'next/image';
import { Pin } from 'lucide-react';

type KpiProps = {
  label: string;
  value: string;
  change: string;
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
