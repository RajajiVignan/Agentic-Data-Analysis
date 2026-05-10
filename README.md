# InsightPilot BI App

A full-stack prototype for prompt-driven data analysis.

## Structure

```text
.
├── server.js          # Backend API and local analysis agent
├── samples/           # Sample CSV/JSON files
├── package.json       # Backend package
└── frontend/          # Next.js frontend package
```

There are two `package.json` files because this is currently a small two-package app:

- root `package.json`: runs the backend API
- `frontend/package.json`: runs the Next.js UI

That separation keeps backend dependencies out of the browser app.

## Run

Backend:

```bash
npm start
```

Frontend:

```bash
cd frontend
npm run dev
```

Open the frontend URL printed by Next.js, usually:

```text
http://localhost:3001
```

## API

```text
GET  /api/health
GET  /api/datasets
POST /api/upload
POST /api/analyze
POST /api/connect-source
```

## Current Agent

The current backend agent is local and deterministic, so it works without paid API keys. It:

- infers column types
- selects a likely metric column from the prompt
- creates SQL
- computes KPIs
- builds trend and segment chart data
- writes recommendations

The production version should replace `runAgentAnalysis` in `server.js` with an LLM-backed agent that can generate governed SQL, validate results, and produce richer dashboard specs.
