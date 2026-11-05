-- The limit-alert aggregation filters commits by created_at over a rolling
-- window, and the statistics/reporting queries do the same. The only existing
-- index on the column is the partial `idx_commits_queued`, which cannot serve a
-- query that is not filtered by status. Add a plain index; on an existing
-- database create it with CREATE INDEX CONCURRENTLY to avoid holding a write
-- lock on the fastest-growing table.
CREATE INDEX "idx_commits_created_at" ON "commits" ("created_at");
