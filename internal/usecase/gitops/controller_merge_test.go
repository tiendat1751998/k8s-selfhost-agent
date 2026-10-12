package gitops

import (
	"context"
	"errors"
	"testing"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/stringutil"
)

func TestMergePR_AtomicSuccess(t *testing.T) {
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	txMgr := &mockTxManager{}

	inc, _ := incident.New("system", "production", "pod-1", incident.TypeOOMKilled, incident.SeverityCritical, "OOM")
	inc.ID = "inc-1"
	_ = incRepo.Create(context.Background(), inc)

	pr, _ := domainGitops.New(inc.ID, domainGitops.ProviderGitHub, "https://github.com/org/repo", "fix-branch", "main", "Fix Title", "Fix Body")
	pr.ID = "pr-1"
	_ = pr.MarkOpen("https://github.com/org/repo/pull/1", 1)
	_ = prRepo.Create(context.Background(), pr)

	ctrl := NewController(prRepo, incRepo, txMgr)

	mergedPR, err := ctrl.MergePR(context.Background(), pr.ID)
	if err != nil {
		t.Fatalf("expected MergePR to succeed, got: %v", err)
	}

	if mergedPR.Status != domainGitops.PRStatusMerged {
		t.Errorf("expected PR status merged, got: %s", mergedPR.Status)
	}

	updatedInc, _ := incRepo.GetByID(context.Background(), inc.ID)
	if updatedInc.Status != incident.StatusResolved {
		t.Errorf("expected incident status resolved, got: %s", updatedInc.Status)
	}

	if txMgr.txCount != 1 {
		t.Errorf("expected 1 transaction execution, got: %d", txMgr.txCount)
	}
}

func TestMergePR_AtomicRollbackOnIncidentUpdateFail(t *testing.T) {
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	incRepo.failUpdate = true
	txMgr := &mockTxManager{}

	inc, _ := incident.New("default", "production", "pod-1", incident.TypeOOMKilled, incident.SeverityCritical, "OOM")
	inc.ID = "inc-1"
	_ = incRepo.Create(context.Background(), inc)

	pr, _ := domainGitops.New(inc.ID, domainGitops.ProviderGitHub, "https://github.com/org/repo", "fix-branch", "main", "Fix Title", "Fix Body")
	pr.ID = "pr-1"
	_ = pr.MarkOpen("https://github.com/org/repo/pull/1", 1)
	_ = prRepo.Create(context.Background(), pr)

	ctrl := NewController(prRepo, incRepo, txMgr)

	_, err := ctrl.MergePR(context.Background(), pr.ID)
	if err == nil {
		t.Fatal("expected MergePR to fail when incident update fails")
	}

	if txMgr.txCount != 1 {
		t.Errorf("expected 1 transaction execution, got: %d", txMgr.txCount)
	}
}

func TestMergePR_RemoteProviderSuccess(t *testing.T) {
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	txMgr := &mockTxManager{}
	provider := &mockGitProvider{}

	inc, _ := incident.New("system", "prod", "api-1", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crash")
	inc.ID = "inc-remote-1"
	_ = incRepo.Create(context.Background(), inc)

	pr, _ := domainGitops.New(inc.ID, domainGitops.ProviderGitHub, "https://github.com/example/repo", "fix/crash", "main", "Fix crash", "body")
	pr.ID = "pr-remote-1"
	_ = pr.MarkOpen("https://github.com/example/repo/pull/101", 101)
	_ = prRepo.Create(context.Background(), pr)

	ctrl := NewController(prRepo, incRepo, txMgr)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	mergedPR, err := ctrl.MergePR(context.Background(), pr.ID)
	if err != nil {
		t.Fatalf("unexpected MergePR failure: %v", err)
	}

	if !provider.mergeCalled {
		t.Error("expected provider.MergePullRequest to be invoked")
	}
	if mergedPR.Status != domainGitops.PRStatusMerged {
		t.Errorf("expected PR status merged, got: %s", mergedPR.Status)
	}

	storedPR, _ := prRepo.GetByID(context.Background(), pr.ID)
	if storedPR.Status != domainGitops.PRStatusMerged {
		t.Errorf("expected DB PR to be updated to merged, got %s", storedPR.Status)
	}
}

func TestMergePR_RemoteProviderFailure_NoDBUpdate(t *testing.T) {
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	txMgr := &mockTxManager{}
	provider := &mockGitProvider{
		mergeErr: errors.New("upstream GitHub API rate limit exceeded / 409 conflict"),
	}

	inc, _ := incident.New("system", "prod", "api-1", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "crash")
	inc.ID = "inc-remote-fail"
	_ = incRepo.Create(context.Background(), inc)

	pr, _ := domainGitops.New(inc.ID, domainGitops.ProviderGitHub, "https://github.com/example/repo", "fix/crash", "main", "Fix crash", "body")
	pr.ID = "pr-remote-fail"
	_ = pr.MarkOpen("https://github.com/example/repo/pull/102", 102)
	_ = prRepo.Create(context.Background(), pr)

	ctrl := NewController(prRepo, incRepo, txMgr)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	_, err := ctrl.MergePR(context.Background(), pr.ID)
	if err == nil {
		t.Fatal("expected MergePR to fail when remote provider fails")
	}
	if !contains(err.Error(), "remote git provider failed to merge PR") {
		t.Errorf("expected error citing remote git provider failure, got: %v", err)
	}

	// Verify DB was NOT updated (transaction must NOT be executed)
	if txMgr.txCount != 0 {
		t.Errorf("expected 0 DB transactions when remote merge fails, got %d", txMgr.txCount)
	}

	storedPR, _ := prRepo.GetByID(context.Background(), pr.ID)
	if storedPR.Status != domainGitops.PRStatusOpen {
		t.Errorf("expected PR in DB to remain open after remote merge failure, got: %s", storedPR.Status)
	}

	storedInc, _ := incRepo.GetByID(context.Background(), inc.ID)
	if storedInc.Status == incident.StatusResolved {
		t.Error("incident was prematurely resolved in DB despite remote merge failure")
	}
}

func TestController_ClosePR(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	ctrl := NewController(prRepo, incRepo, nil)

	pr, _ := domainGitops.New("inc-1", domainGitops.ProviderGitHub, "https://github.com/org/repo", "branch", "main", "title", "desc")
	_ = pr.MarkOpen("https://github.com/org/repo/pull/2", 2)
	_ = prRepo.Create(ctx, pr)

	t.Run("success", func(t *testing.T) {
		closedPR, err := ctrl.ClosePR(ctx, pr.ID)
		if err != nil {
			t.Fatalf("unexpected error closing PR: %v", err)
		}
		if closedPR.Status != domainGitops.PRStatusClosed {
			t.Errorf("expected status 'closed', got '%s'", closedPR.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := ctrl.ClosePR(ctx, "nonexistent-id")
		if err == nil {
			t.Error("expected error for nonexistent PR ID")
		}
	})
}

func TestBuildPRBody(t *testing.T) {
	inc, _ := incident.New("default", "production", "api-server-xyz123", incident.TypeOOMKilled, incident.SeverityCritical, "Container 'api' was OOMKilled")
	inc.ID = "abc12345-6789-0123-4567-890abcdef012"

	rpt, _ := report.New(
		inc.ID,
		"Memory limit exceeded due to memory leak in handler",
		[]string{"OOMKilled event at 12:00", "Memory usage at 98%"},
		0.85,
		report.RiskHigh,
		"Increase memory limit to 512Mi",
		"Revert to previous deployment",
	)

	body := buildPRBody(inc, rpt)

	if body == "" {
		t.Fatal("expected non-empty PR body")
	}

	checks := []string{
		"Auto-Remediation PR",
		"Incident Details",
		"Root Cause Analysis",
		"Memory limit exceeded",
		"Rollback Plan",
		"K8S Self-Healing Agent",
	}

	for _, check := range checks {
		if !contains(body, check) {
			t.Errorf("expected PR body to contain '%s'", check)
		}
	}
}

func TestFormatEvidence(t *testing.T) {
	evidence := []string{"OOMKilled event", "Memory at 98%"}
	result := formatEvidence(evidence)

	if !contains(result, "- OOMKilled event") {
		t.Error("expected formatted evidence to contain first item")
	}
	if !contains(result, "- Memory at 98%") {
		t.Error("expected formatted evidence to contain second item")
	}
}

func TestFormatEvidence_Empty(t *testing.T) {
	result := formatEvidence(nil)
	if result != "" {
		t.Errorf("expected empty string for nil evidence, got '%s'", result)
	}
}

func TestTruncateStr(t *testing.T) {
	if stringutil.Truncate("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}

	result := stringutil.Truncate("this is a very long string that needs truncation", 20)
	if len(result) > 24 {
		t.Errorf("truncated string too long: %s", result)
	}
}
