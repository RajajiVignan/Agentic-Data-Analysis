import { useState, useEffect, useMemo } from "react";
import {
  BarChart3,
  LineChart,
  AreaChart,
  PieChart,
  ScatterChart,
  Hash,
  Play,
  Pin,
  Table2,
  CalendarDays,
  GitBranch,
  PieChart as SunburstIcon,
} from "lucide-react";
import type { Dataset, SemanticField, ExploreResult } from "@/lib/api";
import {
  fetchDatasetFields,
  explore,
  pinChart,
} from "@/lib/api";
import {
  TrendChart,
  SegmentChart,
  LineTrendChart,
  AreaTrendChart,
  ScatterTrendChart,
  MetricTile,
} from "@/components/Charts";
import {
  PivotTable,
  CalendarHeatmap,
  SankeyDiagram,
  SunburstChart,
} from "@/components/VizComponents";

type VizType = "bar" | "line" | "area" | "pie" | "scatter" | "kpi" | "pivottable" | "heatmap" | "sankey" | "sunburst";

const VIZ_OPTIONS: { type: VizType; label: string; icon: React.ReactNode }[] = [
  { type: "bar", label: "Bar", icon: <BarChart3 size={16} /> },
  { type: "line", label: "Line", icon: <LineChart size={16} /> },
  { type: "area", label: "Area", icon: <AreaChart size={16} /> },
  { type: "pie", label: "Pie", icon: <PieChart size={16} /> },
  { type: "scatter", label: "Scatter", icon: <ScatterChart size={16} /> },
  { type: "kpi", label: "KPI", icon: <Hash size={16} /> },
  { type: "pivottable", label: "Pivot", icon: <Table2 size={16} /> },
  { type: "heatmap", label: "Heatmap", icon: <CalendarDays size={16} /> },
  { type: "sankey", label: "Sankey", icon: <GitBranch size={16} /> },
  { type: "sunburst", label: "Sunburst", icon: <SunburstIcon size={16} /> },
];

// viz types that require a second dimension (column / target / inner ring)
const NEEDS_DIM2: VizType[] = ["pivottable", "sankey", "sunburst"];

export function ExploreView({
  datasets,
  selectedDatasetId,
  onToast,
}: {
  datasets: Dataset[];
  selectedDatasetId: string;
  onToast?: (msg: string, kind?: "success" | "error") => void;
}) {
  const [datasetId, setDatasetId] = useState(selectedDatasetId);
  const [vizType, setVizType] = useState<VizType>("bar");
  const [fields, setFields] = useState<SemanticField[]>([]);

  const [dimType, setDimType] = useState<"column" | "field">("column");
  const [dimCol, setDimCol] = useState("");
  const [dimField, setDimField] = useState("");

  const [dim2Type, setDim2Type] = useState<"column" | "field">("column");
  const [dim2Col, setDim2Col] = useState("");
  const [dim2Field, setDim2Field] = useState("");

  const [metricType, setMetricType] = useState<"column" | "field">("column");
  const [metricCol, setMetricCol] = useState("");
  const [metricAgg, setMetricAgg] = useState<"sum" | "avg" | "min" | "max" | "count">("sum");
  const [metricField, setMetricField] = useState("");

  const [xCol, setXCol] = useState("");
  const [yCol, setYCol] = useState("");

  const [sort, setSort] = useState<"desc" | "asc">("desc");
  const [limit, setLimit] = useState(20);

  const [result, setResult] = useState<ExploreResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const dataset = datasets.find((d) => d.id === datasetId);
  const cols = dataset?.profile?.columns ?? [];
  const nums = cols.filter((c) => c.type === "number");
  const cats = cols.filter((c) => c.type === "text" || c.type === "date" || c.type === "number");
  const computedMetrics = fields.filter((f) => f.kind === "metric");
  const computedDims = fields.filter((f) => f.kind === "dimension");

  const defaultDim = useMemo(() => cats[0]?.name ?? "", [cats]);
  const defaultMetric = useMemo(() => nums[0]?.name ?? "", [nums]);

  useEffect(() => {
    if (datasetId) fetchDatasetFields(datasetId).then(setFields).catch(() => setFields([]));
  }, [datasetId]);

  async function run() {
    if (!datasetId) return;
    if (vizType !== "scatter" && !metricCol && metricType === "column") {
      onToast?.("Select a metric column", "error");
      return;
    }
    setLoading(true);
    try {
      const metric =
        metricType === "field"
          ? { type: "field" as const, id: metricField }
          : { type: "column" as const, name: metricCol || defaultMetric, agg: metricAgg };
      const dimension =
        dimType === "field"
          ? { type: "field" as const, id: dimField }
          : { type: "column" as const, name: dimCol || defaultDim };

      const dimension2 =
        dim2Type === "field"
          ? { type: "field" as const, id: dim2Field }
          : { type: "column" as const, name: dim2Col };

      const res = await explore({
        datasetId,
        vizType,
        metric,
        dimension,
        sort,
        limit,
        dimension2: NEEDS_DIM2.includes(vizType) ? dimension2 : undefined,
        xColumn: vizType === "scatter" ? xCol : undefined,
        yColumn: vizType === "scatter" ? yCol : undefined,
      });
      setResult(res);
    } catch (e) {
      onToast?.(String(e), "error");
    } finally {
      setLoading(false);
    }
  }

  // Auto-run when configuration changes
  useEffect(() => {
    if (!datasetId) return;
    if (vizType === "scatter") {
      if (xCol && yCol) run();
    } else if (metricCol || metricType === "field") {
      run();
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [datasetId, vizType, dimCol, dimField, dimType, dim2Col, dim2Field, dim2Type, metricCol, metricAgg, metricField, metricType, xCol, yCol, sort, limit]);

  async function handleSave() {
    if (!result) return;
    setSaving(true);
    try {
      const metricLabel = metricType === "field"
        ? computedMetrics.find((f) => f.id === metricField)?.name ?? "metric"
        : metricCol || defaultMetric;
      const chartType =
        vizType === "pie" ? "segment" : vizType === "kpi" ? "kpi" : "trend";
      const data =
        vizType === "kpi" && result.kpis && result.kpis.length > 0
          ? { value: result.kpis[0].value, change: result.kpis[0].label }
          : vizType === "pivottable" || vizType === "heatmap" || vizType === "sankey" || vizType === "sunburst"
          ? result
          : result.points;
      await pinChart({
        chart_type: chartType,
        label: `${metricLabel} · ${vizType}`,
        data,
      });
      onToast?.("Chart pinned to dashboard", "success");
    } catch (e) {
      onToast?.(String(e), "error");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-6">
      {/* Controls */}
      <div className="bg-white rounded-2xl p-5 border border-slate-200 space-y-4 h-fit">
        <div>
          <label className="text-xs font-medium text-slate-500 uppercase">Dataset</label>
          <select
            value={datasetId}
            onChange={(e) => setDatasetId(e.target.value)}
            className="w-full mt-1 px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white"
          >
            {datasets.map((d) => (
              <option key={d.id} value={d.id}>{d.filename}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="text-xs font-medium text-slate-500 uppercase">Chart Type</label>
          <div className="grid grid-cols-3 gap-2 mt-1">
            {VIZ_OPTIONS.map((v) => (
              <button
                key={v.type}
                onClick={() => setVizType(v.type)}
                className={`flex flex-col items-center gap-1 py-2 rounded-lg border text-xs transition-all ${vizType === v.type ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-slate-200 text-slate-500 hover:border-slate-300"}`}
              >
                {v.icon}
                {v.label}
              </button>
            ))}
          </div>
        </div>

        {vizType !== "scatter" && vizType !== "kpi" && (
          <DimensionPicker
            dimType={dimType} setDimType={setDimType}
            dimCol={dimCol} setDimCol={setDimCol} defaultDim={defaultDim}
            dimField={dimField} setDimField={setDimField}
            cols={cats} fields={computedDims}
          />
        )}

        {NEEDS_DIM2.includes(vizType) && (
          <DimensionPicker
            dimType={dim2Type} setDimType={setDim2Type}
            dimCol={dim2Col} setDimCol={setDim2Col} defaultDim={defaultDim}
            dimField={dim2Field} setDimField={setDim2Field}
            cols={cats} fields={computedDims}
            label={vizType === "sankey" ? "Target" : vizType === "sunburst" ? "Inner ring" : "Column"}
          />
        )}

        {vizType === "scatter" ? (
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-xs font-medium text-slate-500 uppercase">X</label>
              <select value={xCol} onChange={(e) => setXCol(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                <option value="">column…</option>
                {nums.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-slate-500 uppercase">Y</label>
              <select value={yCol} onChange={(e) => setYCol(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                <option value="">column…</option>
                {nums.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
              </select>
            </div>
          </div>
        ) : (
          <MetricPicker
            metricType={metricType} setMetricType={setMetricType}
            metricCol={metricCol} setMetricCol={setMetricCol} defaultMetric={defaultMetric}
            metricAgg={metricAgg} setMetricAgg={setMetricAgg}
            metricField={metricField} setMetricField={setMetricField}
            cols={nums} fields={computedMetrics}
            hideAgg={vizType === "kpi"}
          />
        )}

        {vizType !== "kpi" && vizType !== "scatter" && (
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-xs font-medium text-slate-500 uppercase">Sort</label>
              <select value={sort} onChange={(e) => setSort(e.target.value as "asc" | "desc")} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                <option value="desc">Highest first</option>
                <option value="asc">Lowest first</option>
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-slate-500 uppercase">Top N</label>
              <input type="number" min={1} max={200} value={limit} onChange={(e) => setLimit(Number(e.target.value))} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm" />
            </div>
          </div>
        )}

        <div className="flex gap-2">
          <button onClick={run} disabled={loading} className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
            <Play size={14} /> {loading ? "Running…" : "Run"}
          </button>
          <button onClick={handleSave} disabled={saving || !result} className="inline-flex items-center justify-center gap-2 px-4 py-2 rounded-lg border border-slate-300 text-slate-600 text-sm font-medium hover:bg-slate-50 disabled:opacity-50" title="Pin to dashboard">
            <Pin size={14} /> Save
          </button>
        </div>
      </div>

      {/* Preview */}
      <div className="bg-white rounded-2xl p-5 border border-slate-200 min-h-[400px]">
        {!result && (
          <div className="h-full flex items-center justify-center text-slate-400 text-sm">
            Configure the chart and it will preview here.
          </div>
        )}
        {result && vizType === "kpi" && result.kpis && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {result.kpis.map((k, i) => (
              <MetricTile key={i} label={k.label} value={k.value.toFixed(2)} change={k.change} />
            ))}
          </div>
        )}
        {result && (vizType === "bar") && <TrendChart data={result.points} />}
        {result && vizType === "line" && <LineTrendChart data={result.points} />}
        {result && vizType === "area" && <AreaTrendChart data={result.points} />}
        {result && vizType === "pie" && <SegmentChart data={result.points} />}
        {result && vizType === "scatter" && (
          <ScatterTrendChart data={result.points.map((p) => ({ label: p.label, x: (p as any).x ?? 0, y: (p as any).y ?? 0 }))} />
        )}
        {result && vizType === "pivottable" && result.rowLabels && result.colLabels && (
          <PivotTable rowLabels={result.rowLabels} colLabels={result.colLabels} cells={result.cells ?? []} />
        )}
        {result && vizType === "heatmap" && <CalendarHeatmap points={result.points} />}
        {result && vizType === "sankey" && result.links && <SankeyDiagram links={result.links} />}
        {result && vizType === "sunburst" && result.nodes && <SunburstChart nodes={result.nodes} />}
      </div>
    </div>
  );
}

function DimensionPicker({
  dimType, setDimType, dimCol, setDimCol, defaultDim, dimField, setDimField, cols, fields, label = "Dimension",
}: {
  dimType: "column" | "field";
  setDimType: (v: "column" | "field") => void;
  dimCol: string; setDimCol: (v: string) => void; defaultDim: string;
  dimField: string; setDimField: (v: string) => void;
  cols: { name: string }[];
  fields: SemanticField[];
  label?: string;
}) {
  return (
    <div>
      <label className="text-xs font-medium text-slate-500 uppercase">{label}</label>
      <div className="flex gap-1 mt-1">
        <button onClick={() => setDimType("column")} className={`flex-1 text-xs py-1 rounded ${dimType === "column" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>Column</button>
        <button onClick={() => setDimType("field")} className={`flex-1 text-xs py-1 rounded ${dimType === "field" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>Computed</button>
      </div>
      {dimType === "field" ? (
        <select value={dimField} onChange={(e) => setDimField(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
          <option value="">computed dimension…</option>
          {fields.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
        </select>
      ) : (
        <select value={dimCol || defaultDim} onChange={(e) => setDimCol(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
          {cols.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
        </select>
      )}
    </div>
  );
}

function MetricPicker({
  metricType, setMetricType, metricCol, setMetricCol, defaultMetric, metricAgg, setMetricAgg, metricField, setMetricField, cols, fields, hideAgg,
}: {
  metricType: "column" | "field";
  setMetricType: (v: "column" | "field") => void;
  metricCol: string; setMetricCol: (v: string) => void; defaultMetric: string;
  metricAgg: "sum" | "avg" | "min" | "max" | "count"; setMetricAgg: (v: any) => void;
  metricField: string; setMetricField: (v: string) => void;
  cols: { name: string }[];
  fields: SemanticField[];
  hideAgg?: boolean;
}) {
  return (
    <div>
      <label className="text-xs font-medium text-slate-500 uppercase">Metric</label>
      <div className="flex gap-1 mt-1">
        <button onClick={() => setMetricType("column")} className={`flex-1 text-xs py-1 rounded ${metricType === "column" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>Column</button>
        <button onClick={() => setMetricType("field")} className={`flex-1 text-xs py-1 rounded ${metricType === "field" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>Computed</button>
      </div>
      {metricType === "field" ? (
        <select value={metricField} onChange={(e) => setMetricField(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
          <option value="">computed metric…</option>
          {fields.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
        </select>
      ) : (
        <div className={hideAgg ? "" : "grid grid-cols-2 gap-2"}>
          <select value={metricCol || defaultMetric} onChange={(e) => setMetricCol(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
            {cols.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
          </select>
          {!hideAgg && (
            <select value={metricAgg} onChange={(e) => setMetricAgg(e.target.value)} className="w-full mt-1 px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
              <option value="sum">SUM</option>
              <option value="avg">AVG</option>
              <option value="min">MIN</option>
              <option value="max">MAX</option>
              <option value="count">COUNT</option>
            </select>
          )}
        </div>
      )}
    </div>
  );
}
