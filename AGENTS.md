# InsightPilot — Project Quick Reference

## What It Is
Prompt-driven BI application. Go backend + Next.js frontend. User uploads CSV/JSON data, asks questions in natural language, gets back KPIs, charts, and AI-generated visualizations.

## Architecture at a Glance

```
.
├── cmd/server/main.go              # Go backend entrypoint (port 3000)
├── internal/
│   ├── api/
│   │   ├── handler.go              # HTTP handlers, routes, CORS
│   │   ├── routes.go               # Route registration
│   │   ├── pythonbridge.go         # Python viz bridge (matplotlib/bokeh/plotly)
│   │   ├── analysis.go             # Analysis endpoint logic
│   │   ├── auth.go                 # Auth endpoints (login, register, guest)
│   │   ├── connections.go          # DB connection endpoints
│   │   ├── dashboards.go           # Dashboard CRUD endpoints
│   │   ├── dashboard_editor.go     # Dashboard layout persistence
│   │   ├── pinned.go               # Pin/unpin chart endpoints
│   │   ├── plots.go                # Plot regeneration endpoint
│   │   ├── transform.go            # ETL pipeline endpoint
│   │   ├── joins.go                # Cross-dataset join endpoint
│   │   ├── query.go                # Arbitrary SQL query endpoint
│   │   ├── share.go                # Share link generation
│   │   ├── reports.go              # Scheduled reports & alerts
│   │   ├── ratelimit.go            # Per-IP rate limiting
│   │   ├── sandbox_validator.go    # Generated code sandbox checks
│   │   ├── crypto.go / uuid.go     # Helpers
│   │   └── errors.go               # Structured API errors
│   ├── agent/
│   │   ├── analyzer.go             # Analyzer interface + request/response structs
│   │   ├── deterministic.go        # Deterministic (no LLM) analyzer
│   │   ├── llm.go                  # OpenRouter LLM analyzer
│   │   ├── planner.go              # Multi-step analysis planning
│   │   ├── session.go              # Conversation session management
│   │   ├── tools.go                # Agent tools (profile, agg, group, trend)
│   │   ├── guardrails.go           # Response validation + sanitization
│   │   ├── feedback.go             # User feedback ingestion
│   │   └── validator.go            # Semantic validation
│   ├── data/
│   │   ├── models.go               # Dataset, Profile, Column, Connection structs
│   │   ├── processor.go            # CSV/JSON parsing, KPI/trend/segment builders
│   │   ├── profiler.go             # Advanced column profiling
│   │   ├── duckdb.go               # Direct DuckDB SQL execution
│   │   ├── transform.go            # ETL operations (filter, rename, derive, etc.)
│   │   └── store/
│   │       └── db.go               # Supabase PostgreSQL persistence
├── frontend/
│   ├── src/app/page.tsx            # Main workspace (upload, analyze, sidebar, 10+ tabs)
│   ├── src/components/             # 19 components (Charts, Sidebar, DashboardView,
│   │                               #   DataConnections, SchemaDesigner, SQLQueryEditor,
│   │                               #   DashboardEditor, ScheduleManager, VizWidget, ...)
│   └── src/lib/
│       ├── api.ts                  # All API functions + types (793 lines)
│       └── export.ts               # PNG/JPEG/PDF export utilities
├── uploads/                        # Uploaded files
├── uploads/plots/                  # Generated Python plot images (.py + .png)
├── .env                            # Supabase + OpenRouter + SMTP config
├── server_bin                      # Pre-built Go binary
├── go.mod                          # Module: insightpilot
└── pinned_charts_schema.sql        # Supabase pinned_charts table schema
```

## Key API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | /api/health | Health check (reports DB status) |
| GET | /api/datasets | List uploaded datasets |
| POST | /api/upload | Upload CSV/JSON file |
| POST | /api/analyze | Run analysis (returns KPIs, trend, segments, plotUrl) |
| POST | /api/connect-source | Connect a data source (creates sample dataset) |
| GET | /api/export/cleaned-csv | Export cleaned CSV |
| GET | /api/pinned-charts | List pinned charts |
| POST | /api/pin-chart | Pin a chart to dashboard |
| DELETE | /api/unpin-chart?id= | Unpin a chart |
| GET | /api/python-plot?datasetId= | Generate Python plot on demand |
| GET | /plots/{filename} | Serve generated plot images |

## Data Flow

1. User uploads CSV -> `handleUpload` parses it, stores in `h.datasets` map (in-memory)
2. User asks question -> `handleAnalyze` runs deterministic or LLM analyzer
3. Analyzer returns KPIs, trend, segments, recommendations
4. If dataset has FilePath, Python bridge auto-generates a matplotlib plot
5. Frontend renders: MetricTiles, PythonPlot image, TrendChart (bar), SegmentChart (pie)
6. User can pin charts -> stored in memory + Supabase PostgreSQL

## Key Patterns

- **Analyzer interface**: `internal/agent/analyzer.go` — `Analyze(ctx, req) (resp, err)`
- **Deterministic fallback**: Always works without LLM key; selects columns by type + prompt keywords
- **Python bridge**: Generates self-contained .py script -> exec `python3` -> serves /plots/*.png
- **DB persistence**: `internal/store/db.go` — graceful fallback to in-memory if Supabase unavailable
- **ID generation**: Uses `time.Now().UnixNano()` (not `os.Getpid()` which collides)
- **Concurrency**: Handler uses `sync.RWMutex` for thread-safe map access

## Build & Test

```bash
# Backend
go build -o server_bin ./cmd/server
go test ./...

# Frontend
cd frontend && npm run dev
cd frontend && npm run build
```

## Environment Variables (.env)

- `SUPABASE_URL` — e.g. `https://xxxx.supabase.co`
- `SUPABASE_KEY` — publishable/anon key
- `SUPABASE_DB_PASSWORD` — DB password (falls back to SUPABASE_KEY)
- `OPENROUTER_API_KEY` — LLM API key (optional, falls back to deterministic)
- `OPENROUTER_BASE_URL` — e.g. `https://openrouter.ai/api/v1`
- `OPENROUTER_MODEL` — model name (default: openrouter/owl-alpha)
- `OPENROUTER_MAX_TOKENS` — max tokens (default: 16384)
- `OPENROUTER_TEMPERATURE` — temperature (default: 1.0)
- `OPENROUTER_TIMEOUT_SEC` — HTTP client timeout (default: 120)
- `PORT` — default 3000
- `HOST` — default 127.0.0.1
- `CORS_ALLOWED_ORIGINS` — comma-separated frontend origins for browser API access
- `PLOT_RETENTION_HOURS` — hours to keep generated `.py`/`.png` plot artifacts; default 24, 0 disables cleanup
- `UPLOAD_DIR` — upload directory; default resolves to `<project-root>/uploads`
- `SMTP_HOST` — SMTP server hostname for email delivery (required for scheduled reports & alerts)
- `SMTP_PORT` — SMTP server port (default `587`)
- `SMTP_USER` — SMTP username (also used as `From` address if `SMTP_FROM` not set)
- `SMTP_PASSWORD` — SMTP password
- `SMTP_FROM` — From email address (defaults to `SMTP_USER`)
- `QUERY_TIMEOUT_SEC` — timeout for arbitrary SQL queries via DuckDB (default `30`)
- `REFRESH_INTERVAL_MIN` — interval for live-db dataset refresh in minutes (default `15`)

## Known Issues / Tech Debt

- Reports, alerts, and dashboard layouts persist to Supabase when configured, with in-memory fallback
- LLM analyzer returns 401 without valid OPENROUTER_API_KEY (falls back to deterministic)

## Token Efficiency Rules

These rules apply to all sessions to minimize context/token usage:

1. **Prefers `grep` + `glob` over `read`** — search first, read targeted line ranges only
2. **Use `task` sub-agent for exploration** — delegate open-ended codebase exploration to sub-agents
3. **Batch all independent tool calls** into a single message
4. **Never read entire files** unless the task absolutely requires it; use offset/limit
5. **Avoid full-file writes** when an `edit` suffices
6. **Keep responses under 4 lines** unless the user explicitly asks for detail
7. **Use `bash` with `rg` for precise content searches** instead of reading full files
