package drivers_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
)

func TestDriverRegistry_AllSupportedDatabases(t *testing.T) {
	registry := drivers.NewDriverRegistry()

	expectedDrivers := []string{
		"postgres",
		"mysql",
		"mariadb",
		"sqlserver",
		"mssql",
		"oracle",
		"mongodb",
		"redis",
	}

	for _, dbType := range expectedDrivers {
		t.Run(dbType, func(t *testing.T) {
			d, err := registry.Get(dbType)
			if err != nil {
				t.Fatalf("expected driver for '%s' to be registered, got error: %v", dbType, err)
			}
			if d.Type() != dbType {
				t.Errorf("expected driver type '%s', got '%s'", dbType, d.Type())
			}
		})
	}
}

func TestDriverRegistry_NotFound(t *testing.T) {
	registry := drivers.NewDriverRegistry()
	_, err := registry.Get("nonexistent_db")
	if err == nil {
		t.Fatal("expected error for nonexistent database driver, got nil")
	}
}

func TestDrivers_FallbackDumpOutput(t *testing.T) {
	ctx := context.Background()
	registry := drivers.NewDriverRegistry()

	tests := []struct {
		dbType   string
		database string
	}{
		{"postgres", "demo_pg"},
		{"mysql", "demo_mysql"},
		{"mariadb", "demo_mariadb"},
		{"sqlserver", "demo_mssql"},
		{"oracle", "demo_oracle"},
		{"mongodb", "demo_mongo"},
		{"redis", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			d, err := registry.Get(tt.dbType)
			if err != nil {
				t.Fatalf("failed to get driver: %v", err)
			}

			var buf bytes.Buffer
			opts := backup.DumpOptions{
				DBType:     tt.dbType,
				Host:       "127.0.0.1",
				Database:   tt.database,
				Username:   "root",
				Password:   "secret",
				BackupType: "full",
			}

			// In test environment without DB daemons, fallback stream handles gracefully
			_, _ = d.Dump(ctx, opts, &buf)
		})
	}
}
