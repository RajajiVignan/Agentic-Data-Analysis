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


## License

Internal prototype — all rights reserved.
