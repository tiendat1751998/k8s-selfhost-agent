package gitops

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/stringutil"
)

type mockTxManager struct {
	txCount int
}

func (m *mockTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.txCount++
	return fn(ctx)
}

func (m *mockTxManager) RunInTxWithOpts(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	m.txCount++
	return fn(ctx)
}

type mockGitProvider struct {
	name            string
	createBranchErr error
	commitFilesSHA  string
	commitFilesErr  error
	createPRURL     string
	createPRNumber  int
	createPRErr     error

	branchCreated  bool
	branchRepo     string
	branchName     string
	branchBase     string

	filesCommitted bool
	commitRepo     string
	commitBranch   string
	commitMsg      string
	committedFiles []FileCommit

	prCreated    bool
	prRepo       string
	prBranch     string
	prBaseBranch string
	prTitle      string
	prBody       string
}

func (m *mockGitProvider) CreateBranch(ctx context.Context, repo, branch, baseBranch string) error {
	if m.createBranchErr != nil {
		return m.createBranchErr
	}
	m.branchCreated = true
	m.branchRepo = repo
	m.branchName = branch
	m.branchBase = baseBranch
	return nil
}

func (m *mockGitProvider) CommitFiles(ctx context.Context, repo, branch, message string, files []FileCommit) (string, error) {
	if m.commitFilesErr != nil {
		return "", m.commitFilesErr
	}
	m.filesCommitted = true
	m.commitRepo = repo
	m.commitBranch = branch
	m.commitMsg = message
	m.committedFiles = files
	if m.commitFilesSHA != "" {
		return m.commitFilesSHA, nil
	}
	return "sha-abcdef123456", nil
}

func (m *mockGitProvider) CreatePullRequest(ctx context.Context, repo, branch, baseBranch, title, body string) (string, int, error) {
	if m.createPRErr != nil {
		return "", 0, m.createPRErr
	}
	m.prCreated = true
	m.prRepo = repo
	m.prBranch = branch
	m.prBaseBranch = baseBranch
	m.prTitle = title
	m.prBody = body

	url := m.createPRURL
	if url == "" {
		url = "https://github.com/example/repo/pull/42"
	}
	num := m.createPRNumber
	if num == 0 {
		num = 42
	}
	return url, num, nil
}

func (m *mockGitProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-github"
}

type mockPRRepo struct {
	createErr error
	updateErr error
	getErr    error
	prs       map[string]*domainGitops.PullRequest
	createdPR *domainGitops.PullRequest
	updatedPR *domainGitops.PullRequest
}

func newMockPRRepo() *mockPRRepo {
	return &mockPRRepo{prs: make(map[string]*domainGitops.PullRequest)}
}

func (m *mockPRRepo) Create(ctx context.Context, pr *domainGitops.PullRequest) error {
	if m.createErr != nil {
		return m.createErr
	}
	if pr.ID == "" {
		pr.ID = "pr-mock-1234"
	}
	m.createdPR = pr
	m.prs[pr.ID] = pr
	return nil
}

func (m *mockPRRepo) GetByID(ctx context.Context, id string) (*domainGitops.PullRequest, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	pr, ok := m.prs[id]
	if !ok {
		return nil, errors.New("pr not found")
	}
	return pr, nil
}

func (m *mockPRRepo) GetByIncidentID(ctx context.Context, incidentID string) (*domainGitops.PullRequest, error) {
	for _, pr := range m.prs {
		if pr.IncidentID == incidentID {
			return pr, nil
		}
	}
	return nil, errors.New("pr not found")
}

func (m *mockPRRepo) Update(ctx context.Context, pr *domainGitops.PullRequest) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updatedPR = pr
	m.prs[pr.ID] = pr
	return nil
}

func (m *mockPRRepo) List(ctx context.Context, status *domainGitops.PRStatus, limit, offset int) ([]*domainGitops.PullRequest, int64, error) {
	var res []*domainGitops.PullRequest
	for _, pr := range m.prs {
		if status == nil || pr.Status == *status {
			res = append(res, pr)
		}
	}
	return res, int64(len(res)), nil
}

type mockIncidentRepo struct {
	createErr  error
	updateErr  error
	getErr     error
	incidents  map[string]*incident.Incident
	updatedInc *incident.Incident
	failUpdate bool
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	if m.createErr != nil {
		return m.createErr
	}
	if inc.ID == "" {
		inc.ID = "inc-mock-1234"
	}
	m.incidents[inc.ID] = inc
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	inc, ok := m.incidents[id]
	if !ok {
		return nil, errors.New("incident not found")
	}
	return inc, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	if m.failUpdate || m.updateErr != nil {
		if m.updateErr != nil {
			return m.updateErr
		}
		return errors.New("failed to update incident")
	}
	m.updatedInc = inc
	m.incidents[inc.ID] = inc
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	var res []*incident.Incident
	for _, inc := range m.incidents {
		res = append(res, inc)
	}
	return res, int64(len(res)), nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	for _, inc := range m.incidents {
		if inc.Namespace == namespace && inc.PodName == podName && inc.Type == incidentType {
			return inc, nil
		}
	}
	return nil, errors.New("incident not found")
}

func createTestIncidentAndReport(t *testing.T) (*incident.Incident, *report.Report) {
	inc, err := incident.New("system", "prod-namespace", "api-pod-12345", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "Back-off restarting failed container")
	if err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}
	inc.ID = "12345678-abcd-ef01-2345-6789abcdef01"
	_ = inc.MarkAnalyzing()

	rpt, err := report.New(
		inc.ID,
		"Application failed to connect to database at startup",
		[]string{"Exit Code 1", "Dial tcp 10.0.0.1:5432: i/o timeout"},
		0.95,
		report.RiskLow,
		"Update DATABASE_URL env var in deployment manifest",
		"Revert deployment manifest",
	)
	if err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	return inc, rpt
}

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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
