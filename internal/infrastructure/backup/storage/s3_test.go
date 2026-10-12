package storage_test

import (
	"bytes"
	"context"
	"io"
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

func TestS3Storage_UploadStream_PreventsHTTP411_RequiresContentLength(t *testing.T) {
	t.Setenv("EGRESS_ALLOW_PRIVATE_CIDRS", "127.0.0.0/8")

	// Mock S3 server enforcing strict Content-Length (like AWS S3)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// If Content-Length is missing, 0 with payload, or chunked transfer encoding, AWS returns 411
		if r.Header.Get("Content-Length") == "" && r.ContentLength <= 0 {
			w.WriteHeader(http.StatusLengthRequired)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingContentLength</Code><Message>You must provide the Content-Length HTTP header.</Message></Error>`))
			return
		}

		if len(r.TransferEncoding) > 0 {
			for _, enc := range r.TransferEncoding {
				if enc == "chunked" {
					w.WriteHeader(http.StatusLengthRequired)
					w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingContentLength</Code><Message>Chunked transfer without explicit length rejected.</Message></Error>`))
					return
				}
			}
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s3Config := domainBackup.BackupStorage{
		Bucket:   "prod-backups",
		Endpoint: ts.URL,
		Credentials: map[string]string{
			"access_key": "testkey",
			"secret_key": "testsecret",
		},
	}
	s3Storage := storage.NewS3Storage(&s3Config)

	// Subtest 1: Seekable reader with size 0 (unknown size passed by caller)
	t.Run("seekable reader with size 0", func(t *testing.T) {
		data := bytes.NewReader([]byte("seekable test payload"))
		path, err := s3Storage.UploadStream(context.Background(), "backups/seekable.zst", data, 0, nil)
		if err != nil {
			t.Fatalf("expected successful upload without HTTP 411, got: %v", err)
		}
		if path != "s3://prod-backups/backups/seekable.zst" {
			t.Errorf("expected path 's3://prod-backups/backups/seekable.zst', got '%s'", path)
		}
	})

	// Subtest 2: Non-seekable streaming pipe with size 0
	t.Run("streaming pipe reader with size 0", func(t *testing.T) {
		pr2, pw2 := io.Pipe()
		go func() {
			_, _ = pw2.Write([]byte("streaming pipe content chunk 12345"))
			_ = pw2.Close()
		}()

		path, err := s3Storage.UploadStream(context.Background(), "backups/stream.zst", pr2, 0, nil)
		if err != nil {
			t.Fatalf("expected successful upload without HTTP 411, got: %v", err)
		}
		if path != "s3://prod-backups/backups/stream.zst" {
			t.Errorf("expected path 's3://prod-backups/backups/stream.zst', got '%s'", path)
		}
	})
}


