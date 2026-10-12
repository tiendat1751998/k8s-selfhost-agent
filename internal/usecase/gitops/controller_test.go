package gitops

import (
	"context"
	"errors"
	"testing"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

func TestCreateRemediationPR_HappyPath(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	provider := &mockGitProvider{}

	ctrl := NewController(prRepo, incRepo, nil)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	inc, rpt := createTestIncidentAndReport(t)
	_ = incRepo.Create(ctx, inc)

	files := []FileCommit{
		{Path: "deploy/manifest.yaml", Content: "apiVersion: apps/v1\nkind: Deployment"},
	}

	pr, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitHub, "https://github.com/org/repo", "main", files)
	if err != nil {
		t.Fatalf("unexpected error creating remediation PR: %v", err)
	}

	if pr == nil {
		t.Fatal("expected non-nil PR")
	}
	if pr.Status != domainGitops.PRStatusOpen {
		t.Errorf("expected PR status 'open', got '%s'", pr.Status)
	}
	if pr.PRURL != "https://github.com/example/repo/pull/42" {
		t.Errorf("expected PR URL 'https://github.com/example/repo/pull/42', got '%s'", pr.PRURL)
	}
	if pr.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", pr.PRNumber)
	}
	if pr.Branch != "fix/incident-12345678" {
		t.Errorf("expected branch 'fix/incident-12345678', got '%s'", pr.Branch)
	}
	if len(pr.FilesChanged) != 1 {
		t.Errorf("expected 1 file changed, got %d", len(pr.FilesChanged))
	}

	if !provider.branchCreated {
		t.Error("expected provider.CreateBranch to be called")
	}
	if !provider.filesCommitted {
		t.Error("expected provider.CommitFiles to be called")
	}
	if !provider.prCreated {
		t.Error("expected provider.CreatePullRequest to be called")
	}

	if inc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status 'remediating', got '%s'", inc.Status)
	}
	if incRepo.updatedInc == nil {
		t.Error("expected incident repo Update to be called")
	}
}

func TestCreateRemediationPR_BranchCreationFailure(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	provider := &mockGitProvider{
		createBranchErr: errors.New("remote rejected: permission denied"),
	}

	ctrl := NewController(prRepo, incRepo, nil)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	inc, rpt := createTestIncidentAndReport(t)
	_ = incRepo.Create(ctx, inc)

	files := []FileCommit{
		{Path: "deploy/manifest.yaml", Content: "apiVersion: apps/v1"},
	}

	pr, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitHub, "https://github.com/org/repo", "main", files)
	if err == nil {
		t.Fatal("expected error on branch creation failure, got nil")
	}
	if !contains(err.Error(), "creating branch") {
		t.Errorf("expected error to contain 'creating branch', got: %v", err)
	}
	if pr != nil {
		t.Errorf("expected nil PR on failure, got: %+v", pr)
	}

	if prRepo.updatedPR == nil {
		t.Fatal("expected PR to be updated with failed status")
	}
	if prRepo.updatedPR.Status != domainGitops.PRStatusFailed {
		t.Errorf("expected updated PR status 'failed', got '%s'", prRepo.updatedPR.Status)
	}

	if provider.filesCommitted {
		t.Error("provider.CommitFiles should not be called when branch creation fails")
	}
	if provider.prCreated {
		t.Error("provider.CreatePullRequest should not be called when branch creation fails")
	}
}

func TestCreateRemediationPR_CommitFilesFailure(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	provider := &mockGitProvider{
		commitFilesErr: errors.New("failed to write tree: disk quota exceeded"),
	}

	ctrl := NewController(prRepo, incRepo, nil)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	inc, rpt := createTestIncidentAndReport(t)
	_ = incRepo.Create(ctx, inc)

	files := []FileCommit{
		{Path: "deploy/manifest.yaml", Content: "apiVersion: apps/v1"},
	}

	pr, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitHub, "https://github.com/org/repo", "main", files)
	if err == nil {
		t.Fatal("expected error on commit files failure, got nil")
	}
	if !contains(err.Error(), "committing files") {
		t.Errorf("expected error to contain 'committing files', got: %v", err)
	}
	if pr != nil {
		t.Errorf("expected nil PR on failure, got: %+v", pr)
	}

	if !provider.branchCreated {
		t.Error("expected provider.CreateBranch to have been called")
	}
	if prRepo.updatedPR == nil {
		t.Fatal("expected PR to be updated with failed status")
	}
	if prRepo.updatedPR.Status != domainGitops.PRStatusFailed {
		t.Errorf("expected updated PR status 'failed', got '%s'", prRepo.updatedPR.Status)
	}
	if provider.prCreated {
		t.Error("provider.CreatePullRequest should not be called when commit fails")
	}
}

func TestCreateRemediationPR_PROpeningFailure(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()
	provider := &mockGitProvider{
		createPRErr: errors.New("github api: rate limit exceeded"),
	}

	ctrl := NewController(prRepo, incRepo, nil)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	inc, rpt := createTestIncidentAndReport(t)
	_ = incRepo.Create(ctx, inc)

	files := []FileCommit{
		{Path: "deploy/manifest.yaml", Content: "apiVersion: apps/v1"},
	}

	pr, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitHub, "https://github.com/org/repo", "main", files)
	if err == nil {
		t.Fatal("expected error on PR opening failure, got nil")
	}
	if !contains(err.Error(), "creating pull request") {
		t.Errorf("expected error to contain 'creating pull request', got: %v", err)
	}
	if pr != nil {
		t.Errorf("expected nil PR on failure, got: %+v", pr)
	}

	if !provider.branchCreated {
		t.Error("expected provider.CreateBranch to have succeeded")
	}
	if !provider.filesCommitted {
		t.Error("expected provider.CommitFiles to have succeeded")
	}
	if prRepo.updatedPR == nil {
		t.Fatal("expected PR to be updated with failed status")
	}
	if prRepo.updatedPR.Status != domainGitops.PRStatusFailed {
		t.Errorf("expected updated PR status 'failed', got '%s'", prRepo.updatedPR.Status)
	}
}

func TestCreateRemediationPR_DBPersistFailure(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	prRepo.createErr = errors.New("database connection failed")
	incRepo := newMockIncidentRepo()
	provider := &mockGitProvider{}

	ctrl := NewController(prRepo, incRepo, nil)
	ctrl.RegisterProvider(domainGitops.ProviderGitHub, provider)

	inc, rpt := createTestIncidentAndReport(t)
	_ = incRepo.Create(ctx, inc)

	files := []FileCommit{
		{Path: "deploy/manifest.yaml", Content: "apiVersion: apps/v1"},
	}

	pr, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitHub, "https://github.com/org/repo", "main", files)
	if err == nil {
		t.Fatal("expected error on DB persist failure, got nil")
	}
	if !contains(err.Error(), "persisting PR record") {
		t.Errorf("expected error to contain 'persisting PR record', got: %v", err)
	}
	if pr != nil {
		t.Errorf("expected nil PR on failure, got: %+v", pr)
	}

	if provider.branchCreated {
		t.Error("provider.CreateBranch should not be called when DB persistence fails")
	}
	if provider.filesCommitted {
		t.Error("provider.CommitFiles should not be called when DB persistence fails")
	}
	if provider.prCreated {
		t.Error("provider.CreatePullRequest should not be called when DB persistence fails")
	}
}

func TestCreateRemediationPR_UnsupportedProvider(t *testing.T) {
	ctx := context.Background()
	prRepo := newMockPRRepo()
	incRepo := newMockIncidentRepo()

	ctrl := NewController(prRepo, incRepo, nil)

	inc, rpt := createTestIncidentAndReport(t)

	_, err := ctrl.CreateRemediationPR(ctx, inc, rpt, domainGitops.ProviderGitLab, "https://gitlab.com/org/repo", "main", nil)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !contains(err.Error(), "unsupported git provider") {
		t.Errorf("expected error to contain 'unsupported git provider', got: %v", err)
	}
}
