# InsightPilot BI App

A full-stack prototype for prompt-driven data analysis.

## Structure

```text
.
├── cmd/server/        # Go backend entrypoint
├── internal/api/      # HTTP handlers and routes
├── internal/data/     # CSV/JSON parsing, profiling, and chart data helpers
├── samples/           # Sample CSV/JSON files
├── go.mod             # Go backend module
└── frontend/          # Next.js frontend package
```

The backend is now a Go module. Node is only used inside `frontend/` for the Next.js UI.

## Run

### 1. Start the Backend
The backend is written in Go and provides the API for data processing and analysis.

```bash
# Run from source
go run ./cmd/server

# Or run the checked-in server binary
./server_bin
```

The backend defaults to `http://127.0.0.1:3000`. To use another port:

```bash
PORT=3001 go run ./cmd/server
```

### 2. Start the Frontend
The frontend is a Next.js application.

```bash
cd frontend
npm install
npm run dev
```

Open the frontend URL printed by Next.js, usually:
```text
http://localhost:3000
```

## API

```text
GET  /api/health
GET  /api/datasets
POST /api/upload
POST /api/analyze
POST /api/connect-source
GET  /api/export/cleaned-csv
GET  /api/pinned-charts
POST /api/pin-chart
```

## Tests

The active backend tests are Go tests:

```bash
go test ./cmd/server ./internal/...
```

## Current Agent

The current backend agent is local and deterministic, so it works without paid API keys. It:

- infers column types
- selects a likely metric column from the prompt
- computes KPIs
- builds trend and segment chart data
- writes recommendations

The production version should replace the deterministic analysis logic in `internal/api/handler.go` with an LLM-backed agent that can generate governed SQL, validate results, and produce richer dashboard specs.
