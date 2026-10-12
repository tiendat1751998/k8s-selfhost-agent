package telegram_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/sre"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	"go.uber.org/zap"
)

func TestRunbookExecutor_Dispatch5Runbooks(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	executor := telegram.NewInteractiveRunbookExecutor(nil, logger)

	testCases := []struct {
		id       sre.RunbookID
		params   map[string]string
		contains string
	}{
		{
			id: sre.RunbookRollbackDeployment,
			params: map[string]string{
				"namespace":  "production",
				"deployment": "order-service",
			},
			contains: "Rollback deployment order-service",
		},
		{
			id: sre.RunbookFencingNode,
			params: map[string]string{
				"node": "worker-node-03",
			},
			contains: "worker-node-03 successfully cordoned",
		},
		{
			id: sre.RunbookRotateSecret,
			params: map[string]string{
				"secret_name": "db-credentials",
				"namespace":   "backend",
			},
			contains: "db-credentials in namespace backend rotated",
		},
		{
			id: sre.RunbookRestoreDrill,
			params: map[string]string{
				"database":  "postgres-main",
				"backup_id": "bak-20261012-001",
			},
			contains: "postgres-main (Backup: bak-20261012-001)",
		},
		{
			id: sre.RunbookRenewCertificates,
			params: map[string]string{
				"domain": "api.k8sselfhost.local",
			},
			contains: "api.k8sselfhost.local renewed successfully",
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.id), func(t *testing.T) {
			req := sre.RunbookExecutionRequest{
				RunbookID:   tc.id,
				TriggeredBy: "Telegram:@admin",
				Parameters:  tc.params,
			}
			res, err := executor.Execute(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error executing %s: %v", tc.id, err)
			}
			if res.Status != "SUCCESS" {
				t.Errorf("expected SUCCESS status, got %s", res.Status)
			}
			if !strings.Contains(res.Output, tc.contains) {
				t.Errorf("expected output to contain %q, got: %s", tc.contains, res.Output)
			}
		})
	}

	// Unknown Runbook
	t.Run("UnknownRunbook", func(t *testing.T) {
		req := sre.RunbookExecutionRequest{
			RunbookID:   sre.RunbookID("RUNBOOK-99-INVALID"),
			TriggeredBy: "Telegram:@admin",
		}
		_, err := executor.Execute(context.Background(), req)
		if err == nil {
			t.Fatal("expected error for unknown runbook")
		}
	})
}

func TestRunbookExecutor_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	executor := telegram.NewInteractiveRunbookExecutor(nil, logger)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	req := sre.RunbookExecutionRequest{
		RunbookID:   sre.RunbookRollbackDeployment,
		TriggeredBy: "Telegram:@admin",
		Parameters: map[string]string{
			"namespace":  "default",
			"deployment": "web",
		},
	}
	res, err := executor.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected timeout error for expired context")
	}
	if res == nil || res.Status != "TIMEOUT" {
		t.Fatalf("expected TIMEOUT status, got %+v", res)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error message to contain 'timed out', got: %v", err)
	}
}
