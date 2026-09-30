package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func TestFileLogSource_ListServices_FilterIgnoredLogs(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := []string{
		// Genuine services that should be returned
		"api-server.log",
		"web-frontend.log",
		"payment-processor.log",
		"auth-service.log",

		// Rotated log files that must be filtered out
		"vmware-network.1.log",
		"vmware-network.2.log",
		"api-server.1.log",
		"web-frontend.2.log",
		"payment-processor.log.1.log",

		// OS maintenance / installation noise that must be filtered out
		"alternatives.log",
		"apport.log",
		"dpkg.log",
		"bootstrap.log",
		"cloud-init.log",
		"cloud-init-output.log",
		"ubuntu-advantage.log",
		"dist-upgrade.log",
		"fontconfig.log",
		"faillog.log",
		"lastlog.log",
		"wtmp.log",
		"btmp.log",
		"vmware-network.log",

		// Non-log files
		"readme.txt",
	}

	for _, fname := range testFiles {
		p := filepath.Join(tempDir, fname)
		if err := os.WriteFile(p, []byte("2026-09-16T00:00:00Z [info] test log\n"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", fname, err)
		}
	}

	// Subdirectory with .log extension to verify directories are skipped
	if err := os.Mkdir(filepath.Join(tempDir, "directory.log"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	source := &FileLogSource{logDir: tempDir}
	services, err := source.ListServices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error from ListServices: %v", err)
	}

	sort.Strings(services)
	expectedServices := []string{
		"api-server",
		"auth-service",
		"payment-processor",
		"web-frontend",
	}
	sort.Strings(expectedServices)

	if len(services) != len(expectedServices) {
		t.Fatalf("expected %d services %v, got %d services %v",
			len(expectedServices), expectedServices, len(services), services)
	}

	for i := range expectedServices {
		if services[i] != expectedServices[i] {
			t.Errorf("at index %d: expected service %q, got %q", i, expectedServices[i], services[i])
		}
	}
}

func TestIsIgnoredLogFile(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		// Rotated log files
		{"vmware-network.1.log", true},
		{"vmware-network.2.log", true},
		{"vmware-network.7", true},
		{"api-server.1.log", true},
		{"app.log.1", true},
		{"app.log.2.gz", true},
		{"app.log.gz", true},
		{"app.log.old", true},
		{"app.log.bak", true},
		{"app.bak.log", true},
		{"app.old.log", true},
		{"payment-processor.log.1.log", true},

		// OS maintenance / installation noise
		{"alternatives.log", true},
		{"alternatives", true},
		{"apport.log", true},
		{"apport", true},
		{"dpkg.log", true},
		{"dpkg", true},
		{"bootstrap.log", true},
		{"bootstrap", true},
		{"cloud-init.log", true},
		{"cloud-init", true},
		{"cloud-init-output.log", true},
		{"cloud-init-output", true},
		{"ubuntu-advantage.log", true},
		{"ubuntu-advantage", true},
		{"dist-upgrade.log", true},
		{"dist-upgrade", true},
		{"fontconfig.log", true},
		{"fontconfig", true},
		{"faillog.log", true},
		{"faillog", true},
		{"lastlog.log", true},
		{"lastlog", true},
		{"wtmp.log", true},
		{"wtmp", true},
		{"btmp.log", true},
		{"btmp", true},
		{"vmware-network.log", true},
		{"vmware-network", true},

		// Case variations
		{"DPKG.LOG", true},
		{"Cloud-Init.LOG", true},
		{"VMWARE-NETWORK.1.LOG", true},

		// Genuine application logs (should NOT be ignored)
		{"api-server.log", false},
		{"web-frontend.log", false},
		{"order-processing.log", false},
		{"auth.log", false},
		{"nginx.log", false},
		{"postgres.log", false},
		{"redis.log", false},
		{"my-app.log", false},

		// Edge cases
		{"", true},
		{"/var/log/dpkg.log", true},
		{"/var/log/api-server.log", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := isIgnoredLogFile(tt.name)
			if actual != tt.expected {
				t.Errorf("isIgnoredLogFile(%q) = %v, expected %v", tt.name, actual, tt.expected)
			}
		})
	}
}

func TestFileLogSource_GetLogs_FilterIgnoredLogs(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "app.log"), []byte("2026-09-16T00:00:00Z [info] real log\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "vmware-network.1.log"), []byte("2026-09-16T00:00:00Z [info] noisy rotated log\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "dpkg.log"), []byte("2026-09-16T00:00:00Z [info] noisy os log\n"), 0644)

	source := &FileLogSource{logDir: tempDir}
	entries, err := source.GetLogs(context.Background(), "", 100, nil, nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry from app.log, got %d entries", len(entries))
	}
	if entries[0].Service != "app" {
		t.Errorf("expected service 'app', got %q", entries[0].Service)
	}
}

func TestFileLogSource_IngestToWriter_FilterIgnoredLogs(t *testing.T) {
	tempDir := t.TempDir()
	engineDir := filepath.Join(tempDir, "engine")
	_ = os.MkdirAll(engineDir, 0755)

	logDir := filepath.Join(tempDir, "logs")
	_ = os.MkdirAll(logDir, 0755)

	_ = os.WriteFile(filepath.Join(logDir, "order-service.log"), []byte("2026-09-16T00:00:00Z [info] order placed\n"), 0644)
	_ = os.WriteFile(filepath.Join(logDir, "vmware-network.1.log"), []byte("2026-09-16T00:00:00Z [info] rotated log\n"), 0644)
	_ = os.WriteFile(filepath.Join(logDir, "alternatives.log"), []byte("2026-09-16T00:00:00Z [info] alternatives log\n"), 0644)

	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(engineDir, dict)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	source := &FileLogSource{logDir: logDir, writer: writer}
	if err := source.IngestToWriter(context.Background(), writer); err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Verify that order-service was ingested, but ignored files were not
	services := dict.Export()
	var foundOrder, foundVmware, foundAlternatives bool
	for _, s := range services {
		if s == "order-service" {
			foundOrder = true
		}
		if s == "vmware-network.1" || s == "vmware-network" {
			foundVmware = true
		}
		if s == "alternatives" {
			foundAlternatives = true
		}
	}
	if !foundOrder {
		t.Errorf("expected 'order-service' in dict export %v", services)
	}
	if foundVmware {
		t.Errorf("did not expect 'vmware-network' or rotated file in dict export %v", services)
	}
	if foundAlternatives {
		t.Errorf("did not expect 'alternatives' in dict export %v", services)
	}
}

func TestFileLogSource_EmptyDir(t *testing.T) {
	source := &FileLogSource{logDir: ""}
	services, err := source.ListServices(context.Background())
	if err != nil || services != nil {
		t.Fatalf("expected nil, nil for empty dir, got %v, %v", services, err)
	}

	entries, err := source.GetLogs(context.Background(), "", 10, nil, nil, "", "")
	if err != nil || entries != nil {
		t.Fatalf("expected nil, nil for empty dir, got %v, %v", entries, err)
	}

	if err := source.IngestToWriter(context.Background(), nil); err != nil {
		t.Fatalf("expected nil error for empty dir/writer, got %v", err)
	}
}
