CREATE TABLE IF NOT EXISTS pinned_charts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ DEFAULT now(),
    chart_type TEXT NOT NULL, -- 'kpi', 'trend', 'segment', 'python_plot'
    label TEXT,
    data JSONB,
    url TEXT
);
