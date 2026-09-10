CREATE TABLE IF NOT EXISTS cluster_logs (
    timestamp DateTime64(3, 'UTC') CODEC(DoubleDelta, ZSTD(1)),
    tenant_id LowCardinality(String),
    cluster_id LowCardinality(String),
    namespace LowCardinality(String),
    pod_name LowCardinality(String),
    container_name LowCardinality(String),
    stream LowCardinality(String),
    log_level LowCardinality(String),
    message String CODEC(ZSTD(3)),
    attributes Map(String, String) CODEC(ZSTD(1)),
    INDEX idx_msg message TYPE tokenbf_v1(30720, 2, 0) GRANULARITY 1
) ENGINE = MergeTree
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (tenant_id, cluster_id, namespace, log_level, timestamp)
TTL timestamp + INTERVAL 30 DAY DELETE;
