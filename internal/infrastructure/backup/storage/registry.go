package storage

import (
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type StorageRegistry struct {
	localStorage *LocalStorage
}

func NewStorageRegistry(localStorage *LocalStorage) *StorageRegistry {
	return &StorageRegistry{
		localStorage: localStorage,
	}
}

func (r *StorageRegistry) Resolve(storage *backup.BackupStorage) (backup.StorageTarget, error) {
	if storage == nil {
		return r.localStorage, nil
	}
	switch storage.Type {
	case "local", "nfs":
		return r.localStorage, nil
	case "s3", "minio", "r2", "gcs":
		return NewS3Storage(storage), nil
	default:
		return nil, errors.NewNotFound("storage_type", fmt.Sprintf("unsupported storage type: %s", storage.Type))
	}
}
