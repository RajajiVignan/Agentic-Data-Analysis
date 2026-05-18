# Tester Agent Instructions

**First time here?** Read `AGENTS.md` at the project root for the full project quick-reference (architecture, file map, API endpoints, build commands).

Use this role when validating backend behavior, frontend integration, and future agentic AI features.

## Current Test Stack

- Backend tests are Go tests.
- Test files live beside the code:
  - `internal/api/*_test.go`
  - `internal/data/*_test.go`
- The removed root Jest tests should not be restored for the Go backend.
- Frontend tests can be added separately under `frontend/` if the project chooses a frontend test runner.

## Backend Test Command

Use a project-local Go cache so tests work in restricted environments:

```bash
mkdir -p .cache/go-build && GOCACHE=$PWD/.cache/go-build go test ./cmd/server ./internal/...
```

## What To Test

For API work, cover:

- Status code.
- Response JSON shape.
- Error cases.
- Empty list behavior.
- CSV upload.
- JSON upload.
- Missing dataset IDs.
- Invalid multipart upload.
- CSV export content type and rows.
- CORS preflight if routes are changed.

For data processing, cover:

- CSV parsing with quoted commas and escaped quotes.
- JSON arrays and `{ "data": [...] }` / `{ "rows": [...] }`.
- Type inference for number, text, empty, full date, and month values.
- KPI totals and averages.
- Trend aggregation by month.
- Segment aggregation by category.
- Stable ordering where the UI depends on it.

For agentic AI features, cover:

- Deterministic fallback when no LLM key is configured.
- Provider failures and timeouts.
- Invalid LLM JSON.
- Prompt injection attempts.
- Attempts to request secrets or local files.
- Generated chart specs that miss required fields.
- Generated SQL that is not read-only, if SQL is added.

## Manual API Smoke Test

Run the backend:

```bash
go run ./cmd/server
```

Then test:

```bash
curl http://127.0.0.1:3000/api/health
curl -F file=@samples/revenue.csv http://127.0.0.1:3000/api/upload
```

Use the returned `datasetId`:

```bash
curl -H "Content-Type: application/json" \
  -d '{"datasetId":"DATASET_ID","prompt":"What is total revenue by segment?"}' \
  http://127.0.0.1:3000/api/analyze
```

CSV export:

```bash
curl "http://127.0.0.1:3000/api/export/cleaned-csv?datasetIds=DATASET_ID"
```

## Frontend Smoke Test

Start backend first, then:

```bash
cd frontend
npm run dev
```

Check that:

- Upload works.
- Datasets appear.
- Analyze returns KPIs, trend, and segments.
- Segment chart uses `segment` for `samples/revenue.csv`.
- Trend chart uses `month` for `samples/revenue.csv`.
- Export button or export flow works if exposed in the UI.

## Bug Report Format

When reporting failures, include:

- Command used.
- Expected result.
- Actual result.
- Relevant endpoint or file.
- Minimal sample data if data-related.
- Whether the issue is backend, frontend, or agent orchestration.
