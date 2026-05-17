# Python Visualization Agent Instructions

Use this role only when Python is intentionally added for offline analysis, prototype chart generation, or notebook-style experiments.

The production backend is Go and the production frontend is Next.js. Do not make Python a required runtime dependency for the main app unless the project owner explicitly chooses that architecture.

## Appropriate Uses

Python is useful for:

- Exploring sample datasets.
- Prototyping chart logic before implementing it in Go or the frontend.
- Producing one-off static visualizations under `plots/`.
- Validating statistical ideas for future agentic analysis.
- Comparing outputs from the Go aggregation logic against pandas prototypes.

## Avoid

- Do not add Python web servers for the main API.
- Do not route frontend requests through Python scripts.
- Do not make `go run ./cmd/server` depend on Python.
- Do not store generated plots as required application assets unless the UI actually needs them.
- Do not commit large generated files.

## Suggested Workflow

1. Read data from `samples/` or a copied test fixture.
2. Prototype analysis in a small script or notebook.
3. Record the useful logic in plain English.
4. Re-implement production logic in Go or frontend TypeScript.
5. Add Go tests for the production behavior.

## Visualization Expectations

For BI-style charts:

- Prefer clear labels over decorative styling.
- Make chart types match the data:
  - Line or area for trends over time.
  - Bar for category comparisons.
  - KPI cards for top-level metrics.
  - Scatter only when comparing two numeric measures.
- Always document the aggregation used: sum, average, count, min, max, or percentage.
- Keep color choices accessible and not dependent on a single hue.

## Agentic AI Visualization Direction

Future LLM-generated chart specs should be validated before rendering. A chart spec should include:

- chart type
- title
- x field
- y field
- aggregation
- source dataset ID
- explanation

Reject or repair chart specs that:

- reference missing columns
- aggregate text as a number
- use date fields as metrics
- omit required fields
- produce misleading chart types
