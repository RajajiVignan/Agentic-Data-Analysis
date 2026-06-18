# InsightPilot — AI-Powered Business Intelligence

<p align="center">
  <b>Full-stack, prompt-driven BI application</b><br>
  Upload your data → Ask questions in plain English → Get KPIs & AI-generated visualizations
</p>

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     InsightPilot                         │
├──────────────────────┬───────────────────────────────────┤
│   Go Backend         │   Next.js Frontend                │
│   (port 3000)        │   (React UI)                      │
├──────────────────────┼───────────────────────────────────┤
│  REST API            │  Upload CSV/JSON                  │
│  CSV/JSON profiling  │  Interactive dashboards           │
│  Deterministic &     │  Metric tiles                     │
│  LLM analyzers       │  Python-generated matplotlib plots│
│  Supabase DB layer   │  Pin / unpin charts               │
│  Python viz bridge   │                                   │
│  Structured logging  │                                   │
│  Rate limiting       │                                   │
└──────────────────────┴───────────────────────────────────┘
```

The backend is a **Go module** — Node.js is only used inside `frontend/` for the UI layer.

---

## Project Structure

```
.
├── cmd/server/main.go              # Go backend entrypoint (port 3000)
│
├── internal/
│   ├── api/
│   │   ├── handler.go              # HTTP handlers, routes, CORS
│   │   ├── routes.go               # Chi router + static frontend serving
│   │   ├── ratelimit.go            # Per-IP per-path sliding-window rate limiter
│   │   ├── pythonbridge.go         # Python viz bridge (executes matplotlib scripts)
│   │   ├── plots.go                # Plot service (LLM + deterministic)
│   │   ├── auth.go                 # JWT auth (register, login, guest sessions)
│   │   ├── analysis.go             # Analysis helpers + execPlan
│   │   ├── errors.go               # Structured API error responses
│   │   ├── query.go                # Custom SQL query mode
│   │   ├── transform.go            # Data transformation pipeline
│   │   ├── joins.go                # Cross-dataset joins
│   │   ├── dashboards.go           # Dashboard CRUD
│   │   ├── dashboard_editor.go     # Drag-and-drop layout editor
│   │   ├── reports.go              # Scheduled reports & alerts
│   │   ├── connections.go          # Data source connections
│   │   ├── share.go                # Shareable dashboard links
│   │   ├── handler_test.go         # API handler tests
│   │   └── *_test.go               # Additional test files
│   ├── agent/
│   │   ├── analyzer.go             # Analyzer interface + request/response structs
│   │   ├── deterministic.go        # Deterministic (no LLM) analyzer
│   │   ├── llm.go                  # LLM-backed analyzer (OpenAI-compatible)
│   │   ├── tools.go                # Agent tools (profile, aggregate, group, trend)
│   │   ├── guardrails.go           # Response validation + sanitization
│   │   └── agent_test.go           # Agent tests
│   ├── data/
│   │   ├── models.go               # Dataset, Profile, Column, Connection structs
│   │   ├── processor.go            # CSV/JSON parsing, profiling, KPI/trend/segment builders
│   │   ├── duckdb.go               # DuckDB engine for SQL queries
│   │   ├── transform.go            # Transform pipeline operations
│   │   └── processor_test.go       # Data processing tests
│   └── store/
│       └── db.go                   # Supabase PostgreSQL persistence (pinned charts, datasets, etc.)
│
├── frontend/
│   ├── src/app/page.tsx            # Main workspace page (upload, analyze, dashboard)
│   └── src/components/
│       ├── Charts.tsx              # MetricTile, PythonPlot
│       ├── DashboardView.tsx       # Dashboard renderer (KPIs + Python plot)
│       ├── Sidebar.tsx             # Navigation sidebar
│       ├── UploadArea.tsx          # File upload + dataset selection
│       ├── AnalysisPrompt.tsx      # Question input + run button
│       ├── PinnedDashboard.tsx     # Pinned charts view
│       └── DataConnections.tsx     # Data source connections
│
├── samples/                        # Sample CSV / JSON datasets
├── uploads/                        # Uploaded user files
├── uploads/plots/                  # Generated Python plot images (.py + .png)
│
├── .env.example                    # Documented environment variable template
├── .golangci.yml                   # Linter configuration
├── .github/workflows/ci.yml        # CI pipeline (lint, vet, test)
├── go.mod                          # Module: insightpilot (deps: godotenv, lib/pq, chi)
└── pinned_charts_schema.sql        # Supabase pinned_charts table schema
```

---

## Quick Start

### Prerequisites

- **Go** 1.25+ — [Install Go](https://go.dev/dl/)
- **Node.js** 18+ — [Install Node](https://nodejs.org/)
- (Optional) Supabase account for DB-persisted data
- (Optional) OpenRouter API key for LLM-powered analysis

### 0. Configure Environment

```bash
cp .env.example .env
# Edit .env with your settings (Supabase, OpenRouter, etc.)
```

The app works fully without Supabase or OpenRouter keys — the deterministic analyzer handles all analysis, and storage falls back to in-memory.

### 1. Start the Backend

```bash
# Run from source
go run ./cmd/server

# Or run the pre-built binary
./server_bin
```

The server defaults to **`http://127.0.0.1:3000`**. Set `PORT` to customize.

### 2. Build the Frontend

```bash
cd frontend
npm install
npm run build
```

The Go server serves the built frontend from `frontend/out/` automatically — no separate dev server needed for production.

For development with hot-reload:

```bash
cd frontend
npm run dev
```

Open the URL printed by Next.js (usually **`http://localhost:3001`**).

---

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check (includes DB connectivity status) |
| `GET` | `/api/datasets` | List all uploaded datasets |
| `POST` | `/api/upload` | Upload a CSV or JSON file |
| `POST` | `/api/analyze` | Run analysis — returns KPIs & plot URL |
| `POST` | `/api/connect-source` | Connect a data source (creates a sample dataset) |
| `GET` | `/api/export/cleaned-csv` | Export a cleaned version of the uploaded CSV |
| `GET` | `/api/pinned-charts` | List all pinned dashboard charts |
| `POST` | `/api/pin-chart` | Pin a chart to the dashboard |
| `DELETE` | `/api/unpin-chart?id=` | Unpin a chart from the dashboard |
| `GET` | `/api/python-plot?datasetId=` | Generate a matplotlib plot on demand |
| `GET` | `/api/dataset/profile` | Get detailed column-level dataset profile |
| `POST` | `/api/query` | Run arbitrary SQL queries via DuckDB |
| `POST` | `/api/join` | Cross-dataset joins |
| `POST` | `/api/transform/preview` | Preview a data transform operation |
| `POST` | `/api/transform/apply` | Apply a data transform pipeline |
| `POST` | `/api/share` | Create a shareable dashboard link |
| `GET` | `/api/reports` | List scheduled reports |
| `POST` | `/api/reports` | Create a scheduled report |
| `GET` | `/api/alerts` | List alert rules |
| `POST` | `/api/alerts` | Create an alert rule |
| `POST` | `/api/auth/register` | Register a new user |
| `POST` | `/api/auth/login` | Log in |
| `GET` | `/plots/{filename}` | Serve a generated plot image |

### Data Flow

```
User uploads CSV/JSON
        │
        ▼
┌──────────────────┐    ┌───────────────────┐
│  Parse & Profile  │───▶│  Store in memory  │
│  (data/processor) │    │  (+ DB if config) │
└──────────────────┘    └───────┬───────────┘
                                │
        User asks a question    ▼
┌──────────────────┐    ┌───────────────────┐
│  Render charts    │◀───│  Analyze request  │
│  (frontend/)      │    │  (deterministic   │
│                   │    │   or LLM agent)   │
└──────────────────┘    └───────┬───────────┘
                                │
                   ┌────────────┴────────────┐
                   ▼                         ▼
          ┌──────────────────┐     ┌──────────────────┐
          │  Python bridge   │     │  DB persistence  │
          │  (matplotlib)    │     │  (Supabase)      │
          └──────────────────┘     └──────────────────┘
```

---

## Running Tests

```bash
go test -v -count=1 -race ./...
```

The CI pipeline (`.github/workflows/ci.yml`) also runs lint (`golangci-lint`), `go vet`, and tests on every push.

---

## Environment Variables

Copy `.env.example` to `.env` and fill in your values. All variables are optional — the app degrades gracefully:

| Variable | Description |
|----------|-------------|
| `PORT` | HTTP port (default: `3000`) |
| `HOST` | Bind address (default: `127.0.0.1`) |
| `CORS_ALLOWED_ORIGINS` | Comma-separated frontend origins |
| `SUPABASE_URL` | Supabase project URL |
| `SUPABASE_KEY` | Supabase publishable/anon key |
| `SUPABASE_DB_PASSWORD` | Supabase database password |
| `OPENROUTER_API_KEY` | API key for LLM analyzer |
| `OPENROUTER_BASE_URL` | OpenRouter base URL |
| `OPENROUTER_MODEL` | Model name (default: `openrouter/owl-alpha`) |
| `OPENROUTER_MAX_TOKENS` | Max tokens (default: `16384`) |
| `OPENROUTER_TEMPERATURE` | Temperature (default: `1.0`) |
| `OPENROUTER_TIMEOUT_SEC` | API timeout (default: `120`) |
| `PLOT_RETENTION_HOURS` | Plot artifact cleanup (default: `24`, `0` disables) |
| `QUERY_TIMEOUT_SEC` | DuckDB SQL query timeout (default: `30`) |
| `REFRESH_INTERVAL_MIN` | Live-db dataset refresh interval (default: `15`) |
| `UPLOAD_DIR` | Upload directory (default: `<project>/uploads`) |
| `SMTP_HOST` | SMTP server for email delivery |
| `SMTP_PORT` | SMTP port (default: `587`) |
| `SMTP_USER` | SMTP username |
| `SMTP_PASSWORD` | SMTP password |
| `SMTP_FROM` | From address (defaults to `SMTP_USER`) |
| `ALERT_COOLDOWN_MIN` | Alert cooldown (default: `60`) |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` (default: `info`) |
| `LOG_FORMAT` | Log format: `text` or `json` (default: `text`, `json` in production) |

---

## Key Design Patterns

| Pattern | Description |
|---------|-------------|
| **Analyzer interface** | `internal/agent/analyzer.go` — `Analyze(ctx, req) (resp, err)` — swappable deterministic / LLM implementations |
| **LLM tool-call loop** | LLM can call `get_dataset_profile`, `aggregate_metric`, `group_by_dimension`, `build_trend` tools before producing a final plan |
| **Privacy-safe LLM** | Only dataset metadata (schema + statistics) is sent to the LLM — never raw row data |
| **Graceful fallback** | LLM → deterministic analyzer → in-memory pin storage → Supabase; never breaks without paid keys |
| **Python bridge** | Generates self-contained matplotlib scripts → executes `python3` → serves `/plots/*.png` |
| **Structured logging** | Uses `log/slog` with component attributes; text output in dev, JSON in production |
| **Rate limiting** | Per-IP per-path sliding-window rate limiter on auth endpoints |
| **Request size limits** | All JSON request bodies capped at 1 MB; file uploads capped at 10 MB |
| **Thread-safe state** | All handler maps use `sync.RWMutex` for safe concurrent access |
| **Single-server deploy** | Go serves API, plots, and the built Next.js static export on one port |

---

## Known Issues & Tech Debt

- ~~Export handler uses `strings.Fields` + `Join` which corrupts data containing spaces~~ (fixed)
- In-memory datasets are lost on server restart when no DB is configured
- LLM analyzer falls back to deterministic without a valid API key

---

## Roadmap / Milestones

- [ ] Migrate in-memory datasets to DB persistence by default
- [ ] Add governed SQL generation via LLM agent
- [ ] Implement richer dashboard spec output
- [ ] Background workers for Python plot generation and heavy analysis

---

## License

Internal prototype — all rights reserved.
