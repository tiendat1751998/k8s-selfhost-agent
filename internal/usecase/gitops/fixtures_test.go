package gitops

import (
	"context"
	"errors"
	"testing"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
)

type mockTxManager struct {
	txCount int
}

func (m *mockTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
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

	mergeErr    error
	mergeCalled bool
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

func (m *mockGitProvider) MergePullRequest(ctx context.Context, repo string, prNumber int) error {
	m.mergeCalled = true
	return m.mergeErr
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
