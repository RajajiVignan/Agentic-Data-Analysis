
import { useState, useRef, useEffect, useCallback } from "react";
import {
  GitMerge,
  X,
  Trash2,
  Link,
  Save,
  RefreshCw,
  Info,
  ArrowRight,
  Split,
} from "lucide-react";
import type { Dataset } from "@/lib/api";

type Column = { name: string; type: string };
type TableNode = {
  id: string;
  datasetId: string;
  tableName: string;
  columns: Column[];
  x: number;
  y: number;
};
type Relationship = {
  id: string;
  fromDatasetId: string;
  fromColumn: string;
  toDatasetId: string;
  toColumn: string;
  cardinality: "1:1" | "1:N" | "N:M";
};

type Props = {
  datasets: Dataset[];
  onSchemaApplied?: (relationships: Relationship[]) => void;
};

function generateId(): string {
  return Math.random().toString(36).substring(2, 10);
}

const CARDINALITY_OPTIONS: Relationship["cardinality"][] = ["1:1", "1:N", "N:M"];

const COLUMN_COLORS: Record<string, string> = {
  number: "text-blue-600 bg-blue-50 border-blue-200",
  text: "text-emerald-600 bg-emerald-50 border-emerald-200",
  date: "text-amber-600 bg-amber-50 border-amber-200",
  empty: "text-slate-400 bg-slate-50 border-slate-200",
};

export function SchemaDesigner({ datasets, onSchemaApplied }: Props) {
  const [tableNodes, setTableNodes] = useState<TableNode[]>([]);
  const [relationships, setRelationships] = useState<Relationship[]>([]);
  const [connecting, setConnecting] = useState<{
    datasetId: string;
    column: string;
  } | null>(null);
  const [selectedRel, setSelectedRel] = useState<string | null>(null);
  const [dragging, setDragging] = useState<{
    id: string;
    offsetX: number;
    offsetY: number;
  } | null>(null);
  const [canvasOffset, setCanvasOffset] = useState({ x: 0, y: 0 });
  const canvasRef = useRef<HTMLDivElement>(null);
  const columnRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const svgRef = useRef<SVGSVGElement>(null);

  // Initialize table nodes from datasets
  useEffect(() => {
    if (datasets.length === 0) {
      setTableNodes([]);
      return;
    }
    setTableNodes((prev) => {
      const existing = new Map(prev.map((n) => [n.datasetId, n]));
      const spacingX = 280;
      const spacingY = 200;
      let col = 0;
      let row = 0;
      const maxCol = Math.min(datasets.length, 3);
      return datasets.map((ds, i) => {
        const existingNode = existing.get(ds.id);
        if (existingNode) {
          return existingNode;
        }
        const node = {
          id: generateId(),
          datasetId: ds.id,
          tableName: ds.filename,
          columns: (ds.profile?.columns ?? []).map((c) => ({
            name: c.name,
            type: c.type,
          })),
          x: col * spacingX + 40,
          y: row * spacingY + 40,
        };
        col++;
        if (col >= maxCol) {
          col = 0;
          row++;
        }
        return node;
      });
    });
  }, [datasets]);

  const getColumnKey = (datasetId: string, column: string) =>
    `${datasetId}::${column}`;

  const getColumnPosition = useCallback(
    (datasetId: string, column: string): { x: number; y: number } | null => {
      const key = getColumnKey(datasetId, column);
      const el = columnRefs.current.get(key);
      if (!el || !canvasRef.current) return null;
      const canvasRect = canvasRef.current.getBoundingClientRect();
      const elRect = el.getBoundingClientRect();
      return {
        x: elRect.left - canvasRect.left + elRect.width / 2 + canvasRef.current.scrollLeft,
        y: elRect.top - canvasRect.top + elRect.height / 2 + canvasRef.current.scrollTop,
      };
    },
    []
  );

  function handleColumnClick(datasetId: string, column: string) {
    if (!connecting) {
      setConnecting({ datasetId, column });
      return;
    }
    if (connecting.datasetId === datasetId && connecting.column === column) {
      setConnecting(null);
      return;
    }
    if (connecting.datasetId === datasetId) {
      setConnecting({ datasetId, column });
      return;
    }
    const exists = relationships.some(
      (r) =>
        (r.fromDatasetId === connecting.datasetId &&
          r.fromColumn === connecting.column &&
          r.toDatasetId === datasetId &&
          r.toColumn === column) ||
        (r.fromDatasetId === datasetId &&
          r.fromColumn === column &&
          r.toDatasetId === connecting.datasetId &&
          r.toColumn === connecting.column)
    );
    if (exists) {
      setConnecting(null);
      return;
    }
    const rel: Relationship = {
      id: generateId(),
      fromDatasetId: connecting.datasetId,
      fromColumn: connecting.column,
      toDatasetId: datasetId,
      toColumn: column,
      cardinality: "1:N",
    };
    setRelationships((prev) => [...prev, rel]);
    setConnecting(null);
  }

  function handleDeleteRelationship(id: string) {
    setRelationships((prev) => prev.filter((r) => r.id !== id));
    setSelectedRel(null);
  }

  function handleCardinalityChange(
    id: string,
    cardinality: Relationship["cardinality"]
  ) {
    setRelationships((prev) =>
      prev.map((r) => (r.id === id ? { ...r, cardinality } : r))
    );
  }

  function handleMouseDown(
    e: React.MouseEvent,
    nodeId: string
  ) {
    if ((e.target as HTMLElement).closest(".column-item")) return;
    e.preventDefault();
    const node = tableNodes.find((n) => n.id === nodeId);
    if (!node) return;
    setDragging({
      id: nodeId,
      offsetX: e.clientX - node.x,
      offsetY: e.clientY - node.y,
    });
  }

  useEffect(() => {
    const currentDrag = dragging;
    if (!currentDrag) return;
    const { id: dragId, offsetX, offsetY } = currentDrag;
    function onMouseMove(e: MouseEvent) {
      setTableNodes((prev) =>
        prev.map((n) =>
          n.id === dragId
            ? { ...n, x: Math.max(0, e.clientX - offsetX), y: Math.max(0, e.clientY - offsetY) }
            : n
        )
      );
    }
    function onMouseUp() {
      setDragging(null);
    }
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, [dragging]);

  function renderSvgLines() {
    const lines: React.ReactNode[] = [];
    for (const rel of relationships) {
      const fromPos = getColumnPosition(rel.fromDatasetId, rel.fromColumn);
      const toPos = getColumnPosition(rel.toDatasetId, rel.toColumn);
      if (!fromPos || !toPos) continue;

      const isSelected = selectedRel === rel.id;
      const strokeColor = isSelected ? "#6366f1" : "#94a3b8";
      const strokeWidth = isSelected ? 2.5 : 1.5;

      const midY = (fromPos.y + toPos.y) / 2;
      const path = `M ${fromPos.x} ${fromPos.y} C ${fromPos.x} ${midY}, ${toPos.x} ${midY}, ${toPos.x} ${toPos.y}`;

      lines.push(
        <g key={rel.id}>
          {/* Invisible wider line for easier clicking */}
          <path
            d={path}
            fill="none"
            stroke="transparent"
            strokeWidth={12}
            style={{ cursor: "pointer" }}
            onClick={() => setSelectedRel(isSelected ? null : rel.id)}
          />
          <path
            d={path}
            fill="none"
            stroke={strokeColor}
            strokeWidth={strokeWidth}
            strokeDasharray={isSelected ? "none" : "none"}
            style={{ cursor: "pointer" }}
            onClick={() => setSelectedRel(isSelected ? null : rel.id)}
          />
          {/* Cardinality label */}
          <rect
            x={Math.min(fromPos.x, toPos.x) + Math.abs(toPos.x - fromPos.x) / 2 - 18}
            y={midY - 10}
            width={36}
            height={20}
            rx={4}
            fill={isSelected ? "#eef2ff" : "#f8fafc"}
            stroke={strokeColor}
            strokeWidth={1}
            style={{ cursor: "pointer" }}
            onClick={() => setSelectedRel(isSelected ? null : rel.id)}
          />
          <text
            x={Math.min(fromPos.x, toPos.x) + Math.abs(toPos.x - fromPos.x) / 2}
            y={midY + 4}
            textAnchor="middle"
            fontSize={10}
            fontWeight={600}
            fill={strokeColor}
            style={{ cursor: "pointer", userSelect: "none" }}
            onClick={() => setSelectedRel(isSelected ? null : rel.id)}
          >
            {rel.cardinality}
          </text>
        </g>
      );
    }
    // Connecting line (preview)
    if (connecting) {
      const fromPos = getColumnPosition(connecting.datasetId, connecting.column);
      if (fromPos) {
        lines.push(
          <line
            key="connecting"
            x1={fromPos.x}
            y1={fromPos.y}
            x2={fromPos.x + 100}
            y2={fromPos.y + 50}
            stroke="#6366f1"
            strokeWidth={2}
            strokeDasharray="6 3"
            opacity={0.6}
          />
        );
      }
    }
    return lines;
  }

  function handleClearAll() {
    setRelationships([]);
    setSelectedRel(null);
    setConnecting(null);
  }

  function handleResetLayout() {
    const spacingX = 280;
    const spacingY = 200;
    let col = 0;
    let row = 0;
    const maxCol = Math.min(tableNodes.length, 3);
    setTableNodes((prev) =>
      prev.map((n) => {
        const node = { ...n, x: col * spacingX + 40, y: row * spacingY + 40 };
        col++;
        if (col >= maxCol) {
          col = 0;
          row++;
        }
        return node;
      })
    );
  }

  function handleApplySchema() {
    onSchemaApplied?.(relationships);
  }

  const connectedDatasetIds = new Set<string>();
  for (const rel of relationships) {
    connectedDatasetIds.add(rel.fromDatasetId);
    connectedDatasetIds.add(rel.toDatasetId);
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-2.5">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-slate-800">
            <GitMerge size={16} className="text-indigo-500" />
            Schema Designer
          </div>
          <span className="text-[10px] text-slate-400 hidden sm:inline">
            Click a column, then click another to create a relationship
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <button
            onClick={handleResetLayout}
            className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-all"
            title="Reset table layout"
          >
            <RefreshCw size={14} />
          </button>
          <button
            onClick={handleClearAll}
            disabled={relationships.length === 0}
            className="p-1.5 text-slate-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed"
            title="Clear all relationships"
          >
            <Trash2 size={14} />
          </button>
          {relationships.length > 0 && onSchemaApplied && (
            <button
              onClick={handleApplySchema}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 text-white text-xs font-medium rounded-lg hover:bg-indigo-700 transition-all"
            >
              <ArrowRight size={12} />
              Apply Schema
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-4">
        {/* Canvas */}
        <div
          ref={canvasRef}
          className="relative flex-1 bg-white rounded-xl border border-slate-200 shadow-sm overflow-auto"
          style={{ height: "calc(100vh - 280px)", minHeight: 500 }}
        >
          <div
            className="relative"
            style={{
              width: Math.max(1200, tableNodes.length * 300 + 200),
              height: Math.max(800, Math.ceil(tableNodes.length / 3) * 250 + 200),
            }}
          >
            {/* SVG overlay for relationship lines */}
            <svg
              ref={svgRef}
              className="absolute inset-0 pointer-events-none z-10"
              width="100%"
              height="100%"
              style={{ pointerEvents: "none" }}
            >
              <g style={{ pointerEvents: "auto" }}>{renderSvgLines()}</g>
            </svg>

            {/* Table cards */}
            {tableNodes.map((node) => (
              <div
                key={node.id}
                className={`absolute bg-white rounded-xl border shadow-sm transition-shadow ${
                  connecting && connecting.datasetId === node.datasetId
                    ? "border-indigo-300 shadow-indigo-100 z-20"
                    : "border-slate-200 hover:shadow-md z-10"
                } ${dragging?.id === node.id ? "shadow-lg cursor-grabbing z-30" : ""}`}
                style={{
                  left: node.x,
                  top: node.y,
                  width: 240,
                  cursor: dragging?.id === node.id ? "grabbing" : "default",
                }}
              >
                {/* Header - drag handle */}
                <div
                  className="flex items-center gap-2 px-3 py-2.5 bg-slate-50 border-b border-slate-200 rounded-t-xl cursor-grab active:cursor-grabbing select-none"
                  onMouseDown={(e) => handleMouseDown(e, node.id)}
                >
                  <div className="w-2 h-2 rounded-full bg-indigo-500 shrink-0" />
                  <span className="text-sm font-semibold text-slate-800 truncate flex-1">
                    {node.tableName}
                  </span>
                  <span className="text-[10px] text-slate-400 font-medium">
                    {node.columns.length} cols
                  </span>
                </div>

                {/* Columns */}
                <div className="py-1">
                  {node.columns.length === 0 && (
                    <div className="px-3 py-2 text-[11px] text-slate-400 italic">
                      No columns
                    </div>
                  )}
                  {node.columns.map((col) => {
                    const colKey = getColumnKey(node.datasetId, col.name);
                    const isConnecting =
                      connecting &&
                      connecting.datasetId === node.datasetId &&
                      connecting.column === col.name;
                    const hasConnection = relationships.some(
                      (r) =>
                        (r.fromDatasetId === node.datasetId &&
                          r.fromColumn === col.name) ||
                        (r.toDatasetId === node.datasetId &&
                          r.toColumn === col.name)
                    );
                    const colorClass =
                      COLUMN_COLORS[col.type] ?? COLUMN_COLORS.text;

                    return (
                      <div
                        key={col.name}
                        ref={(el) => {
                          if (el) columnRefs.current.set(colKey, el);
                          else columnRefs.current.delete(colKey);
                        }}
                        className={`column-item flex items-center gap-2 px-3 py-1.5 transition-all cursor-pointer group ${
                          isConnecting
                            ? "bg-indigo-50"
                            : hasConnection
                            ? "bg-indigo-50/40"
                            : "hover:bg-slate-50"
                        } ${connecting && !isConnecting ? "hover:bg-indigo-50/50" : ""}`}
                        onClick={() =>
                          handleColumnClick(node.datasetId, col.name)
                        }
                      >
                        {/* Connection dot */}
                        <div
                          className={`w-2 h-2 rounded-full shrink-0 transition-all ${
                            isConnecting
                              ? "bg-indigo-500 scale-125"
                              : connecting
                              ? "bg-indigo-300 group-hover:bg-indigo-500 group-hover:scale-125"
                              : "bg-slate-300 group-hover:bg-indigo-400 group-hover:scale-125"
                          }`}
                        />
                        <span className="text-xs text-slate-700 truncate flex-1 font-medium">
                          {col.name}
                        </span>
                        <span
                          className={`text-[10px] font-medium px-1.5 py-0.5 rounded border ${colorClass}`}
                        >
                          {col.type}
                        </span>
                        {hasConnection && (
                          <Link size={10} className="text-indigo-400 shrink-0" />
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}

            {/* Empty state */}
            {tableNodes.length === 0 && (
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="text-center">
                  <Split size={32} className="mx-auto mb-3 text-slate-300" />
                  <p className="text-sm text-slate-400">
                    No datasets available. Upload or connect data sources first.
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Relationship inspector sidebar */}
        {tableNodes.length > 0 && (
          <div className="w-64 shrink-0 space-y-3">
            {/* Legend */}
            <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-3 space-y-2">
              <h4 className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
                Legend
              </h4>
              <div className="space-y-1">
                {Object.entries({
                  number: "Number",
                  text: "Text",
                  date: "Date",
                }).map(([type, label]) => (
                  <div key={type} className="flex items-center gap-2 text-xs">
                    <span
                      className={`px-1.5 py-0.5 rounded border text-[10px] font-medium ${
                        COLUMN_COLORS[type]
                      }`}
                    >
                      {type}
                    </span>
                    <span className="text-slate-500">{label}</span>
                  </div>
                ))}
              </div>
            </div>

            {/* Relationships list */}
            <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-3 space-y-2">
              <div className="flex items-center justify-between">
                <h4 className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
                  Relationships ({relationships.length})
                </h4>
                {relationships.length > 0 && (
                  <button
                    onClick={handleClearAll}
                    className="text-[10px] text-red-400 hover:text-red-600 transition-colors"
                  >
                    Clear all
                  </button>
                )}
              </div>
              {relationships.length === 0 ? (
                <p className="text-[11px] text-slate-400 italic">
                  Click a column, then click another to connect them.
                </p>
              ) : (
                <div className="space-y-1.5 max-h-64 overflow-y-auto">
                  {relationships.map((rel) => {
                    const isSelected = selectedRel === rel.id;
                    const fromTable = tableNodes.find(
                      (n) => n.datasetId === rel.fromDatasetId
                    );
                    const toTable = tableNodes.find(
                      (n) => n.datasetId === rel.toDatasetId
                    );
                    return (
                      <div
                        key={rel.id}
                        className={`p-2 rounded-lg border transition-all cursor-pointer ${
                          isSelected
                            ? "border-indigo-300 bg-indigo-50"
                            : "border-slate-200 hover:border-slate-300 bg-white"
                        }`}
                        onClick={() =>
                          setSelectedRel(isSelected ? null : rel.id)
                        }
                      >
                        <div className="flex items-center justify-between mb-1">
                          <div className="flex items-center gap-1 text-[10px] font-medium text-slate-600">
                            <Info size={10} />
                            <select
                              value={rel.cardinality}
                              onChange={(e) =>
                                handleCardinalityChange(
                                  rel.id,
                                  e.target
                                    .value as Relationship["cardinality"]
                                )
                              }
                              className="text-[10px] border border-slate-200 rounded px-1 py-0.5 bg-white cursor-pointer"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {CARDINALITY_OPTIONS.map((opt) => (
                                <option key={opt} value={opt}>
                                  {opt}
                                </option>
                              ))}
                            </select>
                          </div>
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDeleteRelationship(rel.id);
                            }}
                            className="text-slate-300 hover:text-red-500 transition-colors"
                          >
                            <X size={10} />
                          </button>
                        </div>
                        <div className="text-[11px] text-slate-600 flex items-center gap-1">
                          <span className="font-medium truncate max-w-[80px]">
                            {fromTable?.tableName ?? "?"}
                          </span>
                          <span className="text-indigo-500">.</span>
                          <span className="text-indigo-600 font-medium">
                            {rel.fromColumn}
                          </span>
                        </div>
                        <div className="text-[11px] text-slate-400 flex items-center gap-1">
                          <ArrowRight size={10} />
                        </div>
                        <div className="text-[11px] text-slate-600 flex items-center gap-1">
                          <span className="font-medium truncate max-w-[80px]">
                            {toTable?.tableName ?? "?"}
                          </span>
                          <span className="text-indigo-500">.</span>
                          <span className="text-indigo-600 font-medium">
                            {rel.toColumn}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Tips */}
            <div className="bg-indigo-50 rounded-xl border border-indigo-200 p-3 space-y-1">
              <h4 className="text-[10px] font-semibold text-indigo-700 uppercase tracking-wider flex items-center gap-1">
                <Info size={10} />
                Tips
              </h4>
              <ul className="text-[11px] text-indigo-600 space-y-0.5 list-disc pl-3">
                <li>Drag table headers to reposition</li>
                <li>Click column dot to start connection</li>
                <li>Click another column dot to complete</li>
                <li>Click a line to select then change cardinality</li>
              </ul>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
