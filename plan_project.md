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

---

## 7. Current State Review — October 2026

The feature surface is now broad (uploads, transforms, joins, SQL Lab, scheduler,
semantic layer, no-code builder, caching, async jobs). The remaining risk is
**productization**: isolation, security, persistence, and operability.

### 7.1 Prioritized improvements

| # | Area | Severity | Issue | Where |
|---|------|----------|-------|-------|
| I-1 | Auth / multi-tenancy | Critical | Auth middleware attaches a user but never rejects; most routes are unauthenticated | `internal/api/auth.go:238` |
| I-2 | Auth / multi-tenancy | Critical | Only datasets are owner-scoped; dashboards, connections, pinned charts, saved queries, reports, glossary have no owner filter | `internal/api/{dashboards,connections,pinned,sql_lab,reports,glossary}.go` |
| I-3 | Persistence | High | Datasets, connections, sessions, transform pipelines, share tokens are in-memory and lost on restart | `internal/api/handler.go:31-55`, `share.go:19` |
| I-4 | Sandbox | High | LLM-generated Python is executed directly via `python3`; regex blacklist is bypassable | `internal/api/pythonbridge.go:158`, `sandbox_validator.go` |
| I-5 | Ops | High | No CI, Dockerfile/Compose, or Makefile | repo root |
| I-6 | Ops | High | No versioned migrations; schema is ad hoc `CREATE TABLE IF NOT EXISTS` | `internal/store/db.go` |
| I-7 | Observability | Medium | No metrics, tracing, request IDs, or LLM token/cost counters; only logs + `/api/health` | `internal/api/routes.go`, `cmd/server/main.go` |
| I-8 | Maintainability | Medium | Monolithic UI shell and API client (1000+ lines each), tab-boolean state instead of a router/query layer | `frontend/src/app/page.tsx`, `frontend/src/lib/api.ts` |
| I-9 | Reliability | Medium | No backpressure/queue for analysis; uploads buffer fully in memory | `internal/api/handler.go`, `analysis.go` |
| I-10 | Quality | Medium | Playwright specs exist but are not wired to CI; no load/bench tests | `frontend/tests/`, `internal/*/*_test.go` |
| I-11 | Docs | Low | `AGENTS.md` / `README.md` still describe Next.js; app is Vite + React 19 | `AGENTS.md`, `README.md` |

#### I-1 / I-2 — Multi-user isolation (do first)
- Make `authMiddleware` reject unauthenticated requests on mutating and
  tenant-scoped endpoints; keep only upload/share/health/guest-login public.
- Add `owner_id` to dashboards, dashboard layouts, connection configs, pinned
  charts, saved queries, reports, alerts, glossary, and dataset fields.
- Every list/get/update/delete handler filters by `currentUserID(r)`.
- Add RBAC (`viewer` / `editor` / `admin`) and dataset-level sharing.

#### I-3 — Persistence
- Persist share tokens (currently `ShareService` map) and transform pipelines.
- Promote dataset rows from RAM to per-dataset DuckDB files (metadata already
  persists in the `datasets` table).
- Move conversation sessions to DB or make them explicitly ephemeral.

#### I-4 — Real sandbox
- Run generated Python inside a locked-down container or sandbox
  (gVisor/firejail), non-root, no network, read-only FS, only `/plots` writable.
- Or restrict the LLM to selecting/parameterizing pre-approved templates.
- Keep the regex validator as defense-in-depth, not the primary control.

#### I-5 / I-6 / I-7 — Operability
- Add GitHub Actions: `go build`, `go vet`, `go test ./...`, `npm run lint`,
  `tsc --noEmit`, Playwright smoke.
- Add `Dockerfile` + `docker-compose.yml` (Go + frontend build + optional Postgres
  + Python runtime).
- Introduce numbered SQL migrations with a `schema_migrations` table.
- Add Prometheus `/metrics`, structured request IDs, and LLM usage counters.

#### I-8 / I-9 / I-10 / I-11 — Maintainability
- Split `page.tsx` into routed views; introduce TanStack Query for server state.
- Split `lib/api.ts` into domain modules; generate types from an OpenAPI spec.
- Add analysis queue/backpressure and stream large uploads to disk.
- Add load tests and wire E2E into CI.
- Fix the Next.js → Vite/React documentation drift.

### 7.2 Feature additions (next tier)

| Area | Features |
|------|----------|
| Teams | RBAC, workspaces, dataset-level permissions, row-level security |
| Alerting | Slack/Teams/webhook sinks, anomaly rules (z-score/IQR) with LLM explanations |
| Dashboards | Native filter widgets, tabs, markdown tiles, scheduled PDF snapshots, signed iframe embeds |
| Visualization | Geo/map, calendar heatmap, sankey, sunburst, Excel/PPT export |
| AI | Multi-provider + local Ollama/BYO key, NL data cleaning, what-if analysis, narrative summaries |
| Platform | OpenAPI spec + API keys, webhook ingestion, dataset versioning/materialized views, incremental refresh |
| UX | Dark-mode persistence, mobile layout, collaborative chart comments |

### 7.3 Recommended execution order

1. **I-1 + I-2** — enforce auth and owner-scope every resource (security blocker). **DONE** — routes split into public/read/editor groups; `owner_id` + role added across datasets, dashboards, layouts, connections, pinned charts, saved queries, reports, alerts, glossary, semantic fields; admin/editor/viewer roles enforced.
2. **I-3** — persist share tokens and pipelines; DuckDB-backed datasets.
3. **I-5 + I-6** — CI and migrations (prevents regressions while scaling).
4. **I-4** — containerized Python sandbox.
5. **I-7** — metrics/tracing and LLM cost tracking.
6. **I-8 + I-9 + I-10** — frontend decomposition, queueing, load/E2E coverage.
7. Feature tier in §7.2, starting with webhook/alert channels and embeddable dashboards.

### 7.4 Do not re-plan (already implemented)

Transforms (filter/rename/derive/join), cross-dataset joins, SQL Lab (history,
saved queries, async), scheduled reports/alerts (SMTP), profiler, semantic
fields, no-code chart builder, result caching, and expanded viz types are already
shipped per §6. Build on them rather than duplicating.
