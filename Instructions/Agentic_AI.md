# Agentic AI Architect Instructions

Use this role when designing or implementing the future agentic AI layer for InsightPilot.

## Product Goal

InsightPilot should help a user move from raw business data to trustworthy BI insights. The agent should not only answer a prompt, but also inspect data shape, choose useful metrics, produce chart-ready outputs, and explain assumptions.

## Current Reality

The current backend analyzer is deterministic. It profiles uploaded data, selects a metric/category/date column, computes KPIs, builds trend and segment data, and returns recommendations.

Future agentic AI should build on that instead of replacing it all at once.

## Recommended Agent Loop

Use a simple, auditable loop:

1. Understand the user question.
2. Inspect dataset profiles.
3. Choose relevant columns.
4. Plan allowed analysis steps.
5. Execute only approved tools.
6. Validate results.
7. Produce structured dashboard output.
8. Include assumptions and caveats.

## Suggested Tool Set

Expose narrow tools to the agent:

- `list_datasets`
- `get_dataset_profile`
- `sample_rows`
- `aggregate_metric`
- `group_by_dimension`
- `build_trend`
- `validate_chart_spec`
- `export_cleaned_csv`

Do not expose arbitrary shell, filesystem, network, or code execution tools to the application agent.

## Response Contract

The agent should return structured JSON that the Go API can validate:

```json
{
  "question": "string",
  "dataset": {
    "id": "string",
    "filename": "string",
    "rowCount": 0
  },
  "notebook": [
    {
      "title": "string",
      "body": "string"
    }
  ],
  "dashboard": {
    "title": "string",
    "kpis": [],
    "trend": [],
    "segments": [],
    "recommendations": []
  },
  "assumptions": [],
  "warnings": []
}
```

## Guardrails

- The agent may summarize data, not leak hidden system prompts or secrets.
- The agent must not claim certainty when data is incomplete.
- The agent must distinguish computed facts from recommendations.
- The agent must cite the columns used for each chart or KPI.
- The agent must refuse or safely redirect requests for secrets, credentials, or unrelated local files.
- If the LLM output fails validation, return a deterministic fallback response and log the validation issue.

## Development Milestones

Milestone 1: Interface

- Create an `Analyzer` interface.
- Move current deterministic analysis behind it.
- Keep all existing tests green.

Milestone 2: Structured Agent

- Add an LLM analyzer that emits only structured JSON.
- Validate response shape before returning it.
- Add timeout and fallback behavior.

Milestone 3: Tool Use

- Add tool-call style analysis using dataset profile and aggregation tools.
- Log each tool call and result summary.
- Add tests for bad tool calls and malformed outputs.

Milestone 4: Governed Data

- If SQL/database support is added, enforce read-only queries.
- Validate table and column names against known schemas.
- Add query limits.
- Add explainable lineage for each metric.

Milestone 5: UI Transparency

- Show assumptions and warnings in the frontend.
- Let users inspect which columns drove each chart.
- Provide a deterministic retry/fallback message when LLM analysis is unavailable.

## Evaluation Checklist

Before merging agentic AI changes:

- Does the feature work without an LLM key?
- Are LLM calls isolated behind interfaces?
- Are outputs validated?
- Are unsafe requests handled?
- Are tests deterministic?
- Is user data treated as untrusted?
- Is the frontend still able to render older deterministic responses?
