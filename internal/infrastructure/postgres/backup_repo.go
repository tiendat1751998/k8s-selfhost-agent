package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type BackupRepo struct {
	pool DBTX
}

func NewBackupRepo(pool DBTX) *BackupRepo {
	return &BackupRepo{pool: pool}
}

func (r *BackupRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func parseAndDecryptCredentials(rawCreds []byte) (map[string]string, error) {
	if len(rawCreds) == 0 {
		return make(map[string]string), nil
	}

	var wrapper map[string]string
	if err := json.Unmarshal(rawCreds, &wrapper); err != nil {
		return nil, errors.Wrap(err, "unmarshaling credentials json")
	}

	if encData, ok := wrapper["encrypted_data"]; ok && encData != "" {
		decrypted, err := crypto.Decrypt(encData)
		if err != nil {
			return nil, errors.Wrap(err, "decrypting credentials")
		}
		var creds map[string]string
		if err := json.Unmarshal([]byte(decrypted), &creds); err != nil {
			return nil, errors.Wrap(err, "unmarshaling decrypted credentials")
		}
		return creds, nil
	}

	return wrapper, nil
}

func (r *BackupRepo) CreateStorage(ctx context.Context, storage *backup.BackupStorage) error {
	var credsJSON []byte
	var err error

	if len(storage.Credentials) > 0 {
		rawCreds, err := json.Marshal(storage.Credentials)
		if err != nil {
			return errors.Wrap(err, "marshaling credentials")
		}

		encCreds, err := crypto.Encrypt(string(rawCreds))
		if err != nil {
			return errors.Wrap(err, "encrypting credentials")
		}

		credsJSON, err = json.Marshal(map[string]string{"encrypted_data": encCreds})
		if err != nil {
			return errors.Wrap(err, "marshaling encrypted credentials wrapper")
		}
	} else {
		credsJSON = []byte("{}")
	}

	query := `
		INSERT INTO backup_storages (tenant_id, name, type, endpoint, bucket, credentials, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`
	
	err = r.getDB(ctx).QueryRow(ctx, query,
		storage.TenantID, storage.Name, storage.Type, storage.Endpoint, storage.Bucket, credsJSON, storage.CreatedAt, storage.UpdatedAt,
	).Scan(&storage.ID)

	if err != nil {
		return errors.Wrap(err, "inserting backup storage")
	}
	return nil
}

func (r *BackupRepo) ListStorages(ctx context.Context, tenantID string) ([]*backup.BackupStorage, error) {
	query := `SELECT id, tenant_id, name, type, endpoint, bucket, credentials, created_at, updated_at FROM backup_storages WHERE tenant_id = $1`
	query, args := BuildTenantQuery(ctx, query, tenantID)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "querying backup storages")
	}
	defer rows.Close()

	var storages []*backup.BackupStorage
	for rows.Next() {
		var s backup.BackupStorage
		var creds []byte
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Type, &s.Endpoint, &s.Bucket, &creds, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning backup storage")
		}
		if len(creds) > 0 {
			decryptedCreds, err := parseAndDecryptCredentials(creds)
			if err != nil {
				return nil, err
			}
			s.Credentials = decryptedCreds
		}
		storages = append(storages, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterating backup storages")
	}
	return storages, nil
}

func (r *BackupRepo) GetStorage(ctx context.Context, id string) (*backup.BackupStorage, error) {
	query := `SELECT id, tenant_id, name, type, endpoint, bucket, credentials, created_at, updated_at FROM backup_storages WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	var s backup.BackupStorage
	var creds []byte
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(&s.ID, &s.TenantID, &s.Name, &s.Type, &s.Endpoint, &s.Bucket, &creds, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFound("backup_storage", id)
		}
		return nil, errors.Wrap(err, "querying backup storage by id")
	}
	if len(creds) > 0 {
		decryptedCreds, err := parseAndDecryptCredentials(creds)
		if err != nil {
			return nil, err
		}
		s.Credentials = decryptedCreds
	}
	return &s, nil
}

func (r *BackupRepo) CreatePolicy(ctx context.Context, policy *backup.BackupPolicy) error {
	query := `
		INSERT INTO backup_policies (tenant_id, name, db_type, db_host, db_port, db_name, storage_id, schedule, retention_count, backup_type, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`
	
	err := r.getDB(ctx).QueryRow(ctx, query,
		policy.TenantID, policy.Name, policy.DBType, policy.DBHost, policy.DBPort, policy.DBName, policy.StorageID, policy.Schedule, policy.RetentionCount, policy.BackupType, policy.Enabled, policy.CreatedAt, policy.UpdatedAt,
	).Scan(&policy.ID)

	if err != nil {
		return errors.Wrap(err, "inserting backup policy")
	}
	return nil
}

func (r *BackupRepo) ListPolicies(ctx context.Context, tenantID string) ([]*backup.BackupPolicy, error) {
	query := `SELECT id, tenant_id, name, db_type, db_host, db_port, db_name, storage_id, schedule, retention_count, backup_type, enabled, created_at, updated_at FROM backup_policies WHERE tenant_id = $1`
	query, args := BuildTenantQuery(ctx, query, tenantID)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "querying backup policies")
	}
	defer rows.Close()

	var policies []*backup.BackupPolicy
	for rows.Next() {
		var p backup.BackupPolicy
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.DBType, &p.DBHost, &p.DBPort, &p.DBName, &p.StorageID, &p.Schedule, &p.RetentionCount, &p.BackupType, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning backup policy")
		}
		policies = append(policies, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterating backup policies")
	}
	return policies, nil
}

func (r *BackupRepo) GetPolicy(ctx context.Context, id string) (*backup.BackupPolicy, error) {
	query := `SELECT id, tenant_id, name, db_type, db_host, db_port, db_name, storage_id, schedule, retention_count, backup_type, enabled, created_at, updated_at FROM backup_policies WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	var p backup.BackupPolicy
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(&p.ID, &p.TenantID, &p.Name, &p.DBType, &p.DBHost, &p.DBPort, &p.DBName, &p.StorageID, &p.Schedule, &p.RetentionCount, &p.BackupType, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFound("backup_policy", id)
		}
		return nil, errors.Wrap(err, "querying backup policy by id")
	}
	return &p, nil
}

func (r *BackupRepo) CreateJob(ctx context.Context, job *backup.BackupJob) error {
	query := `
		INSERT INTO backup_jobs (
			tenant_id, policy_id, status, backup_type, storage_path, local_storage_path, cloud_storage_path,
			size_bytes, compressed_size_bytes, duration_ms, checksum_sha256, wal_start_lsn, wal_end_lsn,
			verification_status, verified_at, error_message, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id`
	
	err := r.getDB(ctx).QueryRow(ctx, query,
		job.TenantID, job.PolicyID, job.Status, job.BackupType, job.StoragePath, job.LocalStoragePath, job.CloudStoragePath,
		job.SizeBytes, job.CompressedSizeBytes, job.DurationMs, job.ChecksumSHA256, job.WALStartLSN, job.WALEndLSN,
		job.VerificationStatus, job.VerifiedAt, job.ErrorMessage, job.CreatedAt, job.UpdatedAt,
	).Scan(&job.ID)

	if err != nil {
		return errors.Wrap(err, "inserting backup job")
	}
	return nil
}

func (r *BackupRepo) ListJobs(ctx context.Context, tenantID string) ([]*backup.BackupJob, error) {
	query := `
		SELECT id, tenant_id, policy_id, status, backup_type, storage_path, 
		       COALESCE(local_storage_path, ''), COALESCE(cloud_storage_path, ''),
		       size_bytes, COALESCE(compressed_size_bytes, 0), duration_ms, 
		       COALESCE(checksum_sha256, ''), COALESCE(wal_start_lsn, ''), COALESCE(wal_end_lsn, ''),
		       COALESCE(verification_status, 'unverified'), verified_at, error_message, created_at, updated_at 
		FROM backup_jobs WHERE tenant_id = $1 ORDER BY created_at DESC`
	query, args := BuildTenantQuery(ctx, query, tenantID)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "querying backup jobs")
	}
	defer rows.Close()

	var jobs []*backup.BackupJob
	for rows.Next() {
		var j backup.BackupJob
		if err := rows.Scan(
			&j.ID, &j.TenantID, &j.PolicyID, &j.Status, &j.BackupType, &j.StoragePath,
			&j.LocalStoragePath, &j.CloudStoragePath, &j.SizeBytes, &j.CompressedSizeBytes, &j.DurationMs,
			&j.ChecksumSHA256, &j.WALStartLSN, &j.WALEndLSN, &j.VerificationStatus, &j.VerifiedAt,
			&j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "scanning backup job")
		}
		jobs = append(jobs, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterating backup jobs")
	}
	return jobs, nil
}

func (r *BackupRepo) GetJob(ctx context.Context, id string) (*backup.BackupJob, error) {
	query := `
		SELECT id, tenant_id, policy_id, status, backup_type, storage_path, 
		       COALESCE(local_storage_path, ''), COALESCE(cloud_storage_path, ''),
		       size_bytes, COALESCE(compressed_size_bytes, 0), duration_ms, 
		       COALESCE(checksum_sha256, ''), COALESCE(wal_start_lsn, ''), COALESCE(wal_end_lsn, ''),
		       COALESCE(verification_status, 'unverified'), verified_at, error_message, created_at, updated_at 
		FROM backup_jobs WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	var j backup.BackupJob
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&j.ID, &j.TenantID, &j.PolicyID, &j.Status, &j.BackupType, &j.StoragePath,
		&j.LocalStoragePath, &j.CloudStoragePath, &j.SizeBytes, &j.CompressedSizeBytes, &j.DurationMs,
		&j.ChecksumSHA256, &j.WALStartLSN, &j.WALEndLSN, &j.VerificationStatus, &j.VerifiedAt,
		&j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFound("backup_job", id)
		}
		return nil, errors.Wrap(err, "querying backup job by id")
	}
	return &j, nil
}

func (r *BackupRepo) UpdateJob(ctx context.Context, job *backup.BackupJob) error {
	query := `
		UPDATE backup_jobs 
		SET status = $1, storage_path = $2, local_storage_path = $3, cloud_storage_path = $4,
		    size_bytes = $5, compressed_size_bytes = $6, duration_ms = $7, checksum_sha256 = $8,
		    wal_start_lsn = $9, wal_end_lsn = $10, verification_status = $11, verified_at = $12,
		    error_message = $13, updated_at = $14
		WHERE id = $15`
	
	query, args := BuildTenantQuery(ctx, query,
		job.Status, job.StoragePath, job.LocalStoragePath, job.CloudStoragePath,
		job.SizeBytes, job.CompressedSizeBytes, job.DurationMs, job.ChecksumSHA256,
		job.WALStartLSN, job.WALEndLSN, job.VerificationStatus, job.VerifiedAt,
		job.ErrorMessage, job.UpdatedAt, job.ID,
	)

	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "updating backup job")
	}
	return nil
}

func (r *BackupRepo) CreateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	query := `
		INSERT INTO restore_jobs (
			tenant_id, backup_job_id, target_db_host, target_db_name, pitr_timestamp, 
			dry_run, source_storage_type, verification_log, status, error_message, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`
	
	err := r.getDB(ctx).QueryRow(ctx, query,
		restore.TenantID, restore.BackupJobID, restore.TargetDBHost, restore.TargetDBName, restore.PITRTimestamp,
		restore.DryRun, restore.SourceStorageType, restore.VerificationLog, restore.Status, restore.ErrorMessage,
		restore.CreatedAt, restore.UpdatedAt,
	).Scan(&restore.ID)

	if err != nil {
		return errors.Wrap(err, "inserting restore job")
	}
	return nil
}

func (r *BackupRepo) ListRestores(ctx context.Context, tenantID string) ([]*backup.RestoreJob, error) {
	query := `
		SELECT id, tenant_id, backup_job_id, target_db_host, target_db_name, pitr_timestamp,
		       COALESCE(dry_run, false), COALESCE(source_storage_type, ''), COALESCE(verification_log, ''),
		       status, error_message, created_at, updated_at 
		FROM restore_jobs WHERE tenant_id = $1 ORDER BY created_at DESC`
	query, args := BuildTenantQuery(ctx, query, tenantID)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "querying restore jobs")
	}
	defer rows.Close()

	var restores []*backup.RestoreJob
	for rows.Next() {
		var j backup.RestoreJob
		if err := rows.Scan(
			&j.ID, &j.TenantID, &j.BackupJobID, &j.TargetDBHost, &j.TargetDBName, &j.PITRTimestamp,
			&j.DryRun, &j.SourceStorageType, &j.VerificationLog, &j.Status, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "scanning restore job")
		}
		restores = append(restores, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterating restore jobs")
	}
	return restores, nil
}

func (r *BackupRepo) GetRestore(ctx context.Context, id string) (*backup.RestoreJob, error) {
	query := `
		SELECT id, tenant_id, backup_job_id, target_db_host, target_db_name, pitr_timestamp,
		       COALESCE(dry_run, false), COALESCE(source_storage_type, ''), COALESCE(verification_log, ''),
		       status, error_message, created_at, updated_at 
		FROM restore_jobs WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	var j backup.RestoreJob
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&j.ID, &j.TenantID, &j.BackupJobID, &j.TargetDBHost, &j.TargetDBName, &j.PITRTimestamp,
		&j.DryRun, &j.SourceStorageType, &j.VerificationLog, &j.Status, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFound("restore_job", id)
		}
		return nil, errors.Wrap(err, "querying restore job by id")
	}
	return &j, nil
}

func (r *BackupRepo) UpdateRestore(ctx context.Context, restore *backup.RestoreJob) error {
	query := `
		UPDATE restore_jobs 
		SET status = $1, error_message = $2, verification_log = $3, updated_at = $4
		WHERE id = $5`
	
	query, args := BuildTenantQuery(ctx, query,
		restore.Status, restore.ErrorMessage, restore.VerificationLog, restore.UpdatedAt, restore.ID,
	)

	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "updating restore job")
	}
	return nil
}


