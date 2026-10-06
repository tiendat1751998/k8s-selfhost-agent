package drivers_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestSQLServerDriver_SQLInjection_Payloads_Rejected(t *testing.T) {
	driver := drivers.NewSQLServerDriver("sqlserver")
	ctx := context.Background()

	maliciousDatabases := []string{
		"test'; EXEC xp_cmdshell 'whoami'; --",
		"test]--",
		"db; DROP DATABASE master; --",
		"db\" OR 1=1",
		"../../etc/passwd",
		"db name with spaces",
		"db\\name",
		"db;SELECT 1;",
		"db[0]",
		"'; SHUTDOWN; --",
	}

	for _, dbName := range maliciousDatabases {
		t.Run("Dump rejected for: "+dbName, func(t *testing.T) {
			var buf bytes.Buffer
			opts := backup.DumpOptions{
				DBType:   "sqlserver",
				Host:     "127.0.0.1",
				Database: dbName,
				Username: "sa",
			}
			_, err := driver.Dump(ctx, opts, &buf)
			if err == nil {
				t.Fatalf("expected error for malicious database name %q, got nil", dbName)
			}
			if !errors.Is(err, domainerrors.ErrValidation) {
				t.Errorf("expected ErrValidation for %q, got: %v", dbName, err)
			}
		})

		t.Run("Restore rejected for: "+dbName, func(t *testing.T) {
			opts := backup.RestoreOptions{
				DBType:   "sqlserver",
				Host:     "127.0.0.1",
				Database: dbName,
				Username: "sa",
			}
			err := driver.Restore(ctx, opts, bytes.NewReader([]byte("test")))
			if err == nil {
				t.Fatalf("expected error for malicious database name %q, got nil", dbName)
			}
			if !errors.Is(err, domainerrors.ErrValidation) {
				t.Errorf("expected ErrValidation for %q, got: %v", dbName, err)
			}
		})

		t.Run("ValidateConnection rejected for: "+dbName, func(t *testing.T) {
			opts := backup.DumpOptions{
				DBType:   "sqlserver",
				Host:     "127.0.0.1",
				Database: dbName,
				Username: "sa",
			}
			err := driver.ValidateConnection(ctx, opts)
			if err == nil {
				t.Fatalf("expected error for malicious database name %q, got nil", dbName)
			}
			if !errors.Is(err, domainerrors.ErrValidation) {
				t.Errorf("expected ErrValidation for %q, got: %v", dbName, err)
			}
		})
	}
}

func TestSQLServerDriver_ValidDatabases_Accepted(t *testing.T) {
	driver := drivers.NewSQLServerDriver("sqlserver")
	ctx := context.Background()

	validDatabases := []string{
		"master",
		"app_db",
		"app-db",
		"app.db",
		"Db123",
	}

	for _, dbName := range validDatabases {
		t.Run("Dump accepts: "+dbName, func(t *testing.T) {
			var buf bytes.Buffer
			opts := backup.DumpOptions{
				DBType:   "sqlserver",
				Host:     "127.0.0.1",
				Database: dbName,
				Username: "sa",
			}
			_, err := driver.Dump(ctx, opts, &buf)
			// Should NOT fail on validation error; may fail on binary not found or connection error
			if errors.Is(err, domainerrors.ErrValidation) {
				t.Errorf("expected valid db %q not to produce ErrValidation, got: %v", dbName, err)
			}
		})
	}
}
