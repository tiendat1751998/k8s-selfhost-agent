-- Migration: 033_cloud_accounts.sql
-- Description: Creates cloud_accounts table for storing cloud provider configurations and credentials.

CREATE TABLE IF NOT EXISTS cloud_accounts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    provider        VARCHAR(50)  NOT NULL CHECK (provider IN ('aws', 'gcp', 'azure')),
    encrypted_creds TEXT         NOT NULL,
    region          VARCHAR(100) NOT NULL DEFAULT '',
    status          VARCHAR(50)  NOT NULL DEFAULT 'active',
    tenant_id       VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    last_sync_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_cloud_accounts_tenant ON cloud_accounts (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cloud_accounts_provider ON cloud_accounts (provider);
CREATE INDEX IF NOT EXISTS idx_cloud_accounts_status ON cloud_accounts (status);
