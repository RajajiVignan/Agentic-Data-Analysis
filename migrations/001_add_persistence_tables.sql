-- Migration: Add datasets and data_sources tables for persistence
-- This resolves the "in-memory datasets lost on restart" issue.
-- The Go backend now persists uploaded datasets and connected data sources
-- to Supabase PostgreSQL and reloads them on startup.

CREATE TABLE IF NOT EXISTS datasets (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    file_path TEXT,
    profile JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS data_sources (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    connected_at TIMESTAMPTZ DEFAULT now()
);
