import React, { useState, useEffect, useRef, useCallback, useMemo } from "react";
import {
  Search,
  LayoutDashboard,
  Database,
  Edit3,
  Share2,
  GitMerge,
  Terminal,
  CalendarClock,
  Layout,
  BarChart3,
  Sun,
  Moon,
  Plus,
  FileDown,
} from "lucide-react";

type CommandItem = {
  id: string;
  label: string;
  icon: React.ReactNode;
  category: string;
  action: () => void;
};

type CommandPaletteProps = {
  onNavigate: (tab: string) => void;
  onNewAnalysis: () => void;
  onToggleTheme: () => void;
  onExportCsv: () => void;
  onExportPdf: () => void;
  theme: "light" | "dark";
  hasData: boolean;
  hasResults: boolean;
};

export function CommandPalette({
  onNavigate,
  onNewAnalysis,
  onToggleTheme,
  onExportCsv,
  onExportPdf,
  theme,
  hasData,
  hasResults,
}: CommandPaletteProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setQuery("");
        setSelectedIndex(0);
        setTimeout(() => inputRef.current?.focus(), 50);
        setOpen(true);
      }
      if (e.key === "Escape") {
        setOpen(false);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const items: CommandItem[] = [
    { id: "explore", label: "Explore", icon: <Search size={16} />, category: "Navigation", action: () => onNavigate("explore") },
    { id: "dashboards", label: "Dashboards", icon: <LayoutDashboard size={16} />, category: "Navigation", action: () => onNavigate("dashboards") },
    { id: "data", label: "Data Sources", icon: <Database size={16} />, category: "Navigation", action: () => onNavigate("data") },
    { id: "profiler", label: "Data Profiler", icon: <BarChart3 size={16} />, category: "Navigation", action: () => onNavigate("profiler") },
    { id: "context", label: "Verified Context", icon: <Edit3 size={16} />, category: "Navigation", action: () => onNavigate("context") },
    { id: "share", label: "Share & Export", icon: <Share2 size={16} />, category: "Navigation", action: () => onNavigate("share") },
    { id: "schema", label: "Schema Designer", icon: <GitMerge size={16} />, category: "Advanced", action: () => onNavigate("schema") },
    { id: "query", label: "SQL Query", icon: <Terminal size={16} />, category: "Advanced", action: () => onNavigate("query") },
    { id: "reports", label: "Scheduled Reports", icon: <CalendarClock size={16} />, category: "Advanced", action: () => onNavigate("reports") },
    { id: "editor", label: "Dashboard Editor", icon: <Layout size={16} />, category: "Advanced", action: () => onNavigate("editor") },
    { id: "new-analysis", label: "New Analysis", icon: <Plus size={16} />, category: "Actions", action: () => { onNewAnalysis(); setOpen(false); } },
    { id: "toggle-theme", label: `Switch to ${theme === "light" ? "Dark" : "Light"} Mode`, icon: theme === "light" ? <Moon size={16} /> : <Sun size={16} />, category: "Actions", action: () => { onToggleTheme(); setOpen(false); } },
    ...(hasData ? [{ id: "export-csv", label: "Export Cleaned CSV", icon: <FileDown size={16} />, category: "Actions", action: () => { onExportCsv(); setOpen(false); } }] : []),
    ...(hasResults ? [{ id: "export-pdf", label: "Export PDF Report", icon: <FileDown size={16} />, category: "Actions", action: () => { onExportPdf(); setOpen(false); } }] : []),
  ];

  const results = useMemo(() => {
    if (!query.trim()) return items;
    const q = query.toLowerCase();
    return items.filter(i => i.label.toLowerCase().includes(q) || i.category.toLowerCase().includes(q));
  }, [query, items]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex(prev => Math.min(prev + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex(prev => Math.max(prev - 1, 0));
    } else if (e.key === "Enter" && results[selectedIndex]) {
      e.preventDefault();
      results[selectedIndex].action();
      setOpen(false);
    }
  }, [results, selectedIndex]);

  if (!open) return null;

  const categories = [...new Set(results.map(i => i.category))];

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]" onClick={() => setOpen(false)}>
      <div className="fixed inset-0 bg-black/40 backdrop-blur-sm" />
      <div
        className="relative w-full max-w-lg bg-white rounded-2xl border border-slate-200 shadow-2xl overflow-hidden animate-slide-up"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 px-4 py-3 border-b border-slate-100">
          <Search size={18} className="text-slate-400 shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={e => { setQuery(e.target.value); setSelectedIndex(0); }}
            onKeyDown={handleKeyDown}
            placeholder="Search pages and actions..."
            className="flex-1 outline-none text-sm text-slate-700 placeholder:text-slate-400 bg-transparent"
          />
          <kbd className="text-[10px] text-slate-400 px-1.5 py-0.5 rounded border border-slate-200 bg-slate-50 font-mono">ESC</kbd>
        </div>
        <div ref={listRef} className="max-h-80 overflow-y-auto p-2 space-y-1">
          {results.length === 0 && (
            <p className="text-sm text-slate-400 text-center py-8">No results for &quot;{query}&quot;</p>
          )}
          {categories.map(cat => (
            <div key={cat}>
              <p className="text-[10px] font-semibold text-slate-400 uppercase tracking-widest px-2 pt-2 pb-1">{cat}</p>
              {results.filter(i => i.category === cat).map((item) => {
                const globalIdx = results.indexOf(item);
                return (
                  <button
                    key={item.id}
                    onClick={() => { item.action(); setOpen(false); }}
                    className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-all ${
                      globalIdx === selectedIndex
                        ? "bg-indigo-50 text-indigo-700"
                        : "text-slate-600 hover:bg-slate-50"
                    }`}
                  >
                    <span className={`shrink-0 ${globalIdx === selectedIndex ? "text-indigo-500" : "text-slate-400"}`}>
                      {item.icon}
                    </span>
                    <span className="font-medium">{item.label}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </div>
        <div className="flex items-center gap-4 px-4 py-2 border-t border-slate-100 text-[10px] text-slate-400">
          <span><kbd className="px-1 py-0.5 rounded border border-slate-200 bg-slate-50 font-mono">↑↓</kbd> navigate</span>
          <span><kbd className="px-1 py-0.5 rounded border border-slate-200 bg-slate-50 font-mono">↵</kbd> select</span>
          <span><kbd className="px-1 py-0.5 rounded border border-slate-200 bg-slate-50 font-mono">⌘K</kbd> toggle</span>
        </div>
      </div>
    </div>
  );
}
