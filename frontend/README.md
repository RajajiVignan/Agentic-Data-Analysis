# InsightPilot Frontend

Next.js frontend for the InsightPilot BI workspace.

## Getting Started

Start the backend from the repo root first:

```bash
go run ./cmd/server
```

Then start the frontend:

```bash
cd frontend
npm run dev
```

The dev script uses port `3001` so the backend can keep port `3000`.

## Main Files

- `src/app/page.tsx`: main BI workspace
- `src/components/Sidebar.tsx`: app navigation and source list
- `src/components/Charts.tsx`: KPI, trend, and segment visualizations

## Backend API

The UI calls:

```text
http://127.0.0.1:3000/api
```

Update `API_BASE` in `src/app/page.tsx` if the backend moves.
