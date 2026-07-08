# InsightPilot — Roadmap to Superset-level BI

> Goal: borrow the best interactive/exploration patterns from Apache Superset
> (https://github.com/apache/superset) while keeping InsightPilot's AI-native core.
> Superset is **no-code, dataset-centric** BI. InsightPilot is **prompt-driven, AI-native**
> BI. The strategy is to add Superset's *interactive* strengths on top of our AI layer,
> not to replace it.

---

## 1. Positioning: what each does well

| Dimension | Superset | InsightPilot (today) |
|-----------|----------|----------------------|
| Primary mode | No-code Explore UI | Natural-language prompt → KPIs/charts |
| Chart creation | Pick dataset, configure dims/metrics/agg | AI picks columns; `VizWidget`/`PivotBuilder` refine |
| Semantic layer | Virtual metrics & dimensions per dataset | `GlossaryPanel` = definitions only (not computed) |
| Data sources | 40+ SQLAlchemy connectors | DuckDB + few live DB connections |
| Dashboards | Native filters, tabs, Markdown, cross-filter | Pinned charts, `DashboardEditor`, cross-filter |
| SQL | SQL Lab (history, saved queries, multi-tab) | `SQLQueryEditor` + export |
| Caching | Flask-Caching result cache | None |
| Auth/RBAC | Fine-grained roles & permissions | login / register / guest |
| Viz engine | Apache ECharts (broad: geo, sankey, sunburst…) | recharts (bar/line/pie/scatter/area/combo) |
| Scale | Celery workers, K8s, Helm | Single Go binary, in-memory + Supabase |
| AI | None | Deterministic + LLM analyzer, planner, sessions |

**Takeaway:** We already cover much of Superset's surface (SQL, dashboards, joins,
transforms, profiler, scheduler, connections, share). The real gaps are the
**no-code chart builder**, the **computed semantic layer**, and **caching**.

---

## 2. Prioritized adoption plan

### P0 — highest leverage (do first)

**P0-1. Unified No-Code Chart Builder ("Explore")**
- Superset core: pick dataset → choose viz type → set dimension / metric /
  aggregation / sort / color / limit → get a reusable chart saved to a dashboard.
- Today we have fragments: `VizWidget` (viz-type + axis/agg), `PivotBuilder`
  (drag zones), `ChartEditBar` (NL edits). No single coherent flow that produces
  a saved, reusable chart object.
- Target: a single `ExploreView` that, given a selected dataset, exposes:
  - viz-type selector (reuse existing `VizTypeSelector`)
  - dimension (categorical/x-axis) picker
  - metric (numeric) picker + aggregation (sum/avg/min/max/count)
  - sort (asc/desc), top-N limit, color scheme
  - live preview rendered by existing `Charts.tsx` components
  - "Save to dashboard" → reuses `pinChart` / dashboard layout infra
- Reuses: `Charts.tsx`, `DashboardFilterContext`, `pinChart` API, `DashboardEditor`.

**P0-2. Semantic Layer: virtual metrics & dimensions**
- Superset lets users define computed columns per dataset, e.g.
  - metric: `SUM(revenue) / COUNT(*)` named `avg_revenue`
  - dimension: `DATE_TRUNC('month', created_at)` named `month`
- Today `GlossaryPanel` stores text definitions only; they are not used in queries.
- Target: per-dataset computed fields stored in DB, usable in:
  - the chart builder (P0-1) as selectable metric/dimension
  - the SQL query builder (auto-injected expression)
  - the AI analyzer (exposed as known metrics to the LLM)
- Backed by: new table `dataset_fields` (dataset_id, name, expr, kind: metric|dimension).
  Validation + sandbox (reuse `sandbox_validator.go`) so users can't inject raw SQL danger.

### P1 — scale & depth

**P1-1. Result caching** — cache analysis/SQL/DuckDB results keyed by a hash of
(query + dataset version). In-memory TTL now; Redis-ready interface later.

**P1-2. SQL Lab depth** — query history, saved queries, multi-tab, one-click
result export (CSV/JSON). `SQLQueryEditor` + `export.ts` already exist.

**P1-3. Visualization breadth** — add recharts-lacking types Superset shines at:
pivot table, calendar heatmap, sankey, sunburst, geospatial/map. Consider Apache
ECharts for these (Superset's chosen engine) alongside recharts.

**P1-4. Async query execution** — run long DuckDB/SQL queries in a background job
with a job id + poll, instead of the synchronous `QUERY_TIMEOUT_SEC` cut-off.

### P2 — enterprise hardening

**P2-1. RBAC / roles** — roles (viewer/analyst/admin) with per-dataset permissions.
**P2-2. Chart plugin registry** — replace the hardcoded `switch` in `Charts.tsx`
with a registered viz-type map for extensibility.
**P2-3. Native dashboard widgets** — filter components, tab strips, Markdown/text tiles.
**P2-4. Alerting channels** — add Slack/webhook sinks to existing SMTP reports/alerts.

---

## 3. What NOT to copy from Superset

- The 40+ SQLAlchemy connector zoo — heavy ops burden; keep DuckDB + a few live connections.
- Celery + Kubernetes + Helm infrastructure — overkill for the single-binary Go model.
- Generic enterprise RBAC complexity — only add if a real multi-tenant need appears.

---

## 4. Suggested execution order

1. **P0-1** No-code chart builder (biggest gap; complements AI — AI suggests, user refines).
2. **P0-2** Computed metrics/dimensions in semantic layer (feeds builder + SQL + AI).
3. **P1-1** Caching (immediate perf win for large CSVs).
4. **P1-3** More viz types.
5. **P1-2** SQL Lab depth.
6. P2 items as needed.

---

## 5. Open questions before P0 implementation

- Should computed metric SQL be user-authored (needs sandbox) or builder-composed
  from safe primitives (column + operator + aggregation)?
- Where should the chart builder live in the UI? New "Explore" nav tab, or replace
  the current `explore` tab content?
- Persistence: store saved charts as pinned charts (existing) or a new `charts` table?

---

## 6. Implementation status

Decisions (2026-07-08): builder-composed safe metrics, new "Chart Builder" nav tab,
reuse pinned charts for persistence.

**P0 — IMPLEMENTED**

- `internal/data/semantic.go` — `SemanticField` model + safe SQL builder
  (whitelist aggregation/operator/transform; column validation).
- `internal/store/db.go` — `dataset_fields` table + CRUD.
- `internal/api/semantic.go` — `SemanticFieldService` + GET/POST/PUT/DELETE
  `/api/dataset-fields` (mirrors glossary service).
- `internal/api/explore.go` — `/api/explore` no-code chart endpoint
  (bar/line/area/pie/scatter/kpi) with dimension+metric resolution, safe SQL,
  SELECT-only validation.
- `frontend/src/lib/api.ts` — types + `fetchDatasetFields`, `createDatasetField`,
  `updateDatasetField`, `deleteDatasetField`, `explore`.
- `frontend/src/components/SemanticFields.tsx` — builder-composed computed
  metrics & dimensions UI.
- `frontend/src/components/ExploreView.tsx` — no-code chart builder (dataset,
  viz type, dimension, metric+agg, sort, top-N, live preview, save to dashboard).
- `frontend/src/app/page.tsx` + `Sidebar.tsx` — new "Chart Builder" tab wired in.

Verified end-to-end: upload → create computed field → explore bar/KPI/scatter with
real DuckDB queries; SQL-injection / unknown-column attempts rejected.

**P1 — IMPLEMENTED**

- **P1-1. Result caching** (`internal/cache/cache.go`)
  - `Cache` interface + `MemoryCache` (TTL, background sweep, `Redis-ready`).
  - `datasetVersion()` hashes (columns+rowCount+path) so edited data invalidates cache.
  - Wired into `handleQuery` (`cachedExecuteSQL`) and `explore` (`cachedExploreSQL`),
    keyed by `HashKey(query + dataset versions + pagination)`. TTL via `CACHE_TTL_SEC`
    (default 5m). Closed cleanly in `Shutdown()`.

- **P1-4. Async query execution** (`internal/api/async_query.go`)
  - `POST /api/query-async` → returns `{jobId, status:"running"}`; runs DuckDB/SQL in a
    background goroutine with a long timeout (`QUERY_ASYNC_TIMEOUT_SEC`, default 600s),
    no `QUERY_TIMEOUT_SEC` cut-off.
  - `GET /api/query-job?id=` → polls `{status, columns, rows, hasMore, ...}`.
  - `asyncQueryManager` tracks jobs + 30-min sweep.

- **P1-2. SQL Lab depth** (`internal/api/sql_lab.go`, `internal/store/db.go`)
  - `saved_queries` table + `SavedQueryService` (CRUD, persisted).
  - `GET /api/query-history` (bounded in-memory ring, auto-recorded by `handleQuery`).
  - `GET/POST/PUT/DELETE /api/saved-queries` (named, reusable queries).
  - Frontend `SQLQueryEditor.tsx`: multi-tab editor, collapsible History + Saved
    queries panels, "Run async" with polling, and one-click Export CSV / JSON
    (`queryResultToCSV` + `downloadText` in `lib/api.ts`).

- **P1-3. Visualization breadth** (`internal/api/explore.go`, `components/VizComponents.tsx`, `ExploreView.tsx`)
  - `POST /api/explore` extended with `pivottable`, `heatmap`, `sankey`, `sunburst`
    (new SQL builders + result shapes; `dimension2` for column/target/inner-ring).
  - New dependency-free components: `PivotTable`, `CalendarHeatmap`, `SankeyDiagram`
    (2-layer SVG), `SunburstChart` (2-level SVG) in `components/VizComponents.tsx`.
  - `ExploreView` "Chart Builder" gains Pivot / Heatmap / Sankey / Sunburst types and
    a second-dimension control; saved charts keep pinned-chart persistence.

Verified: `go build ./...`, `go vet`, `go test ./internal/...` pass; `npx tsc --noEmit`
on the frontend passes.
