package drivers_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	pkgErrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestRedisDriver_Type(t *testing.T) {
	d := drivers.NewRedisDriver()
	if d.Type() != "redis" {
		t.Errorf("expected type 'redis', got '%s'", d.Type())
	}
}

func TestRedisDriver_Restore_DirectFileReplacement_DataDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "redis-test-dir-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	d := drivers.NewRedisDriver()
	opts := backup.RestoreOptions{
		DBType:   "redis",
		Host:     "127.0.0.1",
		Port:     6379,
		Database: "0",
		ExtraParams: map[string]string{
			"data_dir": tempDir,
		},
	}

	rdbPayload := []byte("REDIS0011\xfa\tredis-ver\x057.2.0\xff")
	reader := bytes.NewReader(rdbPayload)

	err = d.Restore(context.Background(), opts, reader)
	if err != nil {
		t.Fatalf("unexpected error during restore: %v", err)
	}

	targetFile := filepath.Join(tempDir, "dump.rdb")
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read restored rdb file: %v", err)
	}

	if !bytes.Equal(content, rdbPayload) {
		t.Errorf("restored rdb file content mismatch: got %v, want %v", content, rdbPayload)
	}
}

func TestRedisDriver_Restore_DirectFileReplacement_RDBPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "redis-test-dir-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	customPath := filepath.Join(tempDir, "custom-dump.rdb")

	d := drivers.NewRedisDriver()
	opts := backup.RestoreOptions{
		DBType:   "redis",
		Host:     "127.0.0.1",
		Port:     6379,
		Database: "0",
		ExtraParams: map[string]string{
			"rdb_path": customPath,
		},
	}

	rdbPayload := []byte("REDIS0011binarypayloaddata")
	reader := bytes.NewReader(rdbPayload)

	err = d.Restore(context.Background(), opts, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("failed to read custom rdb: %v", err)
	}

	if !bytes.Equal(content, rdbPayload) {
		t.Errorf("content mismatch")
	}
}

func TestRedisDriver_Restore_DryRun(t *testing.T) {
	d := drivers.NewRedisDriver()

	// Valid RDB header
	optsValid := backup.RestoreOptions{
		DBType: "redis",
		DryRun: true,
	}
	validReader := bytes.NewReader([]byte("REDIS0011dummydata"))
	if err := d.Restore(context.Background(), optsValid, validReader); err != nil {
		t.Errorf("expected dry run to pass for valid REDIS header, got: %v", err)
	}

	// Invalid RDB header
	optsInvalid := backup.RestoreOptions{
		DBType: "redis",
		DryRun: true,
	}
	invalidReader := bytes.NewReader([]byte("INVALIDDATA"))
	if err := d.Restore(context.Background(), optsInvalid, invalidReader); err == nil {
		t.Errorf("expected error for invalid RDB header in dry run, got nil")
	}
}

func TestRedisDriver_Restore_FallbackGracefulError(t *testing.T) {
	d := drivers.NewRedisDriver()
	opts := backup.RestoreOptions{
		DBType:   "redis",
		Host:     "remote-redis.internal",
		Port:     6379,
		Database: "0",
		ExtraParams: map[string]string{},
	}

	// Skip if /data or /var/lib/redis exists on the host
	if _, err := os.Stat("/data"); err == nil {
		t.Skip("skipping on environment where /data exists")
	}
	if _, err := os.Stat("/var/lib/redis"); err == nil {
		t.Skip("skipping on environment where /var/lib/redis exists")
	}

	reader := bytes.NewReader([]byte("REDIS0011somepayload"))
	err := d.Restore(context.Background(), opts, reader)
	if err == nil {
		t.Fatalf("expected error when no data_dir or rdb_path provided, got nil")
	}

	if !errors.Is(err, pkgErrors.ErrValidation) {
		t.Errorf("expected error to wrap ErrValidation, got: %v", err)
	}

	if !strings.Contains(err.Error(), "data_dir") && !strings.Contains(err.Error(), "rdb_path") {
		t.Errorf("expected error message to guide user to provide data_dir or rdb_path, got: %v", err)
	}
}
