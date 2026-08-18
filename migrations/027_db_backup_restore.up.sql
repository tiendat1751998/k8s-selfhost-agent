CREATE TABLE backup_storages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    endpoint TEXT NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    credentials JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backup_storages_tenant ON backup_storages(tenant_id);

CREATE TABLE backup_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    db_type VARCHAR(255) NOT NULL,
    db_host VARCHAR(255) NOT NULL,
    db_port INT NOT NULL,
    db_name VARCHAR(255) NOT NULL,
    storage_id UUID NOT NULL REFERENCES backup_storages(id),
    schedule VARCHAR(255) NOT NULL,
    retention_count INT NOT NULL,
    backup_type VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backup_policies_tenant ON backup_policies(tenant_id);

CREATE TABLE backup_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    policy_id UUID NOT NULL REFERENCES backup_policies(id),
    status VARCHAR(255) NOT NULL,
    backup_type VARCHAR(255) NOT NULL,
    storage_path TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    duration_ms BIGINT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backup_jobs_tenant ON backup_jobs(tenant_id);

CREATE TABLE restore_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    backup_job_id UUID NOT NULL REFERENCES backup_jobs(id),
    target_db_host VARCHAR(255) NOT NULL,
    target_db_name VARCHAR(255) NOT NULL,
    pitr_timestamp TIMESTAMPTZ,
    status VARCHAR(255) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_restore_jobs_tenant ON restore_jobs(tenant_id);
