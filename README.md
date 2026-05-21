# ✨ InsightPilot — AI-Powered Business Intelligence

<p align="center">
  <b>Full-stack, prompt-driven BI application</b><br>
  Upload your data → Ask questions in plain English → Get KPIs, charts & AI visualizations
</p>

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     InsightPilot                         │
├──────────────────────┬───────────────────────────────────┤
│   Go Backend         │   Next.js Frontend                │
│   (port 3000)        │   (React UI)                      │
├──────────────────────┼───────────────────────────────────┤
│  REST API            │  Upload CSV/JSON                  │
│  CSV/JSON profiling  │  Interactive dashboards           │
│  Deterministic &     │  Metric tiles, trends, pie charts │
│  LLM analyzers       │  Python-generated matplotlib plots│
│  Supabase DB layer   │  Pin / unpin charts               │
│  Python viz bridge   │                                   │
└──────────────────────┴───────────────────────────────────┘
```

The backend is a **Go module** — Node.js is only used inside `frontend/` for the UI layer.

---

## 📁 Project Structure

```
.
├── cmd/server/main.go              # Go backend entrypoint (port 3000)
│
├── internal/
│   ├── api/
│   │   ├── handler.go              # HTTP handlers, routes, CORS
│   │   ├── pythonbridge.go         # Python viz bridge (executes matplotlib scripts)
│   │   └── handler_test.go         # API handler tests
│   ├── agent/
│   │   ├── analyzer.go             # Analyzer interface + request/response structs
│   │   ├── deterministic.go        # Deterministic (no LLM) analyzer
│   │   ├── llm.go                  # NVIDIA NIM LLM-backed analyzer
│   │   ├── tools.go                # Agent tools (profile, aggregate, group, trend)
│   │   ├── guardrails.go           # Response validation + sanitization
│   │   └── agent_test.go           # Agent tests
│   ├── data/
│   │   ├── models.go               # Dataset, Profile, Column, Connection structs
│   │   ├── processor.go            # CSV/JSON parsing, profiling, KPI/trend/segment builders
│   │   └── processor_test.go       # Data processing tests
│   └── store/
│       └── db.go                   # Supabase PostgreSQL persistence (pinned charts)
│
├── frontend/
│   ├── src/app/page.tsx            # Main workspace page (upload, analyze, dashboard)
│   └── src/components/
│       ├── Charts.tsx              # MetricTile, TrendChart, SegmentChart, PythonPlot
│       └── Sidebar.tsx             # Navigation sidebar
│
├── samples/                        # Sample CSV / JSON datasets
├── uploads/                        # Uploaded user files
├── uploads/plots/                  # Generated Python plot images (.py + .png)
│
├── .env                            # Environment configuration
├── server_bin                      # Pre-built Go binary
├── go.mod                          # Module: insightpilot (deps: godotenv, lib/pq)
└── pinned_charts_schema.sql        # Supabase pinned_charts table schema
```

---

## 🚀 Quick Start

### Prerequisites

- **Go** 1.21+ — [Install Go](https://go.dev/dl/)
- **Node.js** 18+ — [Install Node](https://nodejs.org/)
- (Optional) Supabase account for DB-persisted pinned charts
- (Optional) NVIDIA NIM API key for LLM-powered analysis

### 1. Start the Backend

```bash
# Option A: Run from source
go run ./cmd/server

# Option B: Run the pre-built binary
./server_bin
```

The server defaults to **`http://127.0.0.1:3000`**. To use a custom port:

```bash
PORT=3001 ./server_bin
```

### 2. Start the Frontend

```bash
cd frontend
npm install
npm run dev
```

Open the URL printed by Next.js (usually **`http://localhost:3001`**).

---

## 🔌 API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check (includes DB connectivity status) |
| `GET` | `/api/datasets` | List all uploaded datasets |
| `POST` | `/api/upload` | Upload a CSV or JSON file |
| `POST` | `/api/analyze` | Run analysis — returns KPIs, trends, segments & plot URL |
| `POST` | `/api/connect-source` | Connect a data source (creates a sample dataset) |
| `GET` | `/api/export/cleaned-csv` | Export a cleaned version of the uploaded CSV |
| `GET` | `/api/pinned-charts` | List all pinned dashboard charts |
| `POST` | `/api/pin-chart` | Pin a chart to the dashboard |
| `DELETE` | `/api/unpin-chart?id=` | Unpin a chart from the dashboard |
| `GET` | `/api/python-plot?datasetId=` | Generate a matplotlib plot on demand |
| `GET` | `/plots/{filename}` | Serve a generated plot image |

### 📊 Data Flow

```
User uploads CSV/JSON
        │
        ▼
┌──────────────────┐    ┌───────────────────┐
│  Parse & Profile  │───▶│  Store in memory  │
│  (data/processor) │    │  (thread-safe)    │
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
          │  Python bridge   │     │  Pin to Supabase │
          │  (matplotlib)    │     │  (DB persistence)│
          └──────────────────┘     └──────────────────┘
```

---

## 🧪 Running Tests

```bash
# Backend Go tests (all packages)
go test ./cmd/server ./internal/...
```

---

## 🔧 Environment Variables

Create a `.env` file in the project root:

```env
# Supabase (optional — graceful in-memory fallback)
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_KEY=your-publishable-anon-key
SUPABASE_DB_PASSWORD=your-db-password

# NVIDIA NIM LLM (optional — graceful deterministic fallback)
NVIDIA_API_KEY=your-nvidia-api-key
NVIDIA_BASE_URL=https://api.nvcf.nvidia.com/v2/nvcf

# Server config
PORT=3000
HOST=127.0.0.1
```

> **Note:** The app works fully without Supabase or NVIDIA keys — the deterministic analyzer handles all analysis tasks, and chart pinning falls back to in-memory storage.

---

## 🧩 Key Design Patterns

| Pattern | Description |
|---------|-------------|
| **Analyzer interface** | `internal/agent/analyzer.go` — `Analyze(ctx, req) (resp, err)` — swappable deterministic / LLM implementations |
| **Graceful fallback** | LLM → deterministic analyzer → in-memory pin storage → Supabase; never breaks without paid keys |
| **Python bridge** | Generates self-contained matplotlib scripts → executes `python3` → serves `/plots/*.png` |
| **Thread-safe state** | All handler maps use `sync.RWMutex` for safe concurrent access |
| **Nano-second IDs** | Uses `time.Now().UnixNano()` to avoid PID-based collisions |

---

## 🐛 Known Issues & Tech Debt

- Export handler uses `strings.Fields` + `Join` which corrupts data containing spaces
- In-memory datasets and connections are lost on server restart (no DB persistence yet)
- LLM analyzer returns `401` without a valid `NVIDIA_API_KEY` (falls back to deterministic)
- Generated Python scripts in `uploads/plots/` are not cleaned up after execution
- Frontend port conflicts with backend when both default to `3000`

---

## 📋 Roadmap / Milestones

- [ ] Migrate in-memory datasets to DB persistence
- [ ] Add governed SQL generation via LLM agent
- [ ] Implement richer dashboard spec output
- [ ] Add agentic AI multi-step analysis (see `Instructions/Agentic_AI.md`)
- [ ] Improve Python visualization templates (see `Instructions/Python_Visualizations.md`)

---

## 📄 License

Internal prototype — all rights reserved.

---

> 💡 **Tip:** Check the `Instructions/` directory for role-specific guides (Developer, Tester, Agentic AI, Python Visualizations).
