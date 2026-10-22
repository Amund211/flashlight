-- Create GIN index on username for fast trigram similarity searches
-- Note: pg_trgm extension is created by the migrator before running migrations
CREATE INDEX IF NOT EXISTS idx_usernames_username_trgm ON usernames USING gin (username gin_trgm_ops);
