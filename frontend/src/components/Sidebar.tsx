import React from 'react';
import {
  Search,
  LayoutDashboard,
  Database,
  Edit3,
  Share2,
  Plus,
  Check,
  GitMerge,
  Split,
  Terminal,
  CalendarClock,
  Layout,
  BarChart3,
  BookOpen,
  PanelRightClose,
  PanelRightOpen,
  X,
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
  isOpen: boolean;
  onClose: () => void;
  isPinned: boolean;
  onTogglePin: () => void;
};

const SOURCE_COLORS = ['bg-indigo-500', 'bg-blue-500', 'bg-orange-500', 'bg-emerald-500', 'bg-pink-500', 'bg-cyan-500'];

export function Sidebar({ datasets, selectedDatasetIds, onToggleDataset, onConnectSource, activeNav, onNavChange, isOpen, onClose, isPinned, onTogglePin }: SidebarProps) {
  const handleNavChange = (nav: string) => {
    onNavChange(nav);
    if (!isPinned) onClose();
  };

  return (
    <>
      {isOpen && !isPinned && (
        <div
          className="fixed inset-0 bg-black/20 backdrop-blur-sm z-30 transition-opacity duration-300"
          onClick={onClose}
        />
      )}

      <aside
        className={`fixed top-0 left-0 z-40 w-64 h-screen bg-slate-900 text-slate-300 flex flex-col border-r border-slate-800 transition-all duration-300 ease-out ${
          isOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-full shadow-none'
        }`}
      >
        <div className="p-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-lg flex items-center justify-center text-white font-bold shadow-md shadow-indigo-500/20">
              IP
            </div>
            <div className="select-none">
              <strong className="block text-white leading-none text-sm">InsightPilot</strong>
              <span className="text-[10px] text-slate-500 font-medium">Agentic BI</span>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={onTogglePin}
              className={`p-1.5 rounded-md transition-all ${isPinned ? 'text-indigo-400 hover:text-indigo-300' : 'text-slate-500 hover:text-slate-300 hover:bg-slate-800'}`}
              title={isPinned ? 'Unpin sidebar (auto-hide)' : 'Pin sidebar (keep visible)'}
            >
              {isPinned ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}
            </button>
            {!isPinned && (
              <button
                onClick={onClose}
                className="p-1.5 rounded-md text-slate-500 hover:text-slate-300 hover:bg-slate-800 transition-all"
                title="Close sidebar"
              >
                <X size={16} />
              </button>
            )}
          </div>
        </div>

        <nav className="flex-1 px-4 space-y-0.5 overflow-y-auto">
          <NavItem icon={<Search size={18} />} label="Explore" active={activeNav === 'explore'} onClick={() => handleNavChange('explore')} />
          <NavItem icon={<LayoutDashboard size={18} />} label="Dashboards" active={activeNav === 'dashboards'} onClick={() => handleNavChange('dashboards')} />
          <NavItem icon={<Database size={18} />} label="Data" active={activeNav === 'data'} onClick={() => handleNavChange('data')} />
          <NavItem icon={<BarChart3 size={18} />} label="Profiler" active={activeNav === 'profiler'} onClick={() => handleNavChange('profiler')} />
          <NavItem icon={<Edit3 size={18} />} label="Context" active={activeNav === 'context'} onClick={() => handleNavChange('context')} />
          <NavItem icon={<Share2 size={18} />} label="Share" active={activeNav === 'share'} onClick={() => handleNavChange('share')} />
          <div className="pt-4 pb-1 text-[10px] font-semibold text-slate-600 uppercase tracking-widest px-3">
            Advanced
          </div>
          <NavItem icon={<Split size={18} />} label="Schema" active={activeNav === 'schema'} onClick={() => handleNavChange('schema')} />
          <NavItem icon={<Terminal size={18} />} label="SQL Query" active={activeNav === 'query'} onClick={() => handleNavChange('query')} />
          <NavItem icon={<CalendarClock size={18} />} label="Reports" active={activeNav === 'reports'} onClick={() => handleNavChange('reports')} />
          <NavItem icon={<BookOpen size={18} />} label="Glossary" active={activeNav === 'glossary'} onClick={() => handleNavChange('glossary')} />
          <NavItem icon={<Layout size={18} />} label="Editor" active={activeNav === 'editor'} onClick={() => handleNavChange('editor')} />
        </nav>

        <div className="p-4 space-y-6 border-t border-slate-800/50">
          <div className="space-y-3">
            <div className="flex items-center justify-between text-[10px] font-semibold text-slate-500 uppercase tracking-widest px-1">
              <span>Data Sources</span>
              <button
                onClick={onConnectSource}
                className="hover:text-white transition-colors p-0.5 rounded hover:bg-slate-800"
                title="Connect a data source"
              >
                <Plus size={14} />
              </button>
            </div>
            <div className="space-y-0.5">
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

          <div className="p-3 rounded-xl bg-slate-800/30 border border-slate-700/50 space-y-2">
            <div className="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>Verified Context</span>
              <span className="px-1.5 py-0.5 rounded-md bg-emerald-500/15 text-emerald-400 text-[10px] font-semibold">Verified</span>
            </div>
            <ul className="text-[11px] space-y-1 text-slate-400 list-disc pl-3">
              <li>Revenue excludes failed payments.</li>
              <li>Enterprise is ARR &gt; $50k.</li>
            </ul>
          </div>
        </div>

        {isPinned && (
          <div className="absolute bottom-0 left-0 right-0 p-3 text-center">
            <button
              onClick={onTogglePin}
              className="text-[10px] text-slate-600 hover:text-slate-400 transition-colors"
            >
              Click to unpin &middot; auto-hide
            </button>
          </div>
        )}
      </aside>
    </>
  );
}

function NavItem({ icon, label, active = false, onClick }: NavItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-all ${
        active
          ? 'bg-gradient-to-r from-indigo-600 to-indigo-500 text-white shadow-sm shadow-indigo-500/20'
          : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/50'
      }`}
    >
      <span className={active ? 'text-white' : 'text-slate-500'}>{icon}</span>
      <span>{label}</span>
    </button>
  );
}

function SourceItem({ name, status, color, selected = false, onClick }: SourceItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-2 py-1.5 rounded-lg transition-all group ${
        selected ? 'bg-slate-800/80 ring-1 ring-indigo-500/30' : 'hover:bg-slate-800/50'
      }`}
    >
      <div className={`w-2 h-2 rounded-full ${color} ${selected ? 'ring-2 ring-offset-1 ring-offset-slate-900 ring-indigo-400/30' : ''}`} />
      <div className="text-left flex-1 min-w-0">
        <strong className="block text-xs text-slate-300 group-hover:text-white truncate">{name}</strong>
        <small className="text-[10px] text-slate-500">{status}</small>
      </div>
      {selected && <Check size={12} className="text-indigo-400 shrink-0" />}
    </button>
  );
}
