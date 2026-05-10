import React from 'react';
import { 
  Search, 
  LayoutDashboard, 
  Database, 
  Edit3, 
  Share2, 
} from 'lucide-react';

type NavItemProps = {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
};

type SourceItemProps = {
  name: string;
  status: string;
  color: string;
};

export function Sidebar() {
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
        <NavItem icon={<Search size={18} />} label="Explore" active />
        <NavItem icon={<LayoutDashboard size={18} />} label="Dashboards" />
        <NavItem icon={<Database size={18} />} label="Data" />
        <NavItem icon={<Edit3 size={18} />} label="Context" />
        <NavItem icon={<Share2 size={18} />} label="Share" />
      </nav>

      <div className="p-4 space-y-6 border-t border-slate-800">
        <div className="space-y-3">
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 uppercase tracking-wider">
            <span>Data Sources</span>
            <button className="hover:text-white transition-colors">+</button>
          </div>
          <div className="space-y-2">
            <SourceItem name="Stripe revenue" status="Synced 7m ago" color="bg-indigo-500" />
            <SourceItem name="Postgres warehouse" status="34 tables" color="bg-blue-500" />
            <SourceItem name="HubSpot CRM" status="Pipeline" color="bg-orange-500" />
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

function NavItem({ icon, label, active = false }: NavItemProps) {
  return (
    <button className={`w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-all ${active ? 'bg-indigo-600 text-white shadow-sm' : 'hover:bg-slate-800 hover:text-slate-100'}`}>
      {icon}
      <span>{label}</span>
    </button>
  );
}

function SourceItem({ name, status, color }: SourceItemProps) {
  return (
    <button className="w-full flex items-center gap-3 px-2 py-1.5 rounded-md hover:bg-slate-800 transition-colors group">
      <div className={`w-2 h-2 rounded-full ${color}`} />
      <div className="text-left">
        <strong className="block text-xs text-slate-200 group-hover:text-white">{name}</strong>
        <small className="text-[10px] text-slate-500">{status}</small>
      </div>
    </button>
  );
}
