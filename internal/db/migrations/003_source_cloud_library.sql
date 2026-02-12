-- Migration 003: Add cloud_library_id to sources for per-source library mapping
ALTER TABLE sources ADD COLUMN cloud_library_id TEXT;
