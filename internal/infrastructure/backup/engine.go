package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/dualsync"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type Engine struct {
	repo            backup.Repository
	driverRegistry  *drivers.DriverRegistry
	storageRegistry *storage.StorageRegistry
}

func NewEngine(repo backup.Repository, driverRegistry *drivers.DriverRegistry, storageRegistry *storage.StorageRegistry) *Engine {
	return &Engine{
		repo:            repo,
		driverRegistry:  driverRegistry,
		storageRegistry: storageRegistry,
	}
}

func (e *Engine) terminalCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (e *Engine) ReconcileStaleJobs(ctx context.Context) error {
	if reconciler, ok := e.repo.(backup.StaleJobReconciler); ok {
		return reconciler.ReconcileStaleJobs(ctx)
	}
	return nil
}

func (e *Engine) failJob(ctx context.Context, job *backup.BackupJob, reason string, targets ...backup.StorageTarget) {
	job.Fail(reason)
	termCtx, cancel := e.terminalCtx(ctx)
	defer cancel()

	for _, target := range targets {
		if target == nil {
			continue
		}
		if job.StoragePath != "" {
			if delErr := target.Delete(termCtx, job.StoragePath); delErr != nil && !os.IsNotExist(delErr) {
				logger.Get().Warn("failed to delete partial artifact", zap.String("path", job.StoragePath), zap.Error(delErr))
			}
		}
		if job.LocalStoragePath != "" && job.LocalStoragePath != job.StoragePath {
			if delErr := target.Delete(termCtx, job.LocalStoragePath); delErr != nil && !os.IsNotExist(delErr) {
				logger.Get().Warn("failed to delete partial artifact", zap.String("path", job.LocalStoragePath), zap.Error(delErr))
			}
		}
	}

	if updateErr := e.repo.UpdateJob(termCtx, job); updateErr != nil {
		logger.Get().Error("failed to update backup job status", zap.String("job_id", job.ID), zap.Error(updateErr))
	}
}

func (e *Engine) failRestore(ctx context.Context, restore *backup.RestoreJob, reason string) {
	restore.Fail(reason)
	termCtx, cancel := e.terminalCtx(ctx)
	defer cancel()

	if updateErr := e.repo.UpdateRestore(termCtx, restore); updateErr != nil {
		logger.Get().Error("failed to update restore job status", zap.String("restore_id", restore.ID), zap.Error(updateErr))
	}
}

func (e *Engine) ExecuteBackup(ctx context.Context, jobID string) error {
	start := time.Now()

	// 1. Load Job
	job, err := e.repo.GetJob(ctx, jobID)
	if err != nil {
		return errors.Wrap(err, "loading backup job")
	}

	job.Start()
	termCtx, startCancel := e.terminalCtx(ctx)
	if updateErr := e.repo.UpdateJob(termCtx, job); updateErr != nil {
		logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
	}
	startCancel()

	// 2. Load Policy & Storages
	policy, err := e.repo.GetPolicy(ctx, job.PolicyID)
	if err != nil {
		e.failJob(ctx, job, fmt.Sprintf("failed to get policy: %v", err))
		return err
	}

	primaryStorage, err := e.repo.GetStorage(ctx, policy.StorageID)
	if err != nil {
		e.failJob(ctx, job, fmt.Sprintf("failed to get primary storage: %v", err))
		return err
	}

	var secondaryStorage *backup.BackupStorage
	if policy.SecondaryStorageID != "" {
		sec, err := e.repo.GetStorage(ctx, policy.SecondaryStorageID)
		if err == nil {
			secondaryStorage = sec
		}
	}

	// 3. Resolve Driver & Storage Targets
	driver, err := e.driverRegistry.Get(policy.DBType)
	if err != nil {
		e.failJob(ctx, job, fmt.Sprintf("unsupported db driver %s: %v", policy.DBType, err))
		return err
	}

	primaryTarget, err := e.storageRegistry.Resolve(primaryStorage)
	if err != nil {
		e.failJob(ctx, job, fmt.Sprintf("resolving primary storage: %v", err))
		return err
	}

	var secondaryTarget backup.StorageTarget
	if secondaryStorage != nil {
		secondaryTarget, _ = e.storageRegistry.Resolve(secondaryStorage)
	}

	// 4. Build streaming pipes for dual storage upload (avoids OOM on large DBs)
	timestampStr := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s/%s_%s_%s.sql.zst", job.TenantID, policy.Name, policy.DBName, timestampStr)
	job.StoragePath = filename
	job.LocalStoragePath = filename

	primaryPR, primaryPW := io.Pipe()
	var secondaryPR *io.PipeReader
	var secondaryPW *io.PipeWriter

	dualWriter := &dualsync.DualWriter{
		TargetA: primaryPW,
	}
	if secondaryTarget != nil {
		secondaryPR, secondaryPW = io.Pipe()
		dualWriter.TargetB = secondaryPW
	}

	// 5. Setup Compression & Encryption Pipe
	encKey, err := resolveEncryptionKey(policy)
	if err != nil {
		e.failJob(ctx, job, err.Error(), primaryTarget, secondaryTarget)
		return err
	}

	pipeConfig := dualsync.PipeConfig{
		CompressionLevel: policy.CompressionLevel,
		EncryptionKey:    encKey,
		EnableEncryption: policy.EncryptionEnabled,
	}

	pipe, pipeCloser, err := dualsync.NewProcessingPipe(dualWriter, pipeConfig)
	if err != nil {
		e.failJob(ctx, job, fmt.Sprintf("initializing processing pipe: %v", err), primaryTarget, secondaryTarget)
		return err
	}

	// 6. Execute Dump & Upload (concurrent streaming)
	dumpOpts := backup.DumpOptions{
		DBType:     policy.DBType,
		Host:       policy.DBHost,
		Port:       policy.DBPort,
		Database:   policy.DBName,
		BackupType: policy.BackupType,
	}

	meta := map[string]string{
		"tenant":   job.TenantID,
		"policy":   policy.Name,
		"checksum": "",
	}

	ctxDoneWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if closeErr := primaryPR.CloseWithError(ctx.Err()); closeErr != nil {
				logger.Get().Warn("failed to close primary pipe reader on cancel", zap.Error(closeErr))
			}
			if secondaryPR != nil {
				if closeErr := secondaryPR.CloseWithError(ctx.Err()); closeErr != nil {
					logger.Get().Warn("failed to close secondary pipe reader on cancel", zap.Error(closeErr))
				}
			}
		case <-ctxDoneWatch:
		}
	}()
	defer close(ctxDoneWatch)

	var dumpErr error
	dumpDone := make(chan struct{})
	go func() {
		defer close(dumpDone)
		_, dumpErr = driver.Dump(ctx, dumpOpts, pipeCloser)
		if closeErr := pipeCloser.Close(); closeErr != nil {
			logger.Get().Warn("failed to close pipe closer", zap.Error(closeErr))
		}
		if closeErr := primaryPW.Close(); closeErr != nil {
			logger.Get().Warn("failed to close primary pipe writer", zap.Error(closeErr))
		}
		if secondaryPW != nil {
			if closeErr := secondaryPW.Close(); closeErr != nil {
				logger.Get().Warn("failed to close secondary pipe writer", zap.Error(closeErr))
			}
		}
	}()

	var secondaryPath string
	var secErr error
	var secDone chan struct{}
	if secondaryTarget != nil && secondaryPR != nil {
		secDone = make(chan struct{})
		go func() {
			defer close(secDone)
			secondaryPath, secErr = secondaryTarget.UploadStream(ctx, filename, secondaryPR, 0, meta)
		}()
	}

	primaryPath, uploadErr := primaryTarget.UploadStream(ctx, filename, primaryPR, 0, meta)
	<-dumpDone
	if secDone != nil {
		<-secDone
	}

	if dumpErr != nil || uploadErr != nil || secErr != nil || ctx.Err() != nil {
		err := dumpErr
		if err == nil {
			err = uploadErr
		}
		if err == nil {
			err = secErr
		}
		if err == nil {
			err = ctx.Err()
		}

		reason := fmt.Sprintf("database dump failed: %v", err)
		if ctx.Err() != nil {
			reason = fmt.Sprintf("backup cancelled: %v", ctx.Err())
		} else if uploadErr != nil {
			reason = fmt.Sprintf("primary storage upload failed: %v", uploadErr)
		} else if secErr != nil {
			reason = fmt.Sprintf("secondary storage upload failed: %v", secErr)
		}

		e.failJob(ctx, job, reason, primaryTarget, secondaryTarget)
		return err
	}

	rawSize, compSize, checksum := pipe.Summary()
	job.LocalStoragePath = primaryPath
	job.StoragePath = primaryPath

	if secondaryTarget != nil && secErr == nil && secondaryPath != "" {
		job.CloudStoragePath = secondaryPath
	}

	// 8. Verify by re-downloading and comparing checksum
	verificationStatus := "unverified"
	if ctx.Err() != nil {
		e.failJob(ctx, job, fmt.Sprintf("backup cancelled during verification: %v", ctx.Err()), primaryTarget, secondaryTarget)
		return ctx.Err()
	}

	downloadStream, dlErr := primaryTarget.DownloadStream(ctx, primaryPath)
	if dlErr == nil {
		verifyHasher := sha256.New()
		_, copyErr := io.Copy(verifyHasher, downloadStream)
		if closeErr := downloadStream.Close(); closeErr != nil {
			logger.Get().Warn("failed to close download stream", zap.Error(closeErr))
		}
		if copyErr == nil {
			downloadChecksum := hex.EncodeToString(verifyHasher.Sum(nil))
			if downloadChecksum == checksum {
				verificationStatus = "verified"
			} else {
				verificationStatus = "checksum_mismatch"
			}
		}
	}

	if ctx.Err() != nil {
		e.failJob(ctx, job, fmt.Sprintf("backup cancelled after verification: %v", ctx.Err()), primaryTarget, secondaryTarget)
		return ctx.Err()
	}

	// 9. Update Job with Success
	job.Complete(rawSize, time.Since(start))
	job.CompressedSizeBytes = compSize
	job.ChecksumSHA256 = checksum
	job.MarkVerified(backup.VerificationResult{
		Passed:     verificationStatus == "verified",
		ChecksumOK: verificationStatus == "verified",
		CheckedAt:  time.Now().UTC(),
	})

	termCtx, compCancel := e.terminalCtx(ctx)
	defer compCancel()
	return e.repo.UpdateJob(termCtx, job)
}

func (e *Engine) ExecuteRestore(ctx context.Context, restoreID string) error {
	restore, err := e.repo.GetRestore(ctx, restoreID)
	if err != nil {
		return errors.Wrap(err, "loading restore job")
	}

	restore.Start()
	termCtx, startCancel := e.terminalCtx(ctx)
	if updateErr := e.repo.UpdateRestore(termCtx, restore); updateErr != nil {
		logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
	}
	startCancel()

	job, err := e.repo.GetJob(ctx, restore.BackupJobID)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("loading backup job: %v", err))
		return err
	}

	policy, err := e.repo.GetPolicy(ctx, job.PolicyID)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("loading backup policy: %v", err))
		return err
	}

	storageModel, err := e.repo.GetStorage(ctx, policy.StorageID)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("loading storage config: %v", err))
		return err
	}

	driver, err := e.driverRegistry.Get(policy.DBType)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("resolving driver: %v", err))
		return err
	}

	storageTarget, err := e.storageRegistry.Resolve(storageModel)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("resolving storage target: %v", err))
		return err
	}

	// Download Stream
	srcStream, err := storageTarget.DownloadStream(ctx, job.StoragePath)
	if err != nil {
		// Try fallback to local path if storagePath failed
		if job.LocalStoragePath != "" {
			srcStream, err = storageTarget.DownloadStream(ctx, job.LocalStoragePath)
		}
	}
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("downloading backup stream: %v", err))
		return err
	}
	defer func() {
		if closeErr := srcStream.Close(); closeErr != nil {
			logger.Get().Warn("failed to close restore source stream", zap.Error(closeErr))
		}
	}()

	// Decompress and Decrypt
	encKey, err := resolveEncryptionKey(policy)
	if err != nil {
		e.failRestore(ctx, restore, err.Error())
		return err
	}

	decompStream, err := dualsync.NewDecompressionPipe(srcStream, encKey, policy.EncryptionEnabled)
	if err != nil {
		e.failRestore(ctx, restore, fmt.Sprintf("initializing decompression stream: %v", err))
		return err
	}
	defer func() {
		if closeErr := decompStream.Close(); closeErr != nil {
			logger.Get().Warn("failed to close decompression stream", zap.Error(closeErr))
		}
	}()

	// Execute Restore
	restoreOpts := backup.RestoreOptions{
		DBType:        policy.DBType,
		Host:          restore.TargetDBHost,
		Port:          policy.DBPort,
		Database:      restore.TargetDBName,
		PITRTimestamp: restore.PITRTimestamp,
		DryRun:        restore.DryRun,
		CleanTarget:   !restore.DryRun,
	}

	if err := driver.Restore(ctx, restoreOpts, decompStream); err != nil {
		reason := fmt.Sprintf("database restore execution failed: %v", err)
		if ctx.Err() != nil {
			reason = fmt.Sprintf("database restore cancelled: %v", ctx.Err())
		}
		e.failRestore(ctx, restore, reason)
		return err
	}

	restore.Complete(fmt.Sprintf("Restore completed at %s. Source backup checksum: %s.", time.Now().UTC().Format(time.RFC3339), job.ChecksumSHA256))
	termCtx, compCancel := e.terminalCtx(ctx)
	defer compCancel()
	return e.repo.UpdateRestore(termCtx, restore)
}

func resolveEncryptionKey(policy *backup.BackupPolicy) ([]byte, error) {
	if !policy.EncryptionEnabled {
		return nil, nil
	}
	if policy.EncryptionKeyID == "" {
		return nil, fmt.Errorf("encryption enabled but EncryptionKeyID is empty")
	}
	if len(policy.EncryptionKeyID) >= 32 {
		return []byte(policy.EncryptionKeyID[:32]), nil
	}
	derived := sha256.Sum256([]byte(policy.EncryptionKeyID))
	key := make([]byte, 32)
	copy(key, derived[:])
	return key, nil
}



