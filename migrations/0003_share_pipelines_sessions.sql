-- Persistence for share links, transform pipelines, and chat sessions.

CREATE TABLE IF NOT EXISTS shared_dashboards (
    token TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    charts JSONB NOT NULL DEFAULT '[]'::jsonb,
    url TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS transform_pipelines (
    dataset_id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL DEFAULT '',
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    undone JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS conversation_sessions (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL DEFAULT '',
    dataset_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    history JSONB NOT NULL DEFAULT '[]'::jsonb,
    active_context JSONB,
    created_at TIMESTAMPTZ DEFAULT now(),
    last_active_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_shared_dashboards_owner ON shared_dashboards (owner_id);
CREATE INDEX IF NOT EXISTS idx_shared_dashboards_expires ON shared_dashboards (expires_at);
CREATE INDEX IF NOT EXISTS idx_transform_pipelines_owner ON transform_pipelines (owner_id);
CREATE INDEX IF NOT EXISTS idx_sessions_owner ON conversation_sessions (owner_id);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON conversation_sessions (last_active_at);
