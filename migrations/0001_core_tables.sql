-- Baseline schema for InsightPilot.

CREATE TABLE IF NOT EXISTS pinned_charts (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT now(),
    chart_type TEXT NOT NULL,
    label TEXT,
    data JSONB,
    url TEXT
);

CREATE TABLE IF NOT EXISTS dashboards (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    chart_ids JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS datasets (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    file_path TEXT DEFAULT '',
    profile JSONB DEFAULT '{}'::jsonb,
    rows_data JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scheduled_reports (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    dataset_ids TEXT NOT NULL DEFAULT '[]',
    frequency TEXT NOT NULL DEFAULT 'daily',
    day_of_week INTEGER NOT NULL DEFAULT 0,
    day_of_month INTEGER NOT NULL DEFAULT 1,
    hour INTEGER NOT NULL DEFAULT 9,
    emails TEXT NOT NULL DEFAULT '[]',
    slack_webhook TEXT DEFAULT '',
    teams_webhook TEXT DEFAULT '',
    last_sent TEXT DEFAULT '',
    next_run TEXT DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    metric_col TEXT NOT NULL,
    condition TEXT NOT NULL DEFAULT 'drop',
    threshold DOUBLE PRECISION NOT NULL DEFAULT 10,
    period TEXT DEFAULT '',
    emails TEXT NOT NULL DEFAULT '[]',
    slack_hook TEXT DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_checked TEXT DEFAULT '',
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS dashboard_layouts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    tiles JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS glossary_defs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    expression TEXT DEFAULT '',
    description TEXT DEFAULT '',
    dataset_id TEXT DEFAULT '',
    return_type TEXT DEFAULT 'number',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS dataset_fields (
    id TEXT PRIMARY KEY,
    dataset_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'metric',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS saved_queries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    dataset_ids TEXT NOT NULL DEFAULT '[]',
    sql TEXT NOT NULL,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
