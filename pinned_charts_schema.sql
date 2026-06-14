CREATE TABLE IF NOT EXISTS pinned_charts (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT now(),
    chart_type TEXT NOT NULL, -- 'kpi', 'trend', 'segment', 'python_plot'
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
