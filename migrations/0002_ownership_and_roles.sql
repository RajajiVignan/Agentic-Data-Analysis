-- Owner scoping for tenant resources plus RBAC roles.
-- ALTER ... IF NOT EXISTS keeps this safe for databases created before
-- versioned migrations were introduced.

ALTER TABLE pinned_charts ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboards ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE datasets ADD COLUMN IF NOT EXISTS owner_id TEXT DEFAULT '';
ALTER TABLE scheduled_reports ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_layouts ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE glossary_defs ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE dataset_fields ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE saved_queries ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'editor';

CREATE INDEX IF NOT EXISTS idx_pinned_charts_owner ON pinned_charts (owner_id);
CREATE INDEX IF NOT EXISTS idx_dashboards_owner ON dashboards (owner_id);
CREATE INDEX IF NOT EXISTS idx_datasets_owner ON datasets (owner_id);
CREATE INDEX IF NOT EXISTS idx_reports_owner ON scheduled_reports (owner_id);
CREATE INDEX IF NOT EXISTS idx_alerts_owner ON alert_rules (owner_id);
CREATE INDEX IF NOT EXISTS idx_layouts_owner ON dashboard_layouts (owner_id);
CREATE INDEX IF NOT EXISTS idx_glossary_owner ON glossary_defs (owner_id);
CREATE INDEX IF NOT EXISTS idx_dataset_fields_owner ON dataset_fields (owner_id);
CREATE INDEX IF NOT EXISTS idx_dataset_fields_dataset ON dataset_fields (dataset_id);
CREATE INDEX IF NOT EXISTS idx_saved_queries_owner ON saved_queries (owner_id);
