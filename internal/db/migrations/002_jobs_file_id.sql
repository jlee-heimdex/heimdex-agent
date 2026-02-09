-- Migration 002: Add file_id column to jobs for per-file index jobs
ALTER TABLE jobs ADD COLUMN file_id TEXT REFERENCES files(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_file ON jobs(file_id);
