import React, { useState, useEffect, useCallback, type ComponentType, lazy, Suspense } from "react";
import "react-grid-layout/css/styles.css";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const GridLayout = lazy(() => import("react-grid-layout")) as ComponentType<any>;
import {
  Layout as LayoutIcon,
  Plus,
  Minus,
  BarChart3,
  Trash2,
  Save,
  GripVertical,
  Type,
} from "lucide-react";
import {
  fetchDashboardLayouts,
  createDashboardLayout,
  saveDashboardLayout,
  deleteDashboardLayout,
  addTileToLayout,
  updateTileInLayout,
  removeTileFromLayout,
  fetchPinnedCharts,
} from "@/lib/api";
import type { DashboardLayout, DashboardTile, PinnedChart } from "@/lib/api";
import type { Layout } from "react-grid-layout";
import { MetricTile, TrendChart, SegmentChart, PythonPlot, LineTrendChart, AreaTrendChart } from "@/components/Charts";

type Props = {
  onRefreshCharts: () => void;
};

export function DashboardEditor({ onRefreshCharts }: Props) {
  const [layouts, setLayouts] = useState<DashboardLayout[]>([]);
  const [activeLayoutId, setActiveLayoutId] = useState<string | null>(null);
  const [pinnedCharts, setPinnedCharts] = useState<PinnedChart[]>([]);
  const [editing, setEditing] = useState(false);
  const [showAddMenu, setShowAddMenu] = useState(false);

  useEffect(() => {
    loadLayouts();
    loadCharts();
  }, []);

  async function loadLayouts() {
    try {
      const data = await fetchDashboardLayouts();
      setLayouts(data);
      if (data.length > 0 && !activeLayoutId) {
        setActiveLayoutId(data[0].id);
      }
    } catch {
      // ignore
    }
  }

  async function loadCharts() {
    try {
      setPinnedCharts(await fetchPinnedCharts());
    } catch {
      // ignore
    }
  }

  const activeLayout = layouts.find((l) => l.id === activeLayoutId);

  const handleCreateLayout = useCallback(async () => {
    const name = prompt("Layout name:");
    if (!name) return;
    try {
      const layout = await createDashboardLayout(name);
      setLayouts((prev) => [...prev, layout]);
      setActiveLayoutId(layout.id);
    } catch {
      // ignore
    }
  }, []);

  const handleSaveLayout = useCallback(async () => {
    if (!activeLayout) return;
    try {
      await saveDashboardLayout(activeLayout.id, activeLayout);
    } catch {
      // ignore
    }
  }, [activeLayout]);

  const handleDeleteLayout = useCallback(async (id: string) => {
    if (!confirm("Delete this layout?")) return;
    try {
      await deleteDashboardLayout(id);
      setLayouts((prev) => prev.filter((l) => l.id !== id));
      if (activeLayoutId === id) {
        setActiveLayoutId(null);
      }
    } catch {
      // ignore
    }
  }, [activeLayoutId]);

  const handleAddTile = useCallback(async (type: DashboardTile["type"]) => {
    if (!activeLayout) return;
    const tile: Partial<DashboardTile> = {
      type,
      w: type === "divider" ? 12 : type === "metric" ? 3 : 6,
      h: type === "divider" ? 1 : type === "text" ? 2 : 4,
      x: 0,
      y: Infinity,
      title: type === "text" ? "Text Block" : type === "chart" ? "Chart" : type === "metric" ? "Metric" : "",
      content: type === "text" ? "Add your content here..." : "",
      chartType: type === "chart" ? "bar" : undefined,
    };
    try {
      await addTileToLayout(activeLayout.id, tile);
      await loadLayouts();
    } catch {
      // ignore
    }
  }, [activeLayout]);

  const handleRemoveTile = useCallback(async (tileId: string) => {
    if (!activeLayout) return;
    try {
      await removeTileFromLayout(activeLayout.id, tileId);
      await loadLayouts();
    } catch {
      // ignore
    }
  }, [activeLayout]);

  const handleLayoutChange = useCallback((newLayout: Layout) => {
    if (!activeLayout || !editing) return;
    const updatedTiles = activeLayout.tiles.map((tile) => {
      const pos = newLayout.find((l) => l.i === tile.id);
      if (pos) {
        return { ...tile, x: pos.x, y: pos.y, w: pos.w, h: pos.h };
      }
      return tile;
    });
    const updated = { ...activeLayout, tiles: updatedTiles };
    setLayouts((prev) => prev.map((l) => (l.id === activeLayout.id ? updated : l)));
  }, [activeLayout, editing]);

  if (!activeLayout && layouts.length === 0) {
    return (
      <div className="text-center py-12 space-y-4">
        <LayoutIcon size={48} className="mx-auto text-slate-300" />
        <p className="text-sm text-slate-500">No dashboard layouts yet</p>
        <button
          onClick={handleCreateLayout}
          className="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700"
        >
          Create First Layout
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between bg-white rounded-2xl border border-slate-200 shadow-sm p-3">
        <div className="flex items-center gap-2">
          <select
            value={activeLayoutId ?? ""}
            onChange={(e) => setActiveLayoutId(e.target.value)}
            className="p-2 text-sm border border-slate-200 rounded-lg bg-white font-medium"
          >
            {layouts.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
          <button
            onClick={handleCreateLayout}
            className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600"
            title="New Layout"
          >
            <Plus size={16} />
          </button>
          {activeLayoutId && (
            <button
              onClick={() => handleDeleteLayout(activeLayoutId)}
              className="p-2 hover:bg-red-50 rounded-lg text-slate-400 hover:text-red-500"
              title="Delete Layout"
            >
              <Trash2 size={16} />
            </button>
          )}
        </div>
        <div className="flex items-center gap-2">
          {editing && (
            <>
              <div className="flex items-center gap-1 border-r border-slate-200 pr-2">
                <button
                  onClick={() => handleAddTile("chart")}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600"
                  title="Add Chart"
                >
                  <BarChart3 size={16} />
                </button>
                <button
                  onClick={() => handleAddTile("text")}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600"
                  title="Add Text"
                >
                  <Type size={16} />
                </button>
                <button
                  onClick={() => handleAddTile("metric")}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600"
                  title="Add Metric"
                >
                  <BarChart3 size={16} />
                </button>
                <button
                  onClick={() => handleAddTile("divider")}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600"
                  title="Add Divider"
                >
                  <Minus size={16} />
                </button>
              </div>
              <button
                onClick={handleSaveLayout}
                className="flex items-center gap-1 px-3 py-1.5 bg-indigo-600 text-white text-xs font-medium rounded-lg hover:bg-indigo-700"
              >
                <Save size={12} />
                Save
              </button>
            </>
          )}
          <button
            onClick={() => setEditing(!editing)}
            className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
              editing
                ? "bg-slate-800 text-white hover:bg-slate-700"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200"
            }`}
          >
            {editing ? "Done" : "Edit Layout"}
          </button>
        </div>
      </div>

      {/* Grid */}
      {activeLayout && (
        <div className="relative">
          {editing && (
            <Suspense fallback={<div className="h-96 bg-slate-100 dark:bg-slate-800 rounded-lg animate-pulse" />}>
            <GridLayout
              className="layout"
              layout={activeLayout.tiles.map((t) => ({
                i: t.id,
                x: t.x,
                y: t.y,
                w: t.w,
                h: t.h,
                static: false,
              }))}
              cols={12}
              rowHeight={80}
              width={1200}
              onLayoutChange={handleLayoutChange}
              draggableHandle=".drag-handle"
              isResizable
            >
              {activeLayout.tiles.map((tile) => (
                <div
                  key={tile.id}
                  className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden group"
                >
                  <div className="flex items-center justify-between px-3 py-1.5 bg-slate-50 border-b border-slate-100">
                    <div className="flex items-center gap-1.5 drag-handle cursor-grab">
                      <GripVertical size={12} className="text-slate-300" />
                      <span className="text-[10px] font-medium text-slate-400 uppercase">
                        {tile.type}
                      </span>
                    </div>
                    <button
                      onClick={() => handleRemoveTile(tile.id)}
                      className="p-0.5 hover:bg-red-50 rounded text-slate-300 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      <Trash2 size={10} />
                    </button>
                  </div>
                  <div className="p-3 h-full overflow-hidden">
                    {tile.type === "text" && (
                      <div
                        contentEditable
                        suppressContentEditableWarning
                        className="text-sm text-slate-700 outline-none focus:ring-1 focus:ring-indigo-300 rounded p-1"
                        dangerouslySetInnerHTML={{ __html: tile.content ?? "" }}
                      />
                    )}
                    {tile.type === "divider" && (
                      <hr className="border-slate-200 my-2" />
                    )}
                    {tile.type === "metric" && (
                      <div className="space-y-1">
                        <span className="text-[10px] font-medium text-slate-400 uppercase">
                          {tile.title || "Metric"}
                        </span>
                        <div className="text-2xl font-bold text-slate-900">
                          {tile.content || "—"}
                        </div>
                      </div>
                    )}
                    {tile.type === "chart" && (
                      <div className="flex items-center justify-center h-full text-slate-400 text-xs">
                        Chart container — drop a pinned chart ID: {tile.pinnedId ?? "none"}
                      </div>
                    )}
                    {tile.type === "image" && tile.imageUrl && (
                      <img
                        src={tile.imageUrl}
                        alt={tile.title ?? ""}
                        className="w-full h-full object-contain"
                      />
                    )}
                  </div>
                </div>
              ))}
            </GridLayout>
            </Suspense>
          )}

          {!editing && (
            <div className="space-y-3">
              {activeLayout.tiles.length === 0 && (
                <div className="p-12 text-center text-sm text-slate-400 bg-white rounded-2xl border border-slate-200">
                  Empty layout. Switch to Edit mode to add tiles.
                </div>
              )}
              {activeLayout.tiles.map((tile) => (
                <div
                  key={tile.id}
                  className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden"
                  style={{
                    gridColumn: `span ${tile.w}`,
                  }}
                >
                  {tile.type === "text" && (
                    <div className="p-4 text-sm text-slate-700">
                      {tile.title && (
                        <div className="font-semibold text-slate-800 mb-1">{tile.title}</div>
                      )}
                      <div dangerouslySetInnerHTML={{ __html: tile.content ?? "" }} />
                    </div>
                  )}
                  {tile.type === "divider" && (
                    <hr className="border-slate-200 mx-4" />
                  )}
                  {tile.type === "metric" && (
                    <div className="p-4">
                      <MetricTile
                        label={tile.title ?? "Metric"}
                        value={tile.content ?? "—"}
                        change=""
                      />
                    </div>
                  )}
                  {tile.type === "chart" && tile.pinnedId && (
                    <div className="p-4">
                      {renderPinnedChart(tile.pinnedId, pinnedCharts)}
                    </div>
                  )}
                  {tile.type === "image" && tile.imageUrl && (
                    <img
                      src={tile.imageUrl}
                      alt={tile.title ?? ""}
                      className="w-full h-auto"
                    />
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function renderPinnedChart(pinnedId: string, charts: PinnedChart[]) {
  const chart = charts.find((c) => c.id === pinnedId);
  if (!chart) return <div className="text-xs text-slate-400 p-4">Chart not found</div>;

  switch (chart.chart_type) {
    case "kpi": {
      const data = chart.data as { value?: string; change?: string } | undefined;
      return (
        <MetricTile
          label={chart.label}
          value={data?.value ?? "—"}
          change={data?.change ?? ""}
        />
      );
    }
    case "trend": {
      const trendData = (chart.data as Array<{ label: string; value: number }>) ?? [];
      return <TrendChart data={trendData} />;
    }
    case "segment": {
      const segData = (chart.data as Array<{ label: string; value: number }>) ?? [];
      return <SegmentChart data={segData} />;
    }
    case "python_plot": {
      const url = chart.url ?? (chart.data as { url?: string })?.url;
      return url ? <PythonPlot url={url} /> : null;
    }
    default:
      return null;
  }
}
