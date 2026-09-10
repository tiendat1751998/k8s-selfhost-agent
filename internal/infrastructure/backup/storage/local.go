package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
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

func (l *LocalStorage) fullPath(subpath string) (string, error) {
	cleanBase := filepath.Clean(l.basePath)

	// If subpath is an absolute path, verify it is strictly within cleanBase (clean relative to basePath)
	if filepath.IsAbs(subpath) {
		rel, err := filepath.Rel(cleanBase, subpath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", errors.New("invalid path: path traversal detected")
		}
		return filepath.Clean(subpath), nil
	}

	// Paths starting with / or \ (e.g. /etc/shadow on Windows where IsAbs is false without drive letter)
	if strings.HasPrefix(subpath, "/") || strings.HasPrefix(subpath, "\\") {
		vol := filepath.VolumeName(cleanBase)
		absCandidate := filepath.Join(vol, subpath)
		rel, err := filepath.Rel(cleanBase, absCandidate)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", errors.New("invalid path: path traversal detected")
		}
		return absCandidate, nil
	}

	cleaned := filepath.Clean(subpath)
	if strings.HasPrefix(cleaned, "..") {
		return "", errors.New("invalid path: path traversal detected")
	}

	targetPath := filepath.Join(cleanBase, cleaned)
	rel, err := filepath.Rel(cleanBase, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("invalid path: path traversal detected")
	}

	return targetPath, nil
}

func (l *LocalStorage) UploadStream(ctx context.Context, relPath string, reader io.Reader, size int64, metadata map[string]string) (string, error) {
	dest, err := l.fullPath(relPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", pkgerrors.Wrap(err, "creating local backup directories")
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", pkgerrors.Wrap(err, "creating local backup file")
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", pkgerrors.Wrap(err, "writing backup stream to local disk")
	}

	return dest, nil
}

func (l *LocalStorage) DownloadStream(ctx context.Context, relPath string) (io.ReadCloser, error) {
	dest, err := l.fullPath(relPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(dest)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "opening local backup file")
	}
	return f, nil
}

func (l *LocalStorage) Delete(ctx context.Context, relPath string) error {
	dest, err := l.fullPath(relPath)
	if err != nil {
		return err
	}
	return os.Remove(dest)
}

func (l *LocalStorage) Exists(ctx context.Context, relPath string) (bool, error) {
	dest, err := l.fullPath(relPath)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(dest)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
