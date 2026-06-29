package gitops

import "testing"

func TestNew_ValidPR(t *testing.T) {
	pr, err := New(
		"incident-123",
		ProviderGitHub,
		"https://github.com/org/repo",
		"fix/incident-123",
		"main",
		"fix: increase memory limit for nginx",
		"Auto-generated fix for OOMKilled incident",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pr.IncidentID != "incident-123" {
		t.Errorf("expected incident_id 'incident-123', got '%s'", pr.IncidentID)
	}
	if pr.Provider != ProviderGitHub {
		t.Errorf("expected provider 'github', got '%s'", pr.Provider)
	}
	if pr.Status != PRStatusPending {
		t.Errorf("expected status 'pending', got '%s'", pr.Status)
	}
}

func TestNew_EmptyIncidentID(t *testing.T) {
	_, err := New("", ProviderGitHub, "url", "branch", "main", "title", "desc")
	if err == nil {
		t.Fatal("expected validation error for empty incident_id")
	}
}

func TestNew_EmptyRepoURL(t *testing.T) {
	_, err := New("inc-1", ProviderGitHub, "", "branch", "main", "title", "desc")
	if err == nil {
		t.Fatal("expected validation error for empty repo_url")
	}
}

func TestNew_EmptyBranch(t *testing.T) {
	_, err := New("inc-1", ProviderGitHub, "url", "", "main", "title", "desc")
	if err == nil {
		t.Fatal("expected validation error for empty branch")
	}
}

func TestNew_EmptyTitle(t *testing.T) {
	_, err := New("inc-1", ProviderGitHub, "url", "branch", "main", "", "desc")
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
}

func TestPR_StateTransitions(t *testing.T) {
	pr, _ := New("inc-1", ProviderGitHub, "url", "branch", "main", "title", "desc")

	// pending -> open
	if err := pr.MarkOpen("https://github.com/org/repo/pull/42", 42); err != nil {
		t.Fatalf("unexpected error marking open: %v", err)
	}
	if pr.Status != PRStatusOpen {
		t.Errorf("expected status 'open', got '%s'", pr.Status)
	}
	if pr.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", pr.PRNumber)
	}

	// open -> merged
	if err := pr.MarkMerged(); err != nil {
		t.Fatalf("unexpected error marking merged: %v", err)
	}
	if pr.Status != PRStatusMerged {
		t.Errorf("expected status 'merged', got '%s'", pr.Status)
	}
	if pr.MergedAt == nil {
		t.Error("expected merged_at to be set")
	}
}

func TestPR_InvalidTransition(t *testing.T) {
	pr, _ := New("inc-1", ProviderGitHub, "url", "branch", "main", "title", "desc")

	// Cannot merge from pending
	if err := pr.MarkMerged(); err == nil {
		t.Error("expected error merging from pending status")
	}

	// Cannot open twice
	_ = pr.MarkOpen("url", 1)
	if err := pr.MarkOpen("url2", 2); err == nil {
		t.Error("expected error opening an already open PR")
	}
}

func TestPR_AddFileChange(t *testing.T) {
	pr, _ := New("inc-1", ProviderGitHub, "url", "branch", "main", "title", "desc")
	pr.AddFileChange("deploy/nginx.yaml", "apiVersion: apps/v1...", FileActionModify)
	pr.AddFileChange("deploy/hpa.yaml", "apiVersion: autoscaling/v2...", FileActionCreate)

	if len(pr.FilesChanged) != 2 {
		t.Errorf("expected 2 file changes, got %d", len(pr.FilesChanged))
	}
	if pr.FilesChanged[0].Action != FileActionModify {
		t.Errorf("expected first file action 'modify', got '%s'", pr.FilesChanged[0].Action)
	}
}

func TestPR_MarkFailed(t *testing.T) {
	pr, _ := New("inc-1", ProviderGitHub, "url", "branch", "main", "title", "desc")
	pr.MarkFailed()
	if pr.Status != PRStatusFailed {
		t.Errorf("expected status 'failed', got '%s'", pr.Status)
	}
}
