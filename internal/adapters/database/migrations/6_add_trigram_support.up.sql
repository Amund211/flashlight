-- Enable pg_trgm extension for trigram similarity matching
-- The extension is installed at the database level, not per-schema
-- Using IF NOT EXISTS to handle parallel test execution gracefully
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Add trigram GIN index for fast similarity searches  
-- This speeds up trigram similarity queries significantly
CREATE INDEX IF NOT EXISTS idx_username_queries_username_trgm 
    ON username_queries USING gin (username gin_trgm_ops);
