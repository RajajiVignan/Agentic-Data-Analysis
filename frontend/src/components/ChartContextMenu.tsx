
import { useEffect, useRef, useState, useCallback } from "react";
import {
  ArrowDown, BarChart3, LineChart, PieChart, AreaChart, ScatterChart,
  Download, Filter, Crosshair, X,
} from "lucide-react";

export type ContextMenuAction = "drilldown" | "changeType" | "filter" | "exportPng" | "exportJpeg" | "pin";

export type MenuItem = {
  id: ContextMenuAction;
  label: string;
  icon?: React.ReactNode;
  disabled?: boolean;
  children?: { id: string; label: string; icon?: React.ReactNode }[];
};

type Position = { x: number; y: number };

const CHART_TYPE_ITEMS = [
  { id: "bar", label: "Bar", icon: <BarChart3 size={14} /> },
  { id: "line", label: "Line", icon: <LineChart size={14} /> },
  { id: "pie", label: "Pie", icon: <PieChart size={14} /> },
  { id: "area", label: "Area", icon: <AreaChart size={14} /> },
  { id: "scatter", label: "Scatter", icon: <ScatterChart size={14} /> },
];

type ChartContextMenuProps = {
  chartTitle: string;
  chartType?: string;
  chartData?: unknown[];
  onAction: (action: ContextMenuAction, payload?: string) => void;
  onChangeChartType?: (type: string) => void;
  onExportPng?: () => void;
  onExportJpeg?: () => void;
  onDrillDown?: () => void;
  children: React.ReactNode;
};

export function ChartContextMenu({
  chartTitle,
  chartType,
  onAction,
  onChangeChartType,
  onExportPng,
  onExportJpeg,
  onDrillDown,
  children,
}: ChartContextMenuProps) {
  const [menuPos, setMenuPos] = useState<Position | null>(null);
  const [submenu, setSubmenu] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const closeMenu = useCallback(() => {
    setMenuPos(null);
    setSubmenu(null);
  }, []);

  useEffect(() => {
    if (!menuPos) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        closeMenu();
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeMenu();
    };
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [menuPos, closeMenu]);

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    let x = e.clientX - rect.left;
    let y = e.clientY - rect.top;
    const menuW = 200;
    const menuH = 220;
    if (x + menuW > rect.width) x = rect.width - menuW - 8;
    if (y + menuH > rect.height) y = rect.height - menuH - 8;
    if (x < 8) x = 8;
    if (y < 8) y = 8;
    setMenuPos({ x, y });
  };

  const handleItemClick = (action: ContextMenuAction, payload?: string) => {
    onAction(action, payload);
    if (action === "drilldown" && onDrillDown) onDrillDown();
    if (action === "exportPng" && onExportPng) onExportPng();
    if (action === "exportJpeg" && onExportJpeg) onExportJpeg();
    if (action === "changeType" && payload && onChangeChartType) onChangeChartType(payload);
    closeMenu();
  };

  const baseMenuItems: { id: string; label: string; icon: React.ReactNode; hasSub?: boolean; action?: ContextMenuAction }[] = [
    { id: "drilldown", label: "Drill Down", icon: <Crosshair size={14} />, action: "drilldown" },
    { id: "changeType", label: "Change Chart Type", icon: <BarChart3 size={14} />, hasSub: true },
    { id: "filter", label: "Filter by Selection", icon: <Filter size={14} />, action: "filter" },
    { id: "export", label: "Export", icon: <Download size={14} />, hasSub: true },
  ];

  return (
    <div
      ref={containerRef}
      className="relative"
      onContextMenu={handleContextMenu}
    >
      {children}

      {menuPos && (
        <div
          ref={menuRef}
          className="absolute z-50 min-w-[190px] bg-white rounded-xl border border-slate-200 shadow-xl py-1.5 animate-fade-in"
          style={{ left: menuPos.x, top: menuPos.y }}
        >
          {baseMenuItems.map((item) => (
            <div key={item.id} className="relative">
              {item.hasSub ? (
                <>
                  <button
                    className="w-full flex items-center gap-2.5 px-3 py-2 text-xs font-medium text-slate-600 hover:bg-indigo-50 hover:text-indigo-700 transition-colors"
                    onMouseEnter={() => setSubmenu(item.id)}
                    onClick={() => {
                      if (submenu === item.id) setSubmenu(null);
                      else setSubmenu(item.id);
                    }}
                  >
                    <span className="text-slate-400">{item.icon}</span>
                    <span className="flex-1 text-left">{item.label}</span>
                    <ArrowDown size={10} className="text-slate-300 -rotate-90" />
                  </button>
                  {submenu === item.id && (
                    <div
                      className="absolute left-full top-0 ml-1 min-w-[150px] bg-white rounded-xl border border-slate-200 shadow-lg py-1.5 animate-fade-in"
                    >
                      {item.id === "changeType" && CHART_TYPE_ITEMS.filter(t => t.id !== chartType).map((ct) => (
                        <button
                          key={ct.id}
                          className="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-indigo-50 hover:text-indigo-700 transition-colors"
                          onClick={() => handleItemClick("changeType", ct.id)}
                        >
                          <span className="text-slate-400">{ct.icon}</span>
                          {ct.label}
                        </button>
                      ))}
                      {item.id === "export" && (
                        <>
                          <button
                            className="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-indigo-50 hover:text-indigo-700 transition-colors"
                            onClick={() => handleItemClick("exportPng")}
                          >
                            <Download size={14} className="text-slate-400" />
                            PNG
                          </button>
                          <button
                            className="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-indigo-50 hover:text-indigo-700 transition-colors"
                            onClick={() => handleItemClick("exportJpeg")}
                          >
                            <Download size={14} className="text-slate-400" />
                            JPEG
                          </button>
                        </>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <button
                  className="w-full flex items-center gap-2.5 px-3 py-2 text-xs font-medium text-slate-600 hover:bg-indigo-50 hover:text-indigo-700 transition-colors"
                  onClick={() => item.action && handleItemClick(item.action)}
                >
                  <span className="text-slate-400">{item.icon}</span>
                  {item.label}
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
