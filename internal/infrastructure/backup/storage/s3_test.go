package storage_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/storage"
)

func TestS3Storage_UploadStream_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access Denied to bucket"))
	}))
	defer ts.Close()

	s3Config := domainBackup.BackupStorage{
		Bucket: "test-bucket",
		Endpoint: ts.URL,
		Credentials: map[string]string{
			"access_key": "dummy",
			"secret_key": "dummy",
		},
	}

	s3Storage := storage.NewS3Storage(&s3Config)
	data := bytes.NewReader([]byte("test payload"))
	_, err := s3Storage.UploadStream(context.Background(), "backups/test.zst", data, 12, nil)
	if err == nil {
		t.Fatalf("expected error from S3 upload, got nil")
	}

	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "Access Denied") {
		t.Errorf("expected error message to contain status 403 and body, got: %v", err)
	}
}
