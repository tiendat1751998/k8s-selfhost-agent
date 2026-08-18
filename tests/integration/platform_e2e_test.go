package integration_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/drivers"
	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/dualsync"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/security"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
)

// TestPlatform_FullLifecycleIntegration validates the integration between:
// 1. Database Backup & Dual-Target Compression
// 2. Incident & Alert Storm Debouncing via SRE Telegram Bot
// 3. Real-Time Log Streaming via RingBuffer
// 4. Container Vulnerability & IaC Security Gate
// 5. Terraform & Ansible Infrastructure Automation
func TestPlatform_FullLifecycleIntegration(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// -------------------------------------------------------------
	// 1. DATABASE BACKUP & RESTORE DUAL-SYNC TEST
	// -------------------------------------------------------------
	t.Run("Module 1: Dual-Target DB Backup & Restore", func(t *testing.T) {
		var primaryBuf bytes.Buffer
		var secondaryBuf bytes.Buffer

		dw := &dualsync.DualWriter{
			TargetA: &primaryBuf,
			TargetB: &secondaryBuf,
		}

		pipeCfg := dualsync.PipeConfig{
			CompressionLevel: 3,
			EnableEncryption: false,
		}

		pipe, pipeCloser, err := dualsync.NewProcessingPipe(dw, pipeCfg)
		if err != nil {
			t.Fatalf("failed to create processing pipe: %v", err)
		}

		sampleSQL := []byte("CREATE TABLE enterprise_users (id INT, name VARCHAR(100));\nINSERT INTO enterprise_users VALUES (1, 'Admin');\n")
		_, err = pipeCloser.Write(sampleSQL)
		if err != nil {
			t.Fatalf("failed to write to pipe: %v", err)
		}
		_ = pipeCloser.Close()

		rawSize, compSize, checksum := pipe.Summary()
		if rawSize != int64(len(sampleSQL)) || compSize == 0 || len(checksum) != 64 {
			t.Errorf("invalid metrics: raw=%d, comp=%d, checksum=%s", rawSize, compSize, checksum)
		}

		// Verify dual targets received exact same compressed bytes
		if !bytes.Equal(primaryBuf.Bytes(), secondaryBuf.Bytes()) {
			t.Errorf("primary and secondary dual-sync payloads mismatch")
		}

		// Verify 8 database drivers in registry
		registry := drivers.NewDriverRegistry()
		for _, db := range []string{"postgres", "mysql", "mariadb", "sqlserver", "mssql", "oracle", "mongodb", "redis"} {
			if _, err := registry.Get(db); err != nil {
				t.Errorf("driver for %s not found in registry", db)
			}
		}
	})

	// -------------------------------------------------------------
	// 2. TELEGRAM SRE COPILOT ALERT DEBOUNCER TEST
	// -------------------------------------------------------------
	t.Run("Module 2: SRE Telegram Bot Alert Debouncing & RCA", func(t *testing.T) {
		var receivedAlerts []*telegram.AlertPayload
		debouncer := telegram.NewAlertDebouncer(30*time.Millisecond, func(alert *telegram.AlertPayload) {
			receivedAlerts = append(receivedAlerts, alert)
		})

		for i := 1; i <= 5; i++ {
			debouncer.Push(&telegram.AlertPayload{
				IncidentID: "inc-001",
				Severity:   "CRITICAL",
				Cluster:    "prod-cluster",
				Namespace:  "default",
				Service:    "payment-api",
				Message:    "Pod CrashLoopBackOff OOMKilled",
			})
		}

		time.Sleep(60 * time.Millisecond)

		if len(receivedAlerts) != 1 {
			t.Fatalf("expected 1 aggregated alert, got %d", len(receivedAlerts))
		}
		if receivedAlerts[0].Count != 5 {
			t.Errorf("expected count 5, got %d", receivedAlerts[0].Count)
		}
	})

	// -------------------------------------------------------------
	// 3. LOGGING RING BUFFER & LIVE TAIL TEST
	// -------------------------------------------------------------
	t.Run("Module 3: Real-Time Log Explorer RingBuffer & PubSub", func(t *testing.T) {
		agg := logging.NewLogAggregator(100)
		sub, _ := agg.Subscribe("sub-1", logging.LogFilter{Namespace: "prod", Pod: "backend-1"}, 10)
		defer agg.Unsubscribe("sub-1")

		agg.Ingest(logging.LogEntry{
			Namespace: "prod",
			Pod:       "backend-1",
			Container: "app",
			Level:     "INFO",
			Message:   "Application server listening on :8080",
		})

		select {
		case entry := <-sub.Ch:
			if entry.Message != "Application server listening on :8080" {
				t.Errorf("unexpected log message: %s", entry.Message)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for live log stream")
		}
	})

	// -------------------------------------------------------------
	// 4. DEVSECOPS TRIVY & CHECKOV SCANNERS TEST
	// -------------------------------------------------------------
	t.Run("Module 5: DevSecOps CVE Scanning & Security Gate", func(t *testing.T) {
		trivy := security.NewTrivyScanner()
		report, err := trivy.ScanImage(ctx, "k8sselfhost/backend:latest", "HIGH")
		if err != nil {
			t.Fatalf("Trivy scan failed: %v", err)
		}
		if !report.PassSecurityGate {
			t.Errorf("expected clean baseline to pass security gate")
		}

		checkov := security.NewCheckovScanner()
		iacReport, err := checkov.ScanDirectory(ctx, filepath.Join(tmpDir, "deploy"), "kubernetes")
		if err != nil {
			t.Fatalf("Checkov scan failed: %v", err)
		}
		if iacReport.ScorePercentage < 0 || iacReport.ScorePercentage > 100 {
			t.Errorf("invalid score percentage: %f", iacReport.ScorePercentage)
		}
	})

	// -------------------------------------------------------------
	// 5. IAC TERRAFORM & ANSIBLE RUNNERS TEST
	// -------------------------------------------------------------
	t.Run("Module 6: Infrastructure as Code Runners", func(t *testing.T) {
		tf := iac.NewTerraformRunner("")
		tfRes, err := tf.Plan(ctx, iac.TerraformRunOptions{WorkingDir: tmpDir}, nil)
		if err != nil || !tfRes.Success {
			t.Errorf("terraform runner plan failed: %v", err)
		}

		ansible := iac.NewAnsibleRunner("")
		ansRes, err := ansible.RunPlaybook(ctx, iac.AnsiblePlaybookOptions{PlaybookFile: "deploy/ansible/hardening.yaml"}, nil)
		if err != nil || !ansRes.Success {
			t.Errorf("ansible runner failed: %v", err)
		}
	})
}
