-- 002_cluster_logs_enterprise.sql
-- Enterprise logging additions: Distributed trace correlation, error deduplication fingerprints,
-- and bloom filter indexes for substring search acceleration.

ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS trace_id LowCardinality(String);
ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS span_id String;
ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS error_fingerprint LowCardinality(String);
ALTER TABLE cluster_logs ADD INDEX IF NOT EXISTS idx_ngram message TYPE ngrambf_v1(4, 32768, 2, 0) GRANULARITY 1;
ALTER TABLE cluster_logs ADD INDEX IF NOT EXISTS idx_trace trace_id TYPE bloom_filter(0.01) GRANULARITY 1;
