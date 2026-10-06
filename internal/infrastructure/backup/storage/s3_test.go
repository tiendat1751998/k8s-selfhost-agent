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
	t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "127.0.0.0/8")
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

func TestS3Storage_SSRFProtection(t *testing.T) {
	// 1. Cloud metadata IP (169.254.169.254) must always be blocked
	t.Run("blocks cloud metadata 169.254.169.254", func(t *testing.T) {
		s3Config := domainBackup.BackupStorage{
			Bucket:   "test-bucket",
			Endpoint: "http://169.254.169.254",
			Credentials: map[string]string{
				"access_key": "dummy",
				"secret_key": "dummy",
			},
		}

		s3Storage := storage.NewS3Storage(&s3Config)
		data := bytes.NewReader([]byte("test payload"))
		_, err := s3Storage.UploadStream(context.Background(), "backups/test.zst", data, 12, nil)
		if err == nil {
			t.Fatalf("expected SSRF error when targeting metadata IP, got nil")
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("expected error to indicate blocked address, got: %v", err)
		}
	})

	// 2. Loopback IP (127.0.0.1) must be blocked without explicit allowlist
	t.Run("blocks loopback 127.0.0.1 without allowlist", func(t *testing.T) {
		t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "")
		s3Config := domainBackup.BackupStorage{
			Bucket:   "test-bucket",
			Endpoint: "http://127.0.0.1:59999",
			Credentials: map[string]string{
				"access_key": "dummy",
				"secret_key": "dummy",
			},
		}

		s3Storage := storage.NewS3Storage(&s3Config)
		data := bytes.NewReader([]byte("test payload"))
		_, err := s3Storage.UploadStream(context.Background(), "backups/test.zst", data, 12, nil)
		if err == nil {
			t.Fatalf("expected SSRF error when targeting loopback IP, got nil")
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("expected error to indicate blocked address, got: %v", err)
		}
	})
}

