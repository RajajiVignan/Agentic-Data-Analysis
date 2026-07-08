import { useState, useRef, useCallback, useEffect, Fragment } from 'react';
import { Pin, Info, ChevronDown, ChevronRight, Download, Loader2, Clock, Brain, Coffee, BarChart3, Activity, Image, Code, BarChart as BarChartIcon, X } from 'lucide-react';
import { useDashboardFilter } from '@/components/DashboardFilterContext';
import { ChartContextMenu } from '@/components/ChartContextMenu';
import type { ContextMenuAction } from '@/components/ChartContextMenu';
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  AreaChart,
  Area,
  ScatterChart,
  Scatter,
  ComposedChart,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
  ZAxis,
  Brush,
} from 'recharts';
import { downloadChartPng, downloadChartJpeg } from '@/lib/export';

const LOADING_QUOTES = [
  { icon: Brain, text: "Teaching the AI to read your data... it's not looking great." },
  { icon: Coffee, text: "Brewing coffee for the analysis engine. This might take a while." },
  { icon: BarChart3, text: "Convincing the chart to stop being a bar and become a line..." },
  { icon: Activity, text: "Your data is doing yoga. Stretching into shape." },
  { icon: Clock, text: "Plot twist: the plot isn't ready yet." },
  { icon: Brain, text: "The AI is having an existential crisis about bar charts." },
  { icon: Coffee, text: "Hang tight — we're mining insights with a tiny pickaxe." },
  { icon: BarChart3, text: "Counting all the rows so you don't have to." },
  { icon: Activity, text: "Running linear regression on a hamster wheel." },
  { icon: Clock, text: "Loading... statistically significant chance of waiting." },
  { icon: Coffee, text: "Our data elves are on a coffee break. They'll be right back." },
  { icon: Brain, text: "Reticulating splines on your dataset. This is fine." },
  { icon: BarChart3, text: "The chart is in the shop getting a tune-up." },
  { icon: Activity, text: "Bribing the backend with positive reinforcement." },
  { icon: Clock, text: "The AI is contemplating the meaning of pie." },
  { icon: Coffee, text: "Delegating to a microservice made of duct tape." },
  { icon: Brain, text: "Your data is on fire — in a good way. Probably." },
  { icon: BarChart3, text: "Consulting the ancient scrolls of matplotlib." },
  { icon: Activity, text: "The pandas are processing. Not the animal, the library." },
  { icon: Clock, text: "This would be faster on a toaster." },
];

export function PlotLoading() {
  const [idx, setIdx] = useState(() => Math.floor(Math.random() * LOADING_QUOTES.length));

  useEffect(() => {
    const timer = setInterval(() => {
      setIdx(() => Math.floor(Math.random() * LOADING_QUOTES.length));
    }, 15000);
    return () => clearInterval(timer);
  }, []);

  const Q = LOADING_QUOTES[idx];
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-12 text-center animate-fade-in" key={idx}>
      <div className="relative">
        <Loader2 size={32} className="animate-spin text-indigo-400" />
        <div className="absolute inset-0 animate-ping opacity-20">
          <Q.icon size={32} className="text-indigo-600" />
        </div>
      </div>
      <div className="space-y-1">
        <p className="text-sm font-medium text-slate-500 italic">{Q.text}</p>
        <p className="text-[10px] text-slate-400">Hang on, this is the fun part</p>
      </div>
    </div>
  );
}

type KpiProps = {
  label: string;
  value: string;
  change: string;
};

function getChartColors(): string[] {
  if (typeof document === 'undefined') return ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#06b6d4', '#a855f7'];
  const raw = getComputedStyle(document.documentElement).getPropertyValue('--chart-colors').trim();
  if (!raw) return ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#06b6d4', '#a855f7'];
  return raw.split(',').map(s => s.trim());
}

function getAccentColor(): string {
  if (typeof document === 'undefined') return '#6366f1';
  return getComputedStyle(document.documentElement).getPropertyValue('--accent').trim() || '#6366f1';
}

type ChartPoint = {
  label: string;
  value: number;
};

function PinButton({ onClick }: { onClick?: () => void }) {
  return (
    <button
      onClick={onClick}
      className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600 transition-all"
      title="Pin to Dashboard"
    >
      <Pin size={16} />
    </button>
  );
}

function DownloadDropdown({ elementRef }: { elementRef: React.RefObject<HTMLDivElement | null> }) {
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDownload = useCallback(async (format: "png" | "jpeg") => {
    setOpen(false);
    setError(null);
    if (!elementRef.current) return;
    const title = elementRef.current.dataset?.exportPlot || "chart";
    try {
      if (format === "png") {
        await downloadChartPng(elementRef.current, title);
      } else {
        await downloadChartJpeg(elementRef.current, title);
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Download failed";
      console.error("Download failed:", e);
      setError(msg);
      setTimeout(() => setError(null), 4000);
    }
  }, [elementRef]);

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600 transition-all"
        title="Download chart"
      >
        <Download size={16} />
      </button>
      {error && (
        <div className="absolute right-0 top-full mt-1 z-30 bg-red-50 border border-red-200 rounded-lg px-3 py-1.5 text-xs text-red-600 whitespace-nowrap shadow animate-fade-in">
          {error}
        </div>
      )}
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 top-full mt-1 z-20 bg-white rounded-xl border border-slate-200 shadow-lg py-1 min-w-[100px] animate-fade-in">
            <button
              onClick={() => handleDownload("png")}
              className="w-full text-left px-4 py-2 text-sm text-slate-600 hover:bg-slate-50 hover:text-indigo-600 transition-colors"
            >
              PNG
            </button>
            <button
              onClick={() => handleDownload("jpeg")}
              className="w-full text-left px-4 py-2 text-sm text-slate-600 hover:bg-slate-50 hover:text-indigo-600 transition-colors"
            >
              JPEG
            </button>
          </div>
        </>
      )}
    </div>
  );
}

function ChartBadge({ label }: { label: string }) {
  return (
    <span className="text-[10px] font-medium text-slate-400 px-2 py-0.5 rounded-md bg-slate-100/50 border border-slate-200/50">
      {label}
    </span>
  );
}

function ChartCard({ children, title, badge, onPin, className = "", chartType, onChartTypeChange, onDrillDown, cardRef: externalRef }: {
  children: React.ReactNode;
  title: string;
  badge: string;
  onPin?: () => void;
  className?: string;
  chartType?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
  cardRef?: React.RefObject<HTMLDivElement | null>;
}) {
  const internalRef = useRef<HTMLDivElement>(null);
  const cardRef = externalRef || internalRef;

  const handleContextAction = useCallback((action: ContextMenuAction, payload?: string) => {
    switch (action) {
      case "drilldown":
        onDrillDown?.();
        break;
      case "changeType":
        if (payload && onChartTypeChange) onChartTypeChange(payload);
        break;
      case "filter":
        break;
      case "exportPng":
        if (cardRef.current) {
          downloadChartPng(cardRef.current, title).catch(console.error);
        }
        break;
      case "exportJpeg":
        if (cardRef.current) {
          downloadChartJpeg(cardRef.current, title).catch(console.error);
        }
        break;
    }
  }, [cardRef, title, onDrillDown, onChartTypeChange]);

  return (
    <ChartContextMenu
      chartTitle={title}
      chartType={chartType}
      onAction={handleContextAction}
      onChangeChartType={onChartTypeChange}
      onExportPng={() => { if (cardRef.current) downloadChartPng(cardRef.current, title).catch(console.error); }}
      onExportJpeg={() => { if (cardRef.current) downloadChartJpeg(cardRef.current, title).catch(console.error); }}
      onDrillDown={onDrillDown}
    >
      <div ref={cardRef} className={`p-5 bg-white rounded-2xl space-y-4 relative group card-hover card-modern ${className}`} data-export-plot={title}>
        <div className="flex items-center justify-between">
          <strong className="text-sm font-semibold text-slate-800">{title}</strong>
          <div className="flex items-center gap-1">
            <DownloadDropdown elementRef={cardRef} />
            <PinButton onClick={onPin} />
            <ChartBadge label={badge} />
          </div>
        </div>
        {children}
      </div>
    </ChartContextMenu>
  );
}

export function MetricTile({ label, value, change, onPin }: KpiProps & { onPin?: () => void }) {
  const isPositive = change.includes('+');
  return (
    <div className="p-5 bg-white rounded-2xl space-y-1.5 relative group card-hover card-modern">
      <div className="absolute top-3 right-3">
        <button
          onClick={onPin}
          className="p-1.5 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600 transition-all"
          title="Pin to Dashboard"
        >
          <Pin size={12} />
        </button>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-[11px] font-medium text-slate-400 uppercase tracking-wider">{label}</span>
      </div>
      <strong className="block text-2xl font-bold text-slate-900 tracking-tight">{value}</strong>
      <div className="flex items-center gap-1.5">
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${isPositive ? 'bg-emerald-500' : 'bg-slate-400'}`} />
        <small className={`text-xs font-medium ${isPositive ? 'text-emerald-600' : 'text-slate-500'}`}>
          {change}
        </small>
      </div>
    </div>
  );
}

export function PythonPlot({ url, plotType, onPin }: { url?: string; plotType?: string; onPin?: () => void }) {
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);

  if (!url) return null;

  const isHtml = plotType === 'bokeh' || plotType === 'plotly' || url.endsWith('.html');
  const badge = isHtml ? `Python (${plotType === 'bokeh' ? 'Bokeh' : 'Plotly'})` : 'Python (Matplotlib/Seaborn)';
  const src = retry > 0 ? `${url}${url.includes('?') ? '&' : '?'}_retry=${retry}` : url;

  return (
    <ChartCard title="AI Generated Visualization" badge={badge} onPin={onPin} data-export-plot="Python Plot">
      {!loaded && !error && <PlotLoading />}
      {error && (
        <div className="flex flex-col items-center justify-center gap-3 py-8 text-center">
          <p className="text-sm text-slate-400 italic">Plot escaped. We&apos;re looking for it.</p>
          <button
            onClick={() => { setError(false); setLoaded(false); setRetry(r => r + 1); }}
            className="px-4 py-2 text-xs font-medium text-indigo-600 bg-indigo-50 rounded-lg hover:bg-indigo-100 transition-colors"
          >
            Retry
          </button>
        </div>
      )}
      {isHtml ? (
        <div className={`w-full rounded-xl overflow-hidden border border-slate-100/80 ${loaded ? '' : 'hidden'}`} style={{ height: '520px' }}>
          <iframe
            src={src}
            className="w-full h-full"
            title="Interactive Visualization"
            onLoad={() => setLoaded(true)}
            onError={() => { setLoaded(true); setError(true); }}
            sandbox="allow-scripts allow-same-origin"
          />
        </div>
      ) : (
        <div className={`flex justify-center bg-slate-50/50 rounded-xl overflow-hidden border border-slate-100/80 ${loaded ? '' : 'hidden'}`}>
          <img
            src={src}
            alt="AI Visualization"
            className="w-full h-auto object-contain"
            onLoad={() => setLoaded(true)}
            onError={() => { setLoaded(true); setError(true); }}
          />
        </div>
      )}
    </ChartCard>
  );
}

export function VizTypeSelector({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const options = [
    { id: 'matplotlib', label: 'Matplotlib', icon: Image },
    { id: 'bokeh', label: 'Bokeh', icon: Code },
    { id: 'plotly', label: 'Plotly', icon: BarChartIcon },
  ];

  return (
    <div className="flex items-center gap-1.5">
      {options.map((opt) => {
        const active = value === opt.id;
        return (
          <button
            key={opt.id}
            onClick={() => onChange(opt.id)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-all ${
              active
                ? 'bg-indigo-100 text-indigo-700 shadow-sm ring-1 ring-indigo-200'
                : 'bg-slate-50 text-slate-500 hover:bg-slate-100 hover:text-slate-700'
            }`}
          >
            <opt.icon size={14} />
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

export function TrendChart({ data, onPin, filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ChartPoint[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const accentColor = getAccentColor();

  const handleBarClick = (entry: ChartPoint) => {
    filter.addFilter({ column: filterColumn, value: entry.label, operator: "eq" });
  };

  return (
    <ChartCard title="Interactive Trend" badge="Recharts Bar" onPin={onPin} chartType="bar" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Trend Chart">
      <ResponsiveContainer width="100%" height={280}>
        <BarChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" strokeOpacity={0.5} />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '10px',
              fontSize: '12px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
            }}
          />
          <Bar
            dataKey="value"
            fill={accentColor}
            radius={[6, 6, 0, 0]}
            maxBarSize={48}
            animationDuration={600}
            animationEasing="ease-out"
            cursor="pointer"
            onClick={(entry) => handleBarClick(entry.payload as ChartPoint)}
          >
            {data.map((entry, i) => {
              const highlighted = filter.isActive(filterColumn, entry.label);
              return (
                <Cell
                  key={i}
                  fill={highlighted ? accentColor : `${accentColor}55`}
                  stroke={highlighted ? accentColor : 'none'}
                  strokeWidth={highlighted ? 2 : 0}
                />
              );
            })}
          </Bar>
          {data.length > 5 && <Brush dataKey="label" height={24} stroke={accentColor} fill="#e2e8f0" />}
        </BarChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function SegmentChart({ data, onPin, filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ChartPoint[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const colors = getChartColors();

  const handlePieClick = (entry: ChartPoint) => {
    filter.addFilter({ column: filterColumn, value: entry.label, operator: "eq" });
  };

  return (
    <ChartCard title="Interactive Segments" badge="Recharts Pie" onPin={onPin} chartType="pie" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Segment Chart">
      <ResponsiveContainer width="100%" height={280}>
        <PieChart>
          <Pie
            data={data}
            dataKey="value"
            nameKey="label"
            cx="50%"
            cy="50%"
            outerRadius={100}
            animationBegin={0}
            animationDuration={800}
            animationEasing="ease-out"
            label={({ name, percent }) => `${name ?? ""} ${((percent ?? 0) * 100).toFixed(0)}%`}
            cursor="pointer"
            onClick={(entry) => handlePieClick(entry.payload as ChartPoint)}
          >
            {data.map((entry, i) => {
              const highlighted = filter.isActive(filterColumn, entry.label);
              return (
                <Cell
                  key={i}
                  fill={colors[i % colors.length]}
                  opacity={highlighted ? 1 : 0.5}
                  stroke={highlighted ? colors[i % colors.length] : 'transparent'}
                  strokeWidth={highlighted ? 2 : 0}
                />
              );
            })}
          </Pie>
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '10px',
              fontSize: '12px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
            }}
          />
          <Legend wrapperStyle={{ fontSize: '11px' }} />
        </PieChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function LineTrendChart({ data, onPin, filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ChartPoint[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const accentColor = getAccentColor();

  const handleDotClick = (entry: unknown) => {
    const p = entry as { label?: string };
    if (p?.label) filter.addFilter({ column: filterColumn, value: p.label, operator: "eq" });
  };

  return (
    <ChartCard title="Time Series Trend" badge="Recharts Line" onPin={onPin} chartType="line" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Line Trend">
      <ResponsiveContainer width="100%" height={280}>
        <LineChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" strokeOpacity={0.5} />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '10px',
              fontSize: '12px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
            }}
          />
          <Line
            type="monotone"
            dataKey="value"
            stroke={accentColor}
            strokeWidth={2.5}
            dot={{ fill: accentColor, r: 4, strokeWidth: 2, stroke: '#fff', cursor: 'pointer' }}
            activeDot={{ r: 6, strokeWidth: 0 } as Record<string, unknown>}
            animationDuration={600}
            animationEasing="ease-out"
            onClick={(entry: unknown) => handleDotClick(entry)}
          />
          {data.length > 5 && <Brush dataKey="label" height={24} stroke={accentColor} fill="#e2e8f0" />}
        </LineChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

export function AreaTrendChart({ data, onPin, filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ChartPoint[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const accentColor = getAccentColor();

  const handleAreaClick = (entry: unknown) => {
    const p = entry as { label?: string };
    if (p?.label) filter.addFilter({ column: filterColumn, value: p.label, operator: "eq" });
  };

  const hasActiveFilter = filter.filters.some(f => f.column === filterColumn && f.value);

  return (
    <ChartCard title="Cumulative Trend" badge="Recharts Area" onPin={onPin} chartType="area" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Area Trend">
      <ResponsiveContainer width="100%" height={280}>
        <AreaChart data={data} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" strokeOpacity={0.5} />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '10px',
              fontSize: '12px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
            }}
          />
          <Area
            type="monotone"
            dataKey="value"
            stroke={accentColor}
            fill={accentColor}
            fillOpacity={hasActiveFilter ? 0.05 : 0.1}
            strokeWidth={2.5}
            animationDuration={600}
            animationEasing="ease-out"
            cursor="pointer"
            onClick={(entry: unknown) => handleAreaClick(entry)}
          />
        </AreaChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

type ScatterPoint = {
  label: string;
  x: number;
  y: number;
  z?: number;
};

export function ScatterTrendChart({ data, onPin, filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ScatterPoint[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const colors = getChartColors();
  const accentColor = getAccentColor();

  const filteredData = filter.filters.length > 0
    ? data.filter(d => !filter.filters.some(f => f.column === filterColumn && f.value && String(d.label) !== f.value))
    : data;

  return (
    <ChartCard title="Correlation Explorer" badge="Recharts Scatter" onPin={onPin} chartType="scatter" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Scatter Plot">
      <ResponsiveContainer width="100%" height={280}>
      <ScatterChart margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" strokeOpacity={0.5} />
        <XAxis dataKey="x" name="x" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
        <YAxis dataKey="y" name="y" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
        <ZAxis dataKey="z" range={[60, 400]} />
        <Tooltip
          contentStyle={{
            background: '#fff',
            border: '1px solid #e2e8f0',
            borderRadius: '10px',
            fontSize: '12px',
            boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
          }}
          cursor={{ strokeDasharray: '3 3' }}
        />
        <Scatter data={filteredData} fill={accentColor} fillOpacity={0.6} animationDuration={600} animationEasing="ease-out">
          {filteredData.map((point, i) => (
            <Cell key={i} fill={colors[i % colors.length]} />
          ))}
        </Scatter>
        </ScatterChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

type ComboPoint = {
  label: string;
  bars: number;
  line: number;
};

export function ComboChart({ data, onPin, barKey = "bars", lineKey = "line", filterColumn = "label", onChartTypeChange, onDrillDown }: {
  data: ComboPoint[];
  onPin?: () => void;
  barKey?: string;
  lineKey?: string;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;

  const filteredData = filter.filters.length > 0
    ? data.filter(d => !filter.filters.some(f => f.column === filterColumn && f.value && String(d.label) !== f.value))
    : data;

  return (
    <ChartCard title="Bar + Line Combo" badge="Recharts Composed" onPin={onPin} chartType="combo" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Combo Chart">
      <ResponsiveContainer width="100%" height={280}>
        <ComposedChart data={filteredData} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" strokeOpacity={0.5} />
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fontSize: 11, fill: '#64748b' }} axisLine={false} tickLine={false} />
          <Tooltip
            contentStyle={{
              background: '#fff',
              border: '1px solid #e2e8f0',
              borderRadius: '10px',
              fontSize: '12px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.06)',
            }}
          />
          <Bar dataKey={barKey} fill="var(--accent, #6366f1)" radius={[4, 4, 0, 0]} maxBarSize={32} animationDuration={600} animationEasing="ease-out" />
          <Line type="monotone" dataKey={lineKey} stroke="#ef4444" strokeWidth={2.5} dot={{ fill: '#ef4444', r: 4, strokeWidth: 2, stroke: '#fff' }} animationDuration={600} animationEasing="ease-out" />
        </ComposedChart>
      </ResponsiveContainer>
    </ChartCard>
  );
}

type HeatmapCell = {
  x: string;
  y: string;
  value: number;
};

export function SmartAutoViz({ data, chartType, onPin, title, onChartTypeChange, onDrillDown }: {
  data: ChartPoint[];
  chartType?: string;
  onPin?: () => void;
  title?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  if (!data || data.length === 0) return null;

  const shared = { onPin, onChartTypeChange, onDrillDown };

  if (chartType === 'scatter') {
    const hasNumericLabels = data.every((d) => !isNaN(Number(d.label)));
    if (!hasNumericLabels) {
      return <TrendChart data={data} {...shared} />;
    }
    return <ScatterTrendChart data={data.map((d) => ({ label: d.label, x: Number(d.label), y: d.value }))} {...shared} />;
  }

  switch (chartType) {
    case 'line':
      return <LineTrendChart data={data} {...shared} />;
    case 'pie':
      return <SegmentChart data={data} {...shared} />;
    case 'area':
      return <AreaTrendChart data={data} {...shared} />;
    case 'bar':
    default:
      return <TrendChart data={data} {...shared} />;
  }
}

export function HistogramChart({ column, buckets }: { column: string; buckets: { label: string; count: number }[] }) {
  if (!buckets || buckets.length === 0) return null;
  const maxCount = Math.max(...buckets.map((x) => x.count));
  return (
    <div className="p-4 bg-white rounded-2xl space-y-3 card-hover card-modern">
      <strong className="text-xs font-semibold text-slate-700">{column}</strong>
      <div className="space-y-1.5">
        {buckets.map((b, i) => {
          const pct = maxCount > 0 ? (b.count / maxCount) * 100 : 0;
          return (
            <div key={i} className="flex items-center gap-2 text-[11px]">
              <span className="w-24 text-right text-slate-500 truncate shrink-0">{b.label}</span>
              <div className="flex-1 h-4 bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-indigo-500 to-indigo-400 rounded-full transition-all duration-500"
                  style={{ width: `${pct}%` }}
                />
              </div>
              <span className="w-8 text-right text-slate-600 font-medium">{b.count}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function ExplainSection({ explanations }: { explanations: { chart: string; columns: string; sql?: string; warning?: string; grouping?: string }[] }) {
  const [open, setOpen] = useState(false);
  if (!explanations || explanations.length === 0) return null;
  return (
    <div className="bg-white rounded-2xl overflow-hidden card-hover card-modern">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Info size={16} className="text-indigo-500" />
          How This Analysis Works
        </div>
        {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
      </button>
      {open && (
        <div className="px-5 pb-4 space-y-3 animate-slide-up">
          {explanations.map((exp, i) => (
            <div key={i} className="p-3 bg-slate-50 rounded-xl text-xs space-y-1">
              <div className="flex items-center gap-2">
                <span className="px-1.5 py-0.5 bg-indigo-100 text-indigo-700 rounded text-[10px] font-medium uppercase">{exp.chart}</span>
                <span className="text-slate-500">Columns: {exp.columns}</span>
              </div>
              {exp.grouping && exp.grouping !== 'none' && (
                <p className="text-slate-400">Grouped by: {exp.grouping}</p>
              )}
              {exp.sql && (
                <pre className="mt-1 p-2 bg-slate-800 text-slate-200 rounded-lg overflow-x-auto text-[10px] leading-relaxed">{exp.sql}</pre>
              )}
              {exp.warning && (
                <p className="text-amber-600 flex items-center gap-1">
                  <span>⚠</span> {exp.warning}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function DashboardFilterBar() {
  const filter = useDashboardFilter();
  if (filter.filters.length === 0) return null;

  return (
    <div className="flex items-center gap-2 p-3 bg-indigo-50/80 border border-indigo-100/80 rounded-2xl flex-wrap">
      <span className="text-[10px] font-semibold text-indigo-600 uppercase tracking-wider">Cross-filter:</span>
      {filter.filters.map((f, i) => (
        <span
          key={i}
          className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium bg-white border border-indigo-200 rounded-full text-indigo-700"
        >
          <span className="opacity-60">{f.column}:</span> {f.value}
          <button
            onClick={() => filter.removeFilter(f.column)}
            className="ml-0.5 p-0.5 rounded-full hover:bg-indigo-100 text-indigo-400 hover:text-indigo-600 transition-colors"
          >
            <X size={12} />
          </button>
        </span>
      ))}
      <button
        onClick={filter.clearFilters}
        className="px-2 py-1 text-[10px] font-medium text-indigo-500 hover:text-indigo-700 hover:bg-indigo-100 rounded-lg transition-colors"
      >
        Clear all
      </button>
    </div>
  );
}

export function HeatmapChart({ data, onPin, filterColumn = "x", onChartTypeChange, onDrillDown }: {
  data: HeatmapCell[];
  onPin?: () => void;
  filterColumn?: string;
  onChartTypeChange?: (type: string) => void;
  onDrillDown?: () => void;
}) {
  const filter = useDashboardFilter();
  if (!data || data.length === 0) return null;
  const filteredData = filter.filters.length > 0
    ? data.filter(d => !filter.filters.some(f => f.value && d[filterColumn as keyof HeatmapCell] !== f.value))
    : data;

  const maxVal = Math.max(...filteredData.map((d) => d.value));
  const minVal = Math.min(...filteredData.map((d) => d.value));
  const range = maxVal - minVal || 1;

  const xLabels = [...new Set(filteredData.map((d) => d.x))];
  const yLabels = [...new Set(filteredData.map((d) => d.y))];

  const getColor = (val: number) => {
    const intensity = (val - minVal) / range;
    const r = Math.round(99 + (99 - 99) * intensity);
    const g = Math.round(102 + (241 - 102) * (1 - intensity));
    const b = Math.round(241 + (241 - 241) * intensity);
    return `rgb(${r}, ${g}, ${b})`;
  };

  return (
    <ChartCard title="Density Heatmap" badge="Grid" onPin={onPin} chartType="heatmap" onChartTypeChange={onChartTypeChange} onDrillDown={onDrillDown} data-export-plot="Heatmap">
      <div className="overflow-x-auto">
        <div className="grid gap-0.5" style={{ gridTemplateColumns: `auto repeat(${xLabels.length}, 1fr)` }}>
          <div className="text-[10px] text-slate-400 p-1" />
          {xLabels.map((xl) => (
            <div key={xl} className="text-[10px] text-slate-500 font-medium p-1 text-center truncate">{xl}</div>
          ))}
          {yLabels.map((yl) => (
            <Fragment key={yl}>
              <div className="text-[10px] text-slate-500 font-medium p-1 text-right truncate">{yl}</div>
              {xLabels.map((xl) => {
                const cell = filteredData.find((d) => d.x === xl && d.y === yl);
                const val = cell?.value ?? 0;
                return (
                  <div
                    key={`${xl}-${yl}`}
                    className="p-2 text-center text-[10px] font-medium rounded transition-transform hover:scale-110"
                    style={{
                      backgroundColor: getColor(val),
                      color: val > (minVal + range * 0.6) ? 'white' : '#475569',
                    }}
                    title={`${yl} / ${xl}: ${val.toFixed(1)}`}
                  >
                    {val.toFixed(0)}
                  </div>
                );
              })}
            </Fragment>
          ))}
        </div>
      </div>
    </ChartCard>
  );
}
