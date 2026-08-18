-- 030_enhanced_backup_drivers.up.sql
-- Adds Dual-Target, Verification, Checksum, and Driver configuration columns

ALTER TABLE backup_policies
ADD COLUMN IF NOT EXISTS compression_level INT NOT NULL DEFAULT 3,
ADD COLUMN IF NOT EXISTS encryption_enabled BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS encryption_key_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS secondary_storage_id UUID REFERENCES backup_storages(id);

ALTER TABLE backup_jobs
ADD COLUMN IF NOT EXISTS checksum_sha256 TEXT,
ADD COLUMN IF NOT EXISTS local_storage_path TEXT,
ADD COLUMN IF NOT EXISTS cloud_storage_path TEXT,
ADD COLUMN IF NOT EXISTS wal_start_lsn TEXT,
ADD COLUMN IF NOT EXISTS wal_end_lsn TEXT,
ADD COLUMN IF NOT EXISTS compressed_size_bytes BIGINT,
ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS verification_status VARCHAR(50) DEFAULT 'unverified';

ALTER TABLE restore_jobs
ADD COLUMN IF NOT EXISTS dry_run BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN IF NOT EXISTS verification_log TEXT,
ADD COLUMN IF NOT EXISTS source_storage_type VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_backup_jobs_verification ON backup_jobs(verification_status);
