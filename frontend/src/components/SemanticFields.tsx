import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Trash2,
  Save,
  FunctionSquare,
  Tag,
} from "lucide-react";
import type { Dataset, SemanticField, Column } from "@/lib/api";
import {
  fetchDatasetFields,
  createDatasetField,
  deleteDatasetField,
} from "@/lib/api";

type Operand = { kind: "column" | "number"; name?: string; value?: number };

function numericColumns(cols: Column[]): Column[] {
  return cols.filter((c) => c.type === "number");
}
function categoricalColumns(cols: Column[]): Column[] {
  return cols.filter((c) => c.type === "text" || c.type === "date" || c.type === "number");
}

export function SemanticFields({
  dataset,
  onToast,
}: {
  dataset: Dataset;
  onToast?: (msg: string, kind?: "success" | "error") => void;
}) {
  const [fields, setFields] = useState<SemanticField[]>([]);
  const [loading, setLoading] = useState(false);
  const [kind, setKind] = useState<"metric" | "dimension">("metric");

  // metric form
  const [name, setName] = useState("");
  const [metricMode, setMetricMode] = useState<"aggregate" | "expression">("aggregate");
  const [mColumn, setMColumn] = useState("");
  const [mAgg, setMAgg] = useState<"sum" | "avg" | "min" | "max" | "count">("sum");
  const [leftKind, setLeftKind] = useState<"column" | "number">("column");
  const [leftCol, setLeftCol] = useState("");
  const [leftNum, setLeftNum] = useState("1");
  const [op, setOp] = useState<"+" | "-" | "*" | "/">("+");
  const [rightKind, setRightKind] = useState<"column" | "number">("column");
  const [rightCol, setRightCol] = useState("");
  const [rightNum, setRightNum] = useState("1");

  // dimension form
  const [dimMode, setDimMode] = useState<"column" | "transform">("column");
  const [dColumn, setDColumn] = useState("");
  const [dTransform, setDTransform] = useState<"month" | "year" | "day" | "upper" | "lower">("month");

  const cols = dataset.profile?.columns ?? [];
  const nums = numericColumns(cols);
  const cats = categoricalColumns(cols);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const f = await fetchDatasetFields(dataset.id);
      setFields(f);
    } catch (e) {
      onToast?.(String(e), "error");
    } finally {
      setLoading(false);
    }
  }, [dataset.id, onToast]);

  useEffect(() => {
    load();
  }, [load]);

  function resetForm() {
    setName("");
    setMetricMode("aggregate");
    setMColumn("");
    setMAgg("sum");
    setLeftKind("column");
    setLeftCol("");
    setLeftNum("1");
    setOp("+");
    setRightKind("column");
    setRightCol("");
    setRightNum("1");
    setDimMode("column");
    setDColumn("");
    setDTransform("month");
  }

  function buildConfig(): unknown {
    if (kind === "metric") {
      if (metricMode === "aggregate") {
        return { mode: "aggregate", agg: mAgg, column: mColumn };
      }
      const left: Operand =
        leftKind === "column"
          ? { kind: "column", name: leftCol }
          : { kind: "number", value: Number(leftNum) };
      const right: Operand =
        rightKind === "column"
          ? { kind: "column", name: rightCol }
          : { kind: "number", value: Number(rightNum) };
      return { mode: "expression", left, op, right };
    }
    if (dimMode === "column") {
      return { mode: "column", column: dColumn };
    }
    return { mode: "transform", column: dColumn, transform: dTransform };
  }

  async function handleSave() {
    if (!name) {
      onToast?.("Name is required", "error");
      return;
    }
    const config = buildConfig();
    try {
      await createDatasetField({ datasetId: dataset.id, name, kind, config });
      onToast?.("Field saved", "success");
      resetForm();
      load();
    } catch (e) {
      onToast?.(String(e), "error");
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteDatasetField(id);
      load();
    } catch (e) {
      onToast?.(String(e), "error");
    }
  }

  const metrics = fields.filter((f) => f.kind === "metric");
  const dimensions = fields.filter((f) => f.kind === "dimension");

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-sm font-semibold text-slate-700 flex items-center gap-2">
          <FunctionSquare size={16} /> Computed Fields for {dataset.filename}
        </h3>
        <p className="text-xs text-slate-500 mt-1">
          Build reusable metrics &amp; dimensions without writing SQL. They appear as
          selectable fields in the Chart Builder.
        </p>
      </div>

      {/* Existing fields */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <FieldList title="Metrics" icon={<FunctionSquare size={14} />} items={metrics} onDelete={handleDelete} />
        <FieldList title="Dimensions" icon={<Tag size={14} />} items={dimensions} onDelete={handleDelete} />
      </div>

      {/* Builder form */}
      <div className="bg-white rounded-2xl p-5 border border-slate-200 space-y-4">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setKind("metric")}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${kind === "metric" ? "bg-indigo-600 text-white" : "bg-slate-100 text-slate-600"}`}
          >
            Metric
          </button>
          <button
            onClick={() => setKind("dimension")}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${kind === "dimension" ? "bg-indigo-600 text-white" : "bg-slate-100 text-slate-600"}`}
          >
            Dimension
          </button>
        </div>

        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Field name (e.g. avg_revenue)"
          className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm"
        />

        {kind === "metric" && (
          <div className="space-y-3">
            <select
              value={metricMode}
              onChange={(e) => setMetricMode(e.target.value as "aggregate" | "expression")}
              className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white"
            >
              <option value="aggregate">Aggregation of a column</option>
              <option value="expression">Expression (column op column/number)</option>
            </select>

            {metricMode === "aggregate" ? (
              <div className="grid grid-cols-2 gap-3">
                <select value={mColumn} onChange={(e) => setMColumn(e.target.value)} className="px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                  <option value="">Select column…</option>
                  {nums.map((c) => (
                    <option key={c.name} value={c.name}>{c.name}</option>
                  ))}
                </select>
                <select value={mAgg} onChange={(e) => setMAgg(e.target.value as any)} className="px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                  <option value="sum">SUM</option>
                  <option value="avg">AVG</option>
                  <option value="min">MIN</option>
                  <option value="max">MAX</option>
                  <option value="count">COUNT</option>
                </select>
              </div>
            ) : (
              <div className="grid grid-cols-3 gap-2 items-center">
                <OperandPicker
                  kind={leftKind} setKind={setLeftKind}
                  col={leftCol} setCol={setLeftCol}
                  num={leftNum} setNum={setLeftNum}
                  options={nums}
                />
                <select value={op} onChange={(e) => setOp(e.target.value as any)} className="px-2 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                  <option value="+">+</option>
                  <option value="-">−</option>
                  <option value="*">×</option>
                  <option value="/">÷</option>
                </select>
                <OperandPicker
                  kind={rightKind} setKind={setRightKind}
                  col={rightCol} setCol={setRightCol}
                  num={rightNum} setNum={setRightNum}
                  options={nums}
                />
              </div>
            )}
          </div>
        )}

        {kind === "dimension" && (
          <div className="space-y-3">
            <select value={dimMode} onChange={(e) => setDimMode(e.target.value as "column" | "transform")} className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white">
              <option value="column">Use a column</option>
              <option value="transform">Transform a column</option>
            </select>
            <select value={dColumn} onChange={(e) => setDColumn(e.target.value)} className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white">
              <option value="">Select column…</option>
              {cats.map((c) => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
            {dimMode === "transform" && (
              <select value={dTransform} onChange={(e) => setDTransform(e.target.value as any)} className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white">
                <option value="month">Month of date</option>
                <option value="year">Year of date</option>
                <option value="day">Day of date</option>
                <option value="upper">UPPER(text)</option>
                <option value="lower">LOWER(text)</option>
              </select>
            )}
          </div>
        )}

        <button
          onClick={handleSave}
          disabled={loading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
        >
          <Save size={14} /> Save Field
        </button>
      </div>
    </div>
  );
}

function OperandPicker({
  kind, setKind, col, setCol, num, setNum, options,
}: {
  kind: "column" | "number";
  setKind: (k: "column" | "number") => void;
  col: string; setCol: (v: string) => void;
  num: string; setNum: (v: string) => void;
  options: Column[];
}) {
  return (
    <div className="flex flex-col gap-1">
      <div className="flex gap-1">
        <button onClick={() => setKind("column")} className={`flex-1 text-xs py-1 rounded ${kind === "column" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>col</button>
        <button onClick={() => setKind("number")} className={`flex-1 text-xs py-1 rounded ${kind === "number" ? "bg-indigo-100 text-indigo-700" : "bg-slate-100 text-slate-500"}`}>#</button>
      </div>
      {kind === "column" ? (
        <select value={col} onChange={(e) => setCol(e.target.value)} className="px-2 py-1.5 rounded border border-slate-300 text-sm bg-white">
          <option value="">col…</option>
          {options.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
        </select>
      ) : (
        <input value={num} onChange={(e) => setNum(e.target.value)} className="px-2 py-1.5 rounded border border-slate-300 text-sm" placeholder="0" />
      )}
    </div>
  );
}

function FieldList({
  title, icon, items, onDelete,
}: {
  title: string;
  icon: React.ReactNode;
  items: SemanticField[];
  onDelete: (id: string) => void;
}) {
  return (
    <div className="bg-white rounded-2xl p-4 border border-slate-200">
      <div className="flex items-center gap-2 text-sm font-semibold text-slate-700 mb-2">
        {icon} {title} ({items.length})
      </div>
      {items.length === 0 ? (
        <p className="text-xs text-slate-400 italic">None yet.</p>
      ) : (
        <ul className="space-y-1.5">
          {items.map((f) => (
            <li key={f.id} className="flex items-center justify-between group">
              <span className="text-sm text-slate-700">{f.name}</span>
              <button onClick={() => onDelete(f.id)} className="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-red-500 transition-all">
                <Trash2 size={14} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
