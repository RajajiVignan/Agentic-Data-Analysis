import React from "react";

// Visualization components for P1-3 (breadth): pivot table, calendar heatmap,
// sankey, and sunburst. Implemented without external chart libraries so they
// drop into the existing recharts-based flow without new dependencies.

const PALETTE = [
  "#6366f1", "#0ea5e9", "#10b981", "#f59e0b", "#ef4444",
  "#8b5cf6", "#ec4899", "#14b8a6", "#f97316", "#64748b",
];

function colorFor(index: number): string {
  return PALETTE[index % PALETTE.length];
}

// --- Pivot Table ---

export function PivotTable({
  rowLabels,
  colLabels,
  cells,
}: {
  rowLabels: string[];
  colLabels: string[];
  cells: number[][];
}) {
  if (!rowLabels.length || !colLabels.length) {
    return <div className="text-slate-400 text-sm p-4">No pivot data.</div>;
  }
  const max = Math.max(1, ...cells.flat().map((v) => Math.abs(v) || 0));
  return (
    <div className="overflow-auto">
      <table className="text-sm border-collapse">
        <thead>
          <tr>
            <th className="sticky left-0 bg-slate-50 px-3 py-2 text-left text-xs font-semibold text-slate-500 border-b border-slate-200">
              {""}
            </th>
            {colLabels.map((c, i) => (
              <th key={c} className="px-3 py-2 text-right text-xs font-semibold text-slate-500 border-b border-slate-200">
                {c}
                <span className="block text-[10px] text-slate-400">c{i + 1}</span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rowLabels.map((r, ri) => (
            <tr key={r} className="odd:bg-white even:bg-slate-50/40">
              <th className="sticky left-0 bg-slate-50 px-3 py-2 text-left text-xs font-medium text-slate-600 border-b border-slate-200">
                {r}
              </th>
              {colLabels.map((_, ci) => {
                const v = cells[ri]?.[ci] ?? 0;
                const intensity = Math.min(1, Math.abs(v) / max);
                const bg = v < 0
                  ? `rgba(239,68,68,${0.12 + intensity * 0.5})`
                  : `rgba(99,102,241,${0.12 + intensity * 0.5})`;
                return (
                  <td
                    key={ci}
                    className="px-3 py-2 text-right tabular-nums border-b border-slate-100"
                    style={{ background: bg }}
                  >
                    {Number.isFinite(v) ? v.toFixed(2) : "0.00"}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// --- Calendar Heatmap ---

export function CalendarHeatmap({ points }: { points: { label: string; value: number }[] }) {
  if (!points.length) {
    return <div className="text-slate-400 text-sm p-4">No heatmap data.</div>;
  }
  const values = points.map((p) => p.value);
  const max = Math.max(1, ...values.map((v) => Math.abs(v)));
  const min = Math.min(0, ...values);
  const extent = Math.max(1, max - min);
  const weeks = Math.ceil(points.length / 7);
  return (
    <div>
      <div
        className="grid gap-1"
        style={{ gridTemplateColumns: `repeat(${weeks}, minmax(0, 1fr))` }}
      >
        {points.map((p) => {
          const norm = (p.value - min) / extent;
          const bg = `rgba(99,102,241,${0.15 + norm * 0.8})`;
          return (
            <div
              key={p.label}
              title={`${p.label}: ${p.value.toFixed(2)}`}
              className="aspect-square rounded-sm"
              style={{ background: bg }}
            />
          );
        })}
      </div>
      <div className="flex items-center gap-2 mt-2 text-[11px] text-slate-400">
        <span>low</span>
        <div className="h-2 w-24 rounded" style={{ background: "linear-gradient(90deg, rgba(99,102,241,0.15), rgba(99,102,241,0.95))" }} />
        <span>high</span>
      </div>
    </div>
  );
}

// --- Sankey Diagram (2-layer) ---

type SankeyNode = { name: string; value: number; y0: number; y1: number; x: number };

export function SankeyDiagram({ links }: { links: { source: string; target: string; value: number }[] }) {
  if (!links.length) {
    return <div className="text-slate-400 text-sm p-4">No flow data.</div>;
  }
  const width = 640;
  const height = 360;
  const pad = 24;
  const nodeWidth = 14;
  const gap = 12;

  const sources = Array.from(new Set(links.map((l) => l.source)));
  const targets = Array.from(new Set(links.map((l) => l.target)));
  const sourceVal = new Map<string, number>();
  const targetVal = new Map<string, number>();
  for (const l of links) {
    sourceVal.set(l.source, (sourceVal.get(l.source) ?? 0) + l.value);
    targetVal.set(l.target, (targetVal.get(l.target) ?? 0) + l.value);
  }
  const totalSource = Math.max(1, ...sources.map((s) => sourceVal.get(s) ?? 0));
  const totalTarget = Math.max(1, ...targets.map((t) => targetVal.get(t) ?? 0));
  const total = Math.max(totalSource, totalTarget);

  const layout = (
    names: string[],
    vals: Map<string, number>,
    x: number,
  ): SankeyNode[] => {
    const usable = height - 2 * pad - gap * (names.length - 1);
    let y = pad;
    return names.map((n) => {
      const h = (vals.get(n) ?? 0) / total * usable;
      const node = { name: n, value: vals.get(n) ?? 0, y0: y, y1: y + h };
      y += h + gap;
      return node;
    }).map((node) => ({ ...node, x })) as SankeyNode[];
  };

  const left = layout(sources, sourceVal, pad);
  const right = layout(targets, targetVal, width - pad - nodeWidth);
  const leftByName = new Map(left.map((n) => [n.name, n]));
  const rightByName = new Map(right.map((n) => [n.name, n]));

  const maxLink = Math.max(1, ...links.map((l) => l.value));
  const linkPath = (s: SankeyNode, t: SankeyNode, value: number) => {
    const x0 = s.x + nodeWidth;
    const x1 = t.x;
    const sy = (s.y0 + s.y1) / 2;
    const ty = (t.y0 + t.y1) / 2;
    const cx = (x0 + x1) / 2;
    const w = Math.max(1, (value / maxLink) * 24);
    return `M ${x0} ${sy} C ${cx} ${sy}, ${cx} ${ty}, ${x1} ${ty}`;
  };

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" role="img" aria-label="Sankey diagram">
      {links.map((l, i) => {
        const s = leftByName.get(l.source);
        const t = rightByName.get(l.target);
        if (!s || !t) return null;
        return (
          <path
            key={i}
            d={linkPath(s, t, l.value)}
            fill="none"
            stroke={colorFor(i)}
            strokeOpacity={0.35}
            strokeWidth={Math.max(1, (l.value / maxLink) * 24)}
          >
            <title>{`${l.source} → ${l.target}: ${l.value.toFixed(2)}`}</title>
          </path>
        );
      })}
      {[...left, ...right].map((n, i) => (
        <g key={i}>
          <rect x={n.x} y={n.y0} width={nodeWidth} height={Math.max(1, n.y1 - n.y0)} fill={colorFor(i)} rx={2}>
            <title>{`${n.name}: ${n.value.toFixed(2)}`}</title>
          </rect>
          <text
            x={n.x < width / 2 ? n.x + nodeWidth + 4 : n.x - 4}
            y={(n.y0 + n.y1) / 2}
            textAnchor={n.x < width / 2 ? "start" : "end"}
            dominantBaseline="middle"
            fontSize={11}
            fill="#334155"
          >
            {n.name}
          </text>
        </g>
      ))}
    </svg>
  );
}

// --- Sunburst Chart (2-level) ---

export function SunburstChart({
  nodes,
}: {
  nodes: { name: string; children?: { name: string; value: number }[]; value?: number }[];
}) {
  if (!nodes.length) {
    return <div className="text-slate-400 text-sm p-4">No hierarchy data.</div>;
  }
  const size = 360;
  const cx = size / 2;
  const cy = size / 2;
  const rInner = size * 0.22;
  const rOuter = size * 0.46;

  const total = nodes.reduce((s, n) => s + (n.value ?? n.children?.reduce((cs, c) => cs + c.value, 0) ?? 0), 0) || 1;
  let angle = -Math.PI / 2;

  const polar = (r: number, a: number) => ({ x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) });
  const arc = (r0: number, r1: number, a0: number, a1: number) => {
    const large = a1 - a0 > Math.PI ? 1 : 0;
    const p0 = polar(r1, a0);
    const p1 = polar(r1, a1);
    const p2 = polar(r0, a1);
    const p3 = polar(r0, a0);
    return `M ${p0.x} ${p0.y} A ${r1} ${r1} 0 ${large} 1 ${p1.x} ${p1.y} L ${p2.x} ${p2.y} A ${r0} ${r0} 0 ${large} 0 ${p3.x} ${p3.y} Z`;
  };

  const slices: React.ReactNode[] = [];
  nodes.forEach((n, ni) => {
    const nVal = n.value ?? n.children?.reduce((cs, c) => cs + c.value, 0) ?? 0;
    const nAngle = (nVal / total) * Math.PI * 2;
    const a0 = angle;
    const a1 = angle + nAngle;
    slices.push(
      <path key={`p${ni}`} d={arc(rInner, rOuter, a0, a1)} fill={colorFor(ni)} stroke="#fff" strokeWidth={1}>
        <title>{`${n.name}: ${nVal.toFixed(2)}`}</title>
      </path>,
    );
    if (n.children) {
      const cTotal = n.children.reduce((cs, c) => cs + c.value, 0) || 1;
      let ca = a0;
      n.children.forEach((c, ci) => {
        const cAngle = (c.value / cTotal) * nAngle;
        slices.push(
          <path key={`c${ni}-${ci}`} d={arc(rInner * 0.55, rInner, ca, ca + cAngle)} fill={colorFor(ni + ci + 3)} stroke="#fff" strokeWidth={1}>
            <title>{`${n.name} / ${c.name}: ${c.value.toFixed(2)}`}</title>
          </path>,
        );
        ca += cAngle;
      });
    }
    angle = a1;
  });

  return (
    <svg viewBox={`0 0 ${size} ${size}`} className="w-full h-auto max-w-[420px] mx-auto" role="img" aria-label="Sunburst chart">
      <circle cx={cx} cy={cy} r={rInner * 0.5} fill="#f8fafc" />
      {slices}
    </svg>
  );
}
