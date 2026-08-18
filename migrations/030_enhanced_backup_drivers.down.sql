-- 030_enhanced_backup_drivers.down.sql

DROP INDEX IF EXISTS idx_backup_jobs_verification;

ALTER TABLE restore_jobs
DROP COLUMN IF EXISTS dry_run,
DROP COLUMN IF EXISTS verification_log,
DROP COLUMN IF EXISTS source_storage_type;

ALTER TABLE backup_jobs
DROP COLUMN IF EXISTS checksum_sha256,
DROP COLUMN IF EXISTS local_storage_path,
DROP COLUMN IF EXISTS cloud_storage_path,
DROP COLUMN IF EXISTS wal_start_lsn,
DROP COLUMN IF EXISTS wal_end_lsn,
DROP COLUMN IF EXISTS compressed_size_bytes,
DROP COLUMN IF EXISTS verified_at,
DROP COLUMN IF EXISTS verification_status;

ALTER TABLE backup_policies
DROP COLUMN IF EXISTS compression_level,
DROP COLUMN IF EXISTS encryption_enabled,
DROP COLUMN IF EXISTS encryption_key_id,
DROP COLUMN IF EXISTS secondary_storage_id;
