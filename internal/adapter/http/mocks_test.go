package http

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockIncidentRepo struct {
	mu        sync.Mutex
	incidents map[string]*incident.Incident
	created   []*incident.Incident
	updated   []*incident.Incident
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{
		incidents: make(map[string]*incident.Incident),
	}
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc.ID == "" {
		inc.ID = "inc-sim-123"
	}
	m.incidents[inc.ID] = inc
	m.created = append(m.created, inc)
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inc, ok := m.incidents[id]
	if !ok {
		return nil, errors.NewNotFound("incident", id)
	}
	return inc, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidents[inc.ID] = inc
	m.updated = append(m.updated, inc)
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*incident.Incident
	for _, inc := range m.incidents {
		res = append(res, inc)
	}
	return res, int64(len(res)), nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, inc := range m.incidents {
		if inc.Namespace == namespace && inc.PodName == podName && inc.Type == incidentType {
			return inc, nil
		}
	}
	return nil, nil
}

type mockReportRepo struct {
	mu      sync.Mutex
	reports map[string]*report.Report
	created []*report.Report
}

func newMockReportRepo() *mockReportRepo {
	return &mockReportRepo{
		reports: make(map[string]*report.Report),
	}
}

func (m *mockReportRepo) Create(ctx context.Context, rpt *report.Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rpt.ID == "" {
		rpt.ID = fmt.Sprintf("rpt-%d", time.Now().UnixNano())
	}
	m.reports[rpt.IncidentID] = rpt
	m.created = append(m.created, rpt)
	return nil
}

func (m *mockReportRepo) GetByID(ctx context.Context, id string) (*report.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, errors.NewNotFound("report", id)
}

func (m *mockReportRepo) GetByIncidentID(ctx context.Context, incidentID string) (*report.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[incidentID]
	if !ok {
		return nil, errors.NewNotFound("report", incidentID)
	}
	return r, nil
}

func (m *mockReportRepo) List(ctx context.Context, limit, offset int) ([]*report.Report, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*report.Report
	for _, r := range m.reports {
		res = append(res, r)
	}
	return res, int64(len(res)), nil
}

type mockPRRepo struct {
	mu      sync.Mutex
	prs     map[string]*domainGitops.PullRequest
	created []*domainGitops.PullRequest
}

func newMockPRRepo() *mockPRRepo {
	return &mockPRRepo{
		prs: make(map[string]*domainGitops.PullRequest),
	}
}

func (m *mockPRRepo) Create(ctx context.Context, pr *domainGitops.PullRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pr.ID == "" {
		pr.ID = "pr-sim-123"
	}
	m.prs[pr.ID] = pr
	m.created = append(m.created, pr)
	return nil
}

func (m *mockPRRepo) GetByID(ctx context.Context, id string) (*domainGitops.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[id]
	if !ok {
		return nil, errors.NewNotFound("pr", id)
	}
	return pr, nil
}

func (m *mockPRRepo) GetByIncidentID(ctx context.Context, incidentID string) (*domainGitops.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pr := range m.prs {
		if pr.IncidentID == incidentID {
			return pr, nil
		}
	}
	return nil, errors.NewNotFound("pr", incidentID)
}

func (m *mockPRRepo) Update(ctx context.Context, pr *domainGitops.PullRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prs[pr.ID] = pr
	return nil
}

func (m *mockPRRepo) List(ctx context.Context, status *domainGitops.PRStatus, limit, offset int) ([]*domainGitops.PullRequest, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []*domainGitops.PullRequest
	for _, pr := range m.prs {
		if status == nil || pr.Status == *status {
			res = append(res, pr)
		}
	}
	return res, int64(len(res)), nil
}
