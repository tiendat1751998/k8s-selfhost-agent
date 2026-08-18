package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

func (e *Engine) ExecuteBackup(ctx context.Context, jobID string) error {
	start := time.Now()

	// 1. Load Job
	job, err := e.repo.GetJob(ctx, jobID)
	if err != nil {
		return errors.Wrap(err, "loading backup job")
	}

	job.Start()
	if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
		logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
	}

	// 2. Load Policy & Storages
	policy, err := e.repo.GetPolicy(ctx, job.PolicyID)
	if err != nil {
		job.Fail(fmt.Sprintf("failed to get policy: %v", err))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return err
	}

	primaryStorage, err := e.repo.GetStorage(ctx, policy.StorageID)
	if err != nil {
		job.Fail(fmt.Sprintf("failed to get primary storage: %v", err))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
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
		job.Fail(fmt.Sprintf("unsupported db driver %s: %v", policy.DBType, err))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return err
	}

	primaryTarget, err := e.storageRegistry.Resolve(primaryStorage)
	if err != nil {
		job.Fail(fmt.Sprintf("resolving primary storage: %v", err))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return err
	}

	var secondaryTarget backup.StorageTarget
	if secondaryStorage != nil {
		secondaryTarget, _ = e.storageRegistry.Resolve(secondaryStorage)
	}

	// 4. Build streaming pipes for dual storage upload (avoids OOM on large DBs)
	timestampStr := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s/%s_%s_%s.sql.zst", job.TenantID, policy.Name, policy.DBName, timestampStr)

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
	var encKey []byte
	if policy.EncryptionEnabled && len(policy.EncryptionKeyID) >= 32 {
		encKey = []byte(policy.EncryptionKeyID[:32])
	}

	pipeConfig := dualsync.PipeConfig{
		CompressionLevel: policy.CompressionLevel,
		EncryptionKey:    encKey,
		EnableEncryption: policy.EncryptionEnabled && len(encKey) == 32,
	}

	pipe, pipeCloser, err := dualsync.NewProcessingPipe(dualWriter, pipeConfig)
	if err != nil {
		job.Fail(fmt.Sprintf("initializing processing pipe: %v", err))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
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

	var dumpErr error
	dumpDone := make(chan struct{})
	go func() {
		defer close(dumpDone)
		_, dumpErr = driver.Dump(ctx, dumpOpts, pipeCloser)
		_ = pipeCloser.Close()
		_ = primaryPW.Close()
		if secondaryPW != nil {
			_ = secondaryPW.Close()
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

	if dumpErr != nil {
		job.Fail(fmt.Sprintf("database dump failed: %v", dumpErr))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return dumpErr
	}

	if uploadErr != nil {
		job.Fail(fmt.Sprintf("primary storage upload failed: %v", uploadErr))
		if updateErr := e.repo.UpdateJob(ctx, job); updateErr != nil {
			logger.Get().Error("failed to update backup job status", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return uploadErr
	}

	rawSize, compSize, checksum := pipe.Summary()
	job.LocalStoragePath = primaryPath
	job.StoragePath = primaryPath

	if secondaryTarget != nil && secErr == nil && secondaryPath != "" {
		job.CloudStoragePath = secondaryPath
	}

	// 8. Verify by re-downloading and comparing checksum
	verificationStatus := "unverified"
	downloadStream, dlErr := primaryTarget.DownloadStream(ctx, primaryPath)
	if dlErr == nil {
		verifyHasher := sha256.New()
		_, copyErr := io.Copy(verifyHasher, downloadStream)
		_ = downloadStream.Close()
		if copyErr == nil {
			downloadChecksum := hex.EncodeToString(verifyHasher.Sum(nil))
			if downloadChecksum == checksum {
				verificationStatus = "verified"
			} else {
				verificationStatus = "checksum_mismatch"
			}
		}
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

	return e.repo.UpdateJob(ctx, job)
}

func (e *Engine) ExecuteRestore(ctx context.Context, restoreID string) error {
	restore, err := e.repo.GetRestore(ctx, restoreID)
	if err != nil {
		return errors.Wrap(err, "loading restore job")
	}

	restore.Start()
	if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
		logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
	}

	job, err := e.repo.GetJob(ctx, restore.BackupJobID)
	if err != nil {
		restore.Fail(fmt.Sprintf("loading backup job: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}

	policy, err := e.repo.GetPolicy(ctx, job.PolicyID)
	if err != nil {
		restore.Fail(fmt.Sprintf("loading backup policy: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}

	storageModel, err := e.repo.GetStorage(ctx, policy.StorageID)
	if err != nil {
		restore.Fail(fmt.Sprintf("loading storage config: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}

	driver, err := e.driverRegistry.Get(policy.DBType)
	if err != nil {
		restore.Fail(fmt.Sprintf("resolving driver: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}

	storageTarget, err := e.storageRegistry.Resolve(storageModel)
	if err != nil {
		restore.Fail(fmt.Sprintf("resolving storage target: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
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
		restore.Fail(fmt.Sprintf("downloading backup stream: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}
	defer srcStream.Close()

	// Decompress and Decrypt
	var encKey []byte
	if policy.EncryptionEnabled && len(policy.EncryptionKeyID) >= 32 {
		encKey = []byte(policy.EncryptionKeyID[:32])
	}

	decompStream, err := dualsync.NewDecompressionPipe(srcStream, encKey, policy.EncryptionEnabled && len(encKey) == 32)
	if err != nil {
		restore.Fail(fmt.Sprintf("initializing decompression stream: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}
	defer decompStream.Close()

	// Execute Restore
	restoreOpts := backup.RestoreOptions{
		DBType:        policy.DBType,
		Host:          restore.TargetDBHost,
		Port:          policy.DBPort,
		Database:      restore.TargetDBName,
		PITRTimestamp: restore.PITRTimestamp,
		DryRun:        restore.DryRun,
		CleanTarget:   true,
	}

	if err := driver.Restore(ctx, restoreOpts, decompStream); err != nil {
		restore.Fail(fmt.Sprintf("database restore execution failed: %v", err))
		if updateErr := e.repo.UpdateRestore(ctx, restore); updateErr != nil {
			logger.Get().Error("failed to update restore job status", zap.String("restore_id", restoreID), zap.Error(updateErr))
		}
		return err
	}

	restore.Complete(fmt.Sprintf("Restore completed at %s. Source backup checksum: %s.", time.Now().UTC().Format(time.RFC3339), job.ChecksumSHA256))
	return e.repo.UpdateRestore(ctx, restore)
}
