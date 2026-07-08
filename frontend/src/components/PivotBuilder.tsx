
import { useState, useCallback, useEffect } from "react";
import { GripVertical, X, Plus, Table2, Sigma, Filter, List, LayoutGrid, Play } from "lucide-react";
import type { Column } from "@/lib/api";

type PivotZone = "rows" | "values" | "filters";

type PivotField = {
  column: string;
  type: Column["type"];
  aggregation?: "sum" | "avg" | "count" | "min" | "max";
};

type PivotBuilderProps = {
  columns: Column[];
  onGenerate: (config: { rows: PivotField[]; values: PivotField[]; filters: PivotField[] }) => void;
  loading?: boolean;
};

const ZONE_CONFIG: Record<PivotZone, { label: string; icon: React.ReactNode; placeholder: string; color: string }> = {
  rows: {
    label: "Rows",
    icon: <List size={14} />,
    placeholder: "Drop or click to add row fields",
    color: "border-indigo-200 bg-indigo-50/30",
  },
  values: {
    label: "Values",
    icon: <Sigma size={14} />,
    placeholder: "Drop or click to add numeric fields",
    color: "border-emerald-200 bg-emerald-50/30",
  },
  filters: {
    label: "Filters",
    icon: <Filter size={14} />,
    placeholder: "Drop or click to add filter fields",
    color: "border-amber-200 bg-amber-50/30",
  },
};

const AGG_OPTIONS: { value: PivotField["aggregation"]; label: string }[] = [
  { value: "sum", label: "SUM" },
  { value: "avg", label: "AVG" },
  { value: "count", label: "COUNT" },
  { value: "min", label: "MIN" },
  { value: "max", label: "MAX" },
];

function getAggForType(type: Column["type"]): PivotField["aggregation"] {
  if (type === "numeric" || type === "integer" || type === "float" || type === "number" || type === "real" || type === "decimal") return "sum";
  return "count";
}

export function PivotBuilder({ columns, onGenerate, loading }: PivotBuilderProps) {
  const [rows, setRows] = useState<PivotField[]>([]);
  const [values, setValues] = useState<PivotField[]>([]);
  const [filters, setFilters] = useState<PivotField[]>([]);
  const [dragOver, setDragOver] = useState<PivotZone | null>(null);

  useEffect(() => {
    setRows([]);
    setValues([]);
    setFilters([]);
  }, [columns]);

  const numericCols = columns.filter(c => c.type === "numeric" || c.type === "integer" || c.type === "float" || c.type === "number" || c.type === "real" || c.type === "decimal");

  const handleDragStart = (e: React.DragEvent, col: Column, sourceZone?: PivotZone, sourceIdx?: number) => {
    e.dataTransfer.setData("text/plain", JSON.stringify({ column: col.name, type: col.type, sourceZone, sourceIdx }));
    e.dataTransfer.effectAllowed = "move";
  };

  const handleDragOver = (e: React.DragEvent, zone: PivotZone) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    setDragOver(zone);
  };

  const handleDragLeave = () => setDragOver(null);

  const handleDrop = (e: React.DragEvent, targetZone: PivotZone) => {
    e.preventDefault();
    setDragOver(null);
    try {
      const data = JSON.parse(e.dataTransfer.getData("text/plain"));
      if (data.sourceZone && data.sourceIdx !== undefined) {
        const sourceArr = data.sourceZone === "rows" ? rows : data.sourceZone === "values" ? values : filters;
        const field = sourceArr[data.sourceIdx];
        if (!field) return;
        const newField = { ...field, aggregation: targetZone === "values" ? field.aggregation || getAggForType(field.type) : undefined };
        removeField(data.sourceZone as PivotZone, data.sourceIdx);
        addField(targetZone, newField);
      } else {
        const col = columns.find(c => c.name === data.column);
        if (!col) return;
        const field: PivotField = {
          column: col.name,
          type: col.type,
          aggregation: targetZone === "values" ? getAggForType(col.type) : undefined,
        };
        addField(targetZone, field);
      }
    } catch { /* ignore */ }
  };

  const addField = (zone: PivotZone, field: PivotField) => {
    if (zone === "rows") setRows(prev => [...prev, field]);
    else if (zone === "values") setValues(prev => [...prev, field]);
    else setFilters(prev => [...prev, field]);
  };

  const removeField = (zone: PivotZone, idx: number) => {
    if (zone === "rows") setRows(prev => prev.filter((_, i) => i !== idx));
    else if (zone === "values") setValues(prev => prev.filter((_, i) => i !== idx));
    else setFilters(prev => prev.filter((_, i) => i !== idx));
  };

  const changeAgg = (idx: number, agg: PivotField["aggregation"]) => {
    setValues(prev => prev.map((f, i) => i === idx ? { ...f, aggregation: agg } : f));
  };

  const handleQuickAddColumn = (col: Column) => {
    if (numericCols.includes(col)) {
      if (!values.some(v => v.column === col.name)) {
        setValues(prev => [...prev, { column: col.name, type: col.type, aggregation: getAggForType(col.type) }]);
      }
    } else {
      if (!rows.some(r => r.column === col.name)) {
        setRows(prev => [...prev, { column: col.name, type: col.type }]);
      }
    }
  };

  const canGenerate = rows.length > 0 && values.length > 0;

  const handleGenerate = () => {
    if (!canGenerate) return;
    onGenerate({ rows, values, filters });
  };

  const renderZone = (zone: PivotZone) => {
    const items = zone === "rows" ? rows : zone === "values" ? values : filters;
    const config = ZONE_CONFIG[zone];

    return (
      <div
        onDragOver={(e) => handleDragOver(e, zone)}
        onDragLeave={handleDragLeave}
        onDrop={(e) => handleDrop(e, zone)}
        className={`p-3 rounded-xl border-2 border-dashed transition-all min-h-[80px] ${
          dragOver === zone ? "border-indigo-400 bg-indigo-50" : config.color
        }`}
      >
        <div className="flex items-center gap-1.5 mb-2">
          <span className="text-slate-400">{config.icon}</span>
          <span className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">{config.label}</span>
          <span className="text-[10px] text-slate-400 ml-auto">{items.length}</span>
        </div>
        {items.length === 0 ? (
          <p className="text-[11px] text-slate-400 italic">{config.placeholder}</p>
        ) : (
          <div className="space-y-1.5">
            {items.map((field, i) => (
              <div
                key={`${field.column}-${i}`}
                draggable
                onDragStart={(e) => handleDragStart(e, { name: field.column, type: field.type } as Column, zone, i)}
                className="flex items-center gap-1.5 px-2.5 py-1.5 bg-white rounded-lg border border-slate-200 shadow-sm text-xs cursor-grab active:cursor-grabbing hover:border-indigo-300 transition-colors group"
              >
                <GripVertical size={12} className="text-slate-300 shrink-0" />
                <span className="font-medium text-slate-700 truncate flex-1">{field.column}</span>
                {zone === "values" && (
                  <select
                    value={field.aggregation || "sum"}
                    onChange={(e) => changeAgg(i, e.target.value as PivotField["aggregation"])}
                    onClick={(e) => e.stopPropagation()}
                    className="text-[10px] font-mono text-indigo-600 bg-indigo-50 border border-indigo-200 rounded px-1 py-0.5 cursor-pointer hover:bg-indigo-100"
                  >
                    {AGG_OPTIONS.map(opt => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                )}
                <button
                  onClick={() => removeField(zone, i)}
                  className="p-0.5 rounded text-slate-300 hover:text-red-500 hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-all"
                >
                  <X size={12} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="p-4 bg-white rounded-2xl border border-slate-200/80 shadow-sm space-y-4">
      <div className="flex items-center gap-2 px-1">
        <div className="p-1.5 bg-indigo-100 rounded-lg">
          <LayoutGrid size={14} className="text-indigo-600" />
        </div>
        <span className="text-xs font-semibold text-slate-700 uppercase tracking-wider">Pivot Builder</span>
      </div>

      {columns.length === 0 ? (
        <div className="p-4 text-center text-xs text-slate-400 bg-slate-50 rounded-xl">
          Select a dataset to see available columns
        </div>
      ) : (
        <>
          <div className="space-y-3">
            {renderZone("rows")}
            {renderZone("values")}
            {renderZone("filters")}
          </div>

          <div className="pt-2 border-t border-slate-100">
            <div className="flex items-center gap-1.5 mb-2">
              <Table2 size={12} className="text-slate-400" />
              <span className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
                Available Columns
              </span>
              <span className="text-[10px] text-slate-400 ml-auto">{columns.length}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {columns.map((col) => {
                const inRows = rows.some(r => r.column === col.name);
                const inValues = values.some(v => v.column === col.name);
                const inFilters = filters.some(f => f.column === col.name);
                const inUse = inRows || inValues || inFilters;
                const isNumeric = numericCols.includes(col);
                return (
                  <button
                    key={col.name}
                    draggable
                    onDragStart={(e) => handleDragStart(e, col)}
                    onClick={() => handleQuickAddColumn(col)}
                    className={`inline-flex items-center gap-1 px-2 py-1 text-[11px] font-medium rounded-lg border transition-all group ${
                      inUse
                        ? "bg-indigo-100 border-indigo-200 text-indigo-600 cursor-default"
                        : "bg-slate-50 border-slate-200 text-slate-600 hover:bg-indigo-50 hover:border-indigo-200 hover:text-indigo-600 cursor-pointer"
                    }`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${isNumeric ? "bg-emerald-400" : "bg-indigo-400"}`} />
                    {col.name}
                    {!inUse && <Plus size={10} className="opacity-0 group-hover:opacity-100" />}
                  </button>
                );
              })}
            </div>
          </div>

          <div className="pt-2 border-t border-slate-100">
            <button
              onClick={handleGenerate}
              disabled={!canGenerate || loading}
              className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-700 hover:to-indigo-600 text-white rounded-xl font-semibold text-sm transition-all disabled:opacity-40 disabled:cursor-not-allowed shadow-sm shadow-indigo-500/20 hover:shadow-md active:scale-[0.98]"
            >
              <Play size={14} fill="currentColor" />
              {loading ? "Generating..." : "Generate Analysis"}
            </button>
            {!canGenerate && !loading && (
              <p className="text-[10px] text-slate-400 text-center mt-1.5">Add at least one Row and one Value field</p>
            )}
          </div>
        </>
      )}
    </div>
  );
}
