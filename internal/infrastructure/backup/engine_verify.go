package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/dualsync"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// VerifyBackup performs a dry-run verification of a backup artifact to ensure it is valid
// and restorable without touching or mutating production database tables.
func (e *Engine) VerifyBackup(ctx context.Context, jobID string) error {
	job, err := e.repo.GetJob(ctx, jobID)
	if err != nil {
		return errors.Wrap(err, "loading backup job for verification")
	}

	policy, err := e.repo.GetPolicy(ctx, job.PolicyID)
	if err != nil {
		return errors.Wrap(err, "loading backup policy for verification")
	}

	storageModel, err := e.repo.GetStorage(ctx, policy.StorageID)
	if err != nil {
		return errors.Wrap(err, "loading storage config for verification")
	}

	driver, err := e.driverRegistry.Get(policy.DBType)
	if err != nil {
		return errors.Wrap(err, "resolving driver for verification")
	}

	storageTarget, err := e.storageRegistry.Resolve(storageModel)
	if err != nil {
		return errors.Wrap(err, "resolving storage target for verification")
	}

	srcStream, err := storageTarget.DownloadStream(ctx, job.StoragePath)
	if err != nil && job.LocalStoragePath != "" {
		srcStream, err = storageTarget.DownloadStream(ctx, job.LocalStoragePath)
	}
	if err != nil {
		return errors.Wrap(err, "downloading backup stream for verification")
	}
	defer srcStream.Close()

	var encKey []byte
	if policy.EncryptionEnabled && len(policy.EncryptionKeyID) >= 32 {
		encKey = []byte(policy.EncryptionKeyID[:32])
	}

	decompStream, err := dualsync.NewDecompressionPipe(srcStream, encKey, policy.EncryptionEnabled && len(encKey) == 32)
	if err != nil {
		return errors.Wrap(err, "initializing decompression stream for verification")
	}
	defer decompStream.Close()

	// DryRun is explicitly true and CleanTarget is explicitly false: NEVER touch production tables
	restoreOpts := backup.RestoreOptions{
		DBType:      policy.DBType,
		Host:        policy.DBHost,
		Port:        policy.DBPort,
		Database:    policy.DBName,
		DryRun:      true,
		CleanTarget: false,
	}

	if err := driver.Restore(ctx, restoreOpts, decompStream); err != nil {
		job.MarkVerified(backup.VerificationResult{
			Passed:     false,
			ChecksumOK: false,
			CheckedAt:  time.Now().UTC(),
		})
		termCtx, compCancel := e.terminalCtx(ctx)
		defer compCancel()
		_ = e.repo.UpdateJob(termCtx, job)
		return fmt.Errorf("backup verification failed: %w", err)
	}

	job.MarkVerified(backup.VerificationResult{
		Passed:     true,
		ChecksumOK: true,
		CheckedAt:  time.Now().UTC(),
	})
	termCtx, compCancel := e.terminalCtx(ctx)
	defer compCancel()
	return e.repo.UpdateJob(termCtx, job)
}
