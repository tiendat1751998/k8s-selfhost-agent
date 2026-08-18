package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	if basePath == "" {
		basePath = "/var/lib/k8sselfhost/backups"
	}
	return &LocalStorage{basePath: basePath}
}

func (l *LocalStorage) Type() string {
	return "local"
}

func (l *LocalStorage) fullPath(subpath string) string {
	if filepath.IsAbs(subpath) {
		return subpath
	}
	return filepath.Join(l.basePath, subpath)
}


func (l *LocalStorage) UploadStream(ctx context.Context, relPath string, reader io.Reader, size int64, metadata map[string]string) (string, error) {
	dest := l.fullPath(relPath)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", errors.Wrap(err, "creating local backup directories")
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", errors.Wrap(err, "creating local backup file")
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", errors.Wrap(err, "writing backup stream to local disk")
	}

	return dest, nil
}

func (l *LocalStorage) DownloadStream(ctx context.Context, relPath string) (io.ReadCloser, error) {
	dest := l.fullPath(relPath)
	f, err := os.Open(dest)
	if err != nil {
		return nil, errors.Wrap(err, "opening local backup file")
	}
	return f, nil
}

func (l *LocalStorage) Delete(ctx context.Context, relPath string) error {
	dest := l.fullPath(relPath)
	return os.Remove(dest)
}

func (l *LocalStorage) Exists(ctx context.Context, relPath string) (bool, error) {
	dest := l.fullPath(relPath)
	_, err := os.Stat(dest)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
