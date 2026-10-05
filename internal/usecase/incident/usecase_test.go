package incident_test

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	incidentUsecase "github.com/datdt/k8sselfhost/internal/usecase/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type testIncidentRepo struct {
	incidents map[string]*incident.Incident
	seq       int
}

func newTestIncidentRepo() *testIncidentRepo {
	return &testIncidentRepo{
		incidents: make(map[string]*incident.Incident),
	}
}

func (r *testIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	r.seq++
	if inc.ID == "" {
		inc.ID = "inc-" + string(rune('0'+r.seq))
	}
	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	inc.TenantID = tenantID
	r.incidents[inc.ID] = inc
	return nil
}

func (r *testIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	inc, ok := r.incidents[id]
	if !ok {
		return nil, errors.NewNotFound("incident", id)
	}
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" {
		callerTenant := tenancy.TenantIDFromContext(ctx)
		if callerTenant != "" && inc.TenantID != "" && inc.TenantID != callerTenant {
			return nil, errors.NewNotFound("incident", id)
		}
	}
	return inc, nil
}

func (r *testIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	existing, err := r.GetByID(ctx, inc.ID)
	if err != nil {
		return err
	}
	r.incidents[existing.ID] = inc
	return nil
}

func (r *testIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	var list []*incident.Incident
	for _, inc := range r.incidents {
		list = append(list, inc)
	}
	return list, int64(len(list)), nil
}

func (r *testIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	for _, inc := range r.incidents {
		if inc.Namespace == namespace && inc.PodName == podName && inc.Type == incidentType && inc.Status != incident.StatusResolved {
			return inc, nil
		}
	}
	return nil, nil
}

func TestUsecase_CreateIncident_Success(t *testing.T) {
	repo := newTestIncidentRepo()
	uc := incidentUsecase.NewUsecase(repo)

	ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")
	cmd := incidentUsecase.CreateCommand{
		Title:       "High CPU on payment service",
		Severity:    "high",
		Description: "CPU consumption spiked above 95% threshold",
		ClusterID:   "cluster-us-east",
		ServiceName: "payment-api",
	}

	inc, err := uc.CreateIncident(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error creating incident: %v", err)
	}

	if inc.ID == "" {
		t.Error("expected non-empty incident ID")
	}
	if inc.Title != "High CPU on payment service" {
		t.Errorf("expected title 'High CPU on payment service', got '%s'", inc.Title)
	}
	if inc.Severity != incident.SeverityHigh {
		t.Errorf("expected severity 'high', got '%s'", inc.Severity)
	}
	if inc.ClusterID != "cluster-us-east" {
		t.Errorf("expected cluster_id 'cluster-us-east', got '%s'", inc.ClusterID)
	}
	if inc.ServiceName != "payment-api" {
		t.Errorf("expected service_name 'payment-api', got '%s'", inc.ServiceName)
	}
	if inc.Status != incident.StatusDetected {
		t.Errorf("expected status 'detected', got '%s'", inc.Status)
	}
	if inc.TenantID != "tenant-alpha" {
		t.Errorf("expected tenant_id 'tenant-alpha', got '%s'", inc.TenantID)
	}
}

func TestUsecase_CreateIncident_ValidationErrors(t *testing.T) {
	repo := newTestIncidentRepo()
	uc := incidentUsecase.NewUsecase(repo)
	ctx := context.Background()

	cases := []struct {
		name string
		cmd  incidentUsecase.CreateCommand
	}{
		{
			name: "missing title",
			cmd: incidentUsecase.CreateCommand{
				Severity:    "high",
				Description: "desc",
				ClusterID:   "c1",
				ServiceName: "s1",
			},
		},
		{
			name: "invalid severity",
			cmd: incidentUsecase.CreateCommand{
				Title:       "title",
				Severity:    "invalid-sev",
				Description: "desc",
				ClusterID:   "c1",
				ServiceName: "s1",
			},
		},
		{
			name: "missing description",
			cmd: incidentUsecase.CreateCommand{
				Title:       "title",
				Severity:    "high",
				Description: "",
				ClusterID:   "c1",
				ServiceName: "s1",
			},
		},
		{
			name: "missing cluster_id",
			cmd: incidentUsecase.CreateCommand{
				Title:       "title",
				Severity:    "high",
				Description: "desc",
				ClusterID:   "",
				ServiceName: "s1",
			},
		},
		{
			name: "missing service_name",
			cmd: incidentUsecase.CreateCommand{
				Title:       "title",
				Severity:    "high",
				Description: "desc",
				ClusterID:   "c1",
				ServiceName: "",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.CreateIncident(ctx, tc.cmd)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			if !errors.Is(err, errors.ErrValidation) {
				t.Errorf("expected validation error type, got: %v", err)
			}
		})
	}
}

func TestUsecase_MitigateIncident_Success(t *testing.T) {
	repo := newTestIncidentRepo()
	uc := incidentUsecase.NewUsecase(repo)
	ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")

	created, err := uc.CreateIncident(ctx, incidentUsecase.CreateCommand{
		Title:       "Out of memory crash",
		Severity:    "critical",
		Description: "OOM killer terminated container",
		ClusterID:   "prod-cluster",
		ServiceName: "worker-svc",
	})
	if err != nil {
		t.Fatalf("failed to setup incident: %v", err)
	}

	mitigated, err := uc.MitigateIncident(ctx, created.ID, incidentUsecase.MitigateCommand{
		Operator: "sre-lead",
		Notes:    "Allocated 2GB additional RAM and restarted replica",
		Status:   incident.StatusMitigating,
	})
	if err != nil {
		t.Fatalf("unexpected error mitigating incident: %v", err)
	}

	if mitigated.Status != incident.StatusMitigating {
		t.Errorf("expected status 'mitigating', got '%s'", mitigated.Status)
	}
	if mitigated.RawData["mitigated_by"] != "sre-lead" {
		t.Errorf("expected mitigated_by 'sre-lead', got '%s'", mitigated.RawData["mitigated_by"])
	}
	if mitigated.RawData["mitigation_notes"] != "Allocated 2GB additional RAM and restarted replica" {
		t.Errorf("expected mitigation_notes, got '%s'", mitigated.RawData["mitigation_notes"])
	}
}

func TestUsecase_ResolveIncident_Success(t *testing.T) {
	repo := newTestIncidentRepo()
	uc := incidentUsecase.NewUsecase(repo)
	ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")

	created, err := uc.CreateIncident(ctx, incidentUsecase.CreateCommand{
		Title:       "Database connection spike",
		Severity:    "medium",
		Description: "Conn pool max reached",
		ClusterID:   "prod-cluster",
		ServiceName: "db-proxy",
	})
	if err != nil {
		t.Fatalf("failed to setup incident: %v", err)
	}

	resolved, err := uc.ResolveIncident(ctx, created.ID, incidentUsecase.ResolveCommand{
		Operator: "sre-bob",
		Notes:    "Scaled pool and verified metrics stabilized",
	})
	if err != nil {
		t.Fatalf("unexpected error resolving incident: %v", err)
	}

	if resolved.Status != incident.StatusResolved {
		t.Errorf("expected status 'resolved', got '%s'", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Error("expected non-nil ResolvedAt timestamp")
	}
	if resolved.RawData["resolved_by"] != "sre-bob" {
		t.Errorf("expected resolved_by 'sre-bob', got '%s'", resolved.RawData["resolved_by"])
	}
}

func TestUsecase_TenantIsolation(t *testing.T) {
	repo := newTestIncidentRepo()
	uc := incidentUsecase.NewUsecase(repo)

	ctxTenantA := tenancy.WithTenantID(context.Background(), "tenant-alpha")
	created, err := uc.CreateIncident(ctxTenantA, incidentUsecase.CreateCommand{
		Title:       "Secret incident",
		Severity:    "high",
		Description: "Confidential outage",
		ClusterID:   "c-a",
		ServiceName: "s-a",
	})
	if err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	ctxTenantB := tenancy.WithTenantID(context.Background(), "tenant-bravo")
	_, err = uc.GetIncident(ctxTenantB, created.ID)
	if err == nil {
		t.Error("expected error when tenant-bravo accesses tenant-alpha incident")
	}
	if !errors.Is(err, errors.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant access, got: %v", err)
	}

	_, err = uc.MitigateIncident(ctxTenantB, created.ID, incidentUsecase.MitigateCommand{
		Operator: "attacker",
		Notes:    "unauthorized mitigate",
	})
	if err == nil {
		t.Error("expected error when tenant-bravo mitigates tenant-alpha incident")
	}
	if !errors.Is(err, errors.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant mitigate, got: %v", err)
	}
}
