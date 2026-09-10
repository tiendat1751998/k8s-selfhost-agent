package storage

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_PathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewLocalStorage(tempDir)
	ctx := context.Background()

	traversalPaths := []string{
		"../../etc/passwd",
		"/etc/shadow",
		"../../../sensitive_file",
		"sub/../../outside",
		"..",
	}

	for _, p := range traversalPaths {
		t.Run("fullPath rejects "+p, func(t *testing.T) {
			_, err := storage.fullPath(p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path: path traversal detected")
		})

		t.Run("DownloadStream rejects "+p, func(t *testing.T) {
			rc, err := storage.DownloadStream(ctx, p)
			if rc != nil {
				rc.Close()
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path: path traversal detected")
		})

		t.Run("UploadStream rejects "+p, func(t *testing.T) {
			_, err := storage.UploadStream(ctx, p, bytes.NewReader([]byte("test")), 4, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path: path traversal detected")
		})

		t.Run("Delete rejects "+p, func(t *testing.T) {
			err := storage.Delete(ctx, p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path: path traversal detected")
		})

		t.Run("Exists rejects "+p, func(t *testing.T) {
			exists, err := storage.Exists(ctx, p)
			assert.False(t, exists)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid path: path traversal detected")
		})
	}

	t.Run("valid subpath allowed", func(t *testing.T) {
		validPath := "valid/backup.tar.gz"
		got, err := storage.fullPath(validPath)
		require.NoError(t, err)
		expected := filepath.Join(tempDir, filepath.FromSlash("valid/backup.tar.gz"))
		assert.Equal(t, expected, got)
	})
}
