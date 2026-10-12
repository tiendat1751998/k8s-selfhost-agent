package drivers_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
)

func TestPostgresDriver_Restore_DryRun_NoDrop(t *testing.T) {
	d := drivers.NewPostgresDriver()
	ctx := context.Background()

	t.Run("DryRun with CleanTarget true never executes drop or clean and passes valid PGDMP archive", func(t *testing.T) {
		// Valid PostgreSQL custom dump magic header: PGDMP
		validCustomPayload := append([]byte("PGDMP"), []byte("\x01\x14\x00\x00\x00data")...)
		opts := backup.RestoreOptions{
			DBType:      "postgres",
			Host:        "127.0.0.1",
			Port:        5432,
			Database:    "production_db",
			Username:    "postgres",
			DryRun:      true,
			CleanTarget: true, // Should be ignored in DryRun
		}

		err := d.Restore(ctx, opts, bytes.NewReader(validCustomPayload))
		if err != nil {
			t.Fatalf("expected dry run to succeed for valid PGDMP header, got: %v", err)
		}
	})

	t.Run("DryRun passes valid SQL preamble dump", func(t *testing.T) {
		validSQLPayload := []byte("-- PostgreSQL database dump\n-- Dumped by pg_dump version 15.0\n")
		opts := backup.RestoreOptions{
			DBType:      "postgres",
			Host:        "127.0.0.1",
			Port:        5432,
			Database:    "production_db",
			Username:    "postgres",
			DryRun:      true,
			CleanTarget: true,
		}

		err := d.Restore(ctx, opts, bytes.NewReader(validSQLPayload))
		if err != nil {
			t.Fatalf("expected dry run to succeed for valid SQL preamble, got: %v", err)
		}
	})

	t.Run("DryRun rejects corrupted archive header", func(t *testing.T) {
		corruptPayload := []byte("CORRUPT_ARCHIVE_DATA_NOT_POSTGRES")
		opts := backup.RestoreOptions{
			DBType:      "postgres",
			Host:        "127.0.0.1",
			Port:        5432,
			Database:    "production_db",
			Username:    "postgres",
			DryRun:      true,
			CleanTarget: false,
		}

		err := d.Restore(ctx, opts, bytes.NewReader(corruptPayload))
		if err == nil {
			t.Fatal("expected validation error for corrupted archive in dry run, got nil")
		}
		if !strings.Contains(err.Error(), "invalid PostgreSQL archive header") && !strings.Contains(err.Error(), "archive") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}
