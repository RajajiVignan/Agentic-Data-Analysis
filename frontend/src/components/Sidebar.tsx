import React, { useState } from 'react';
import {
  Search,
  LayoutDashboard,
  Database,
  Edit3,
  Share2,
  Plus,
  Check,
  GitMerge,
  Terminal,
  CalendarClock,
  Layout,
  BarChart3,
} from 'lucide-react';

type Dataset = {
  id: string;
  filename: string;
};

type NavItemProps = {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  onClick?: () => void;
};

type SourceItemProps = {
  name: string;
  status: string;
  color: string;
  selected?: boolean;
  onClick?: () => void;
};

type SidebarProps = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
  onToggleDataset: (id: string) => void;
  onConnectSource: () => void;
  activeNav: string;
  onNavChange: (nav: string) => void;
};

const SOURCE_COLORS = ['bg-indigo-500', 'bg-blue-500', 'bg-orange-500', 'bg-emerald-500', 'bg-pink-500', 'bg-cyan-500'];

export function Sidebar({ datasets, selectedDatasetIds, onToggleDataset, onConnectSource, activeNav, onNavChange }: SidebarProps) {
  return (
    <aside className="w-64 h-screen bg-slate-900 text-slate-300 flex flex-col border-r border-slate-800">
      <div className="p-6 flex items-center gap-3">
        <div className="w-8 h-8 bg-indigo-600 rounded-lg flex items-center justify-center text-white font-bold">IP</div>
        <div>
          <strong className="block text-white leading-none">InsightPilot</strong>
          <span className="text-xs text-slate-500">Agentic BI</span>
        </div>
      </div>

      <nav className="flex-1 px-4 space-y-1">
        <NavItem icon={<Search size={18} />} label="Explore" active={activeNav === 'explore'} onClick={() => onNavChange('explore')} />
        <NavItem icon={<LayoutDashboard size={18} />} label="Dashboards" active={activeNav === 'dashboards'} onClick={() => onNavChange('dashboards')} />
        <NavItem icon={<Database size={18} />} label="Data" active={activeNav === 'data'} onClick={() => onNavChange('data')} />
        <NavItem icon={<BarChart3 size={18} />} label="Profiler" active={activeNav === 'profiler'} onClick={() => onNavChange('profiler')} />
        <NavItem icon={<Edit3 size={18} />} label="Context" active={activeNav === 'context'} onClick={() => onNavChange('context')} />
        <NavItem icon={<Share2 size={18} />} label="Share" active={activeNav === 'share'} onClick={() => onNavChange('share')} />
        <div className="pt-3 pb-1 text-[10px] font-medium text-slate-600 uppercase tracking-wider px-3">
          Advanced
        </div>
        <NavItem icon={<GitMerge size={18} />} label="Joins" active={activeNav === 'joins'} onClick={() => onNavChange('joins')} />
        <NavItem icon={<Terminal size={18} />} label="SQL Query" active={activeNav === 'query'} onClick={() => onNavChange('query')} />
        <NavItem icon={<CalendarClock size={18} />} label="Reports" active={activeNav === 'reports'} onClick={() => onNavChange('reports')} />
        <NavItem icon={<Layout size={18} />} label="Editor" active={activeNav === 'editor'} onClick={() => onNavChange('editor')} />
      </nav>

      <div className="p-4 space-y-6 border-t border-slate-800">
        <div className="space-y-3">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 uppercase tracking-wider">
            <span>Data Sources</span>
            <button
              onClick={onConnectSource}
              className="hover:text-white transition-colors p-0.5 rounded hover:bg-slate-800"
              title="Connect a data source"
            >
              <Plus size={14} />
            </button>
          </div>
          <div className="space-y-1">
            {datasets.length === 0 ? (
              <p className="text-[11px] text-slate-500 italic px-2">No datasets loaded. Upload a file or connect a source.</p>
            ) : (
              datasets.map((ds, i) => (
                <SourceItem
                  key={ds.id}
                  name={ds.filename}
                  status={selectedDatasetIds.includes(ds.id) ? 'Selected' : 'Ready'}
                  color={SOURCE_COLORS[i % SOURCE_COLORS.length]}
                  selected={selectedDatasetIds.includes(ds.id)}
                  onClick={() => onToggleDataset(ds.id)}
                />
              ))
            )}
          </div>
        </div>

        <div className="p-3 rounded-xl bg-slate-800/50 border border-slate-700 space-y-2">
          <div className="flex items-center justify-between text-xs font-semibold text-slate-400">
            <span>Verified Context</span>
            <span className="px-1.5 py-0.5 rounded-md bg-emerald-500/20 text-emerald-400 text-[10px]">Verified</span>
          </div>
          <ul className="text-[11px] space-y-1 text-slate-400 list-disc pl-3">
            <li>Revenue excludes failed payments.</li>
            <li>Enterprise is ARR &gt; $50k.</li>
          </ul>
        </div>
      </div>
    </aside>
  );
}

function NavItem({ icon, label, active = false, onClick }: NavItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-all ${active ? 'bg-indigo-600 text-white shadow-sm' : 'hover:bg-slate-800 hover:text-slate-100'}`}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}

function SourceItem({ name, status, color, selected = false, onClick }: SourceItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-2 py-1.5 rounded-md transition-colors group ${selected ? 'bg-slate-800 ring-1 ring-indigo-500/50' : 'hover:bg-slate-800'}`}
    >
      <div className={`w-2 h-2 rounded-full ${color}`} />
      <div className="text-left flex-1 min-w-0">
        <strong className="block text-xs text-slate-200 group-hover:text-white truncate">{name}</strong>
        <small className="text-[10px] text-slate-500">{status}</small>
      </div>
      {selected && <Check size={12} className="text-indigo-400 shrink-0" />}
    </button>
  );
}
