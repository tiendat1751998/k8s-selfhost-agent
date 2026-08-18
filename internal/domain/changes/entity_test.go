package changes_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/changes"
)

func TestNewChangeRequest_Success(t *testing.T) {
	req, err := changes.NewChangeRequest(
		"Scale up payments deployment",
		"Increase replicas to 5 for holiday traffic",
		changes.TypeStandard,
		"ops@company.com",
		"prod-us-east-1",
		"payments",
		"deployment/payments-service",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Title != "Scale up payments deployment" {
		t.Errorf("expected title 'Scale up payments deployment', got '%s'", req.Title)
	}
	if req.Description != "Increase replicas to 5 for holiday traffic" {
		t.Errorf("expected description 'Increase replicas to 5 for holiday traffic', got '%s'", req.Description)
	}
	if req.Type != changes.TypeStandard {
		t.Errorf("expected type 'standard', got '%s'", req.Type)
	}
	if req.Requester != "ops@company.com" {
		t.Errorf("expected requester 'ops@company.com', got '%s'", req.Requester)
	}
	if req.Cluster != "prod-us-east-1" {
		t.Errorf("expected cluster 'prod-us-east-1', got '%s'", req.Cluster)
	}
	if req.Namespace != "payments" {
		t.Errorf("expected namespace 'payments', got '%s'", req.Namespace)
	}
	if req.Resource != "deployment/payments-service" {
		t.Errorf("expected resource 'deployment/payments-service', got '%s'", req.Resource)
	}
	if req.Status != changes.StatusPending {
		t.Errorf("expected status 'pending', got '%s'", req.Status)
	}
	if req.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if req.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestNewChangeRequest_DefaultType(t *testing.T) {
	req, err := changes.NewChangeRequest(
		"Emergency hotfix",
		"Patching CVE-2026-0001",
		"",
		"security@company.com",
		"prod-cluster",
		"default",
		"pod/auth-proxy",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Type != changes.TypeStandard {
		t.Errorf("expected default type '%s', got '%s'", changes.TypeStandard, req.Type)
	}
}

func TestNewChangeRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		changeType  changes.ChangeType
		requester   string
		cluster     string
		namespace   string
		resource    string
	}{
		{
			name:        "empty title",
			title:       "",
			description: "desc",
			changeType:  changes.TypeStandard,
			requester:   "ops",
			cluster:     "prod",
			namespace:   "default",
			resource:    "deployment/api",
		},
		{
			name:        "empty requester",
			title:       "title",
			description: "desc",
			changeType:  changes.TypeStandard,
			requester:   "",
			cluster:     "prod",
			namespace:   "default",
			resource:    "deployment/api",
		},
		{
			name:        "empty cluster",
			title:       "title",
			description: "desc",
			changeType:  changes.TypeStandard,
			requester:   "ops",
			cluster:     "",
			namespace:   "default",
			resource:    "deployment/api",
		},
		{
			name:        "empty namespace",
			title:       "title",
			description: "desc",
			changeType:  changes.TypeStandard,
			requester:   "ops",
			cluster:     "prod",
			namespace:   "",
			resource:    "deployment/api",
		},
		{
			name:        "empty resource",
			title:       "title",
			description: "desc",
			changeType:  changes.TypeStandard,
			requester:   "ops",
			cluster:     "prod",
			namespace:   "default",
			resource:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := changes.NewChangeRequest(tt.title, tt.description, tt.changeType, tt.requester, tt.cluster, tt.namespace, tt.resource)
			if err == nil {
				t.Errorf("expected validation error for %s, got request: %+v", tt.name, req)
			}
		})
	}
}

func TestChangeRequest_Approve_Success(t *testing.T) {
	req, err := changes.NewChangeRequest(
		"Upgrade database",
		"Upgrade postgres to 16.2",
		changes.TypeStandard,
		"dba@company.com",
		"prod",
		"database",
		"statefulset/pg",
	)
	if err != nil {
		t.Fatalf("failed to create change request: %v", err)
	}

	if err := req.Approve("lead-architect"); err != nil {
		t.Fatalf("unexpected error on approve: %v", err)
	}

	if req.Status != changes.StatusApproved {
		t.Errorf("expected status 'approved', got '%s'", req.Status)
	}
	if req.Approver != "lead-architect" {
		t.Errorf("expected approver 'lead-architect', got '%s'", req.Approver)
	}
	if req.ApprovedAt == nil {
		t.Error("expected approved_at timestamp to be set")
	}
}

func TestChangeRequest_Approve_InvalidStatus(t *testing.T) {
	invalidStatuses := []changes.ChangeStatus{
		changes.StatusApproved,
		changes.StatusRejected,
		changes.StatusDeployed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+string(status), func(t *testing.T) {
			req := &changes.ChangeRequest{Status: status}
			err := req.Approve("lead")
			if err == nil {
				t.Errorf("expected error approving change request in status '%s'", status)
			}
		})
	}
}

func TestChangeRequest_Reject_Success(t *testing.T) {
	req, err := changes.NewChangeRequest(
		"Delete unused service",
		"Remove old v1 service",
		changes.TypeStandard,
		"dev@company.com",
		"staging",
		"legacy",
		"service/v1",
	)
	if err != nil {
		t.Fatalf("failed to create change request: %v", err)
	}

	if err := req.Reject("security-team"); err != nil {
		t.Fatalf("unexpected error on reject: %v", err)
	}

	if req.Status != changes.StatusRejected {
		t.Errorf("expected status 'rejected', got '%s'", req.Status)
	}
	if req.Approver != "security-team" {
		t.Errorf("expected approver 'security-team', got '%s'", req.Approver)
	}
}

func TestChangeRequest_Reject_InvalidStatus(t *testing.T) {
	invalidStatuses := []changes.ChangeStatus{
		changes.StatusApproved,
		changes.StatusRejected,
		changes.StatusDeployed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+string(status), func(t *testing.T) {
			req := &changes.ChangeRequest{Status: status}
			err := req.Reject("security-team")
			if err == nil {
				t.Errorf("expected error rejecting change request in status '%s'", status)
			}
		})
	}
}

func TestChangeRequest_MarkDeployed_Success(t *testing.T) {
	req, _ := changes.NewChangeRequest(
		"Deploy new container image",
		"Update image to tag v2.1.0",
		changes.TypeEmergency,
		"sre@company.com",
		"prod",
		"apps",
		"deployment/web",
	)
	_ = req.Approve("sre-lead")

	if err := req.MarkDeployed(); err != nil {
		t.Fatalf("unexpected error marking deployed: %v", err)
	}

	if req.Status != changes.StatusDeployed {
		t.Errorf("expected status 'deployed', got '%s'", req.Status)
	}
}

func TestChangeRequest_MarkDeployed_InvalidStatus(t *testing.T) {
	invalidStatuses := []changes.ChangeStatus{
		changes.StatusPending,
		changes.StatusRejected,
		changes.StatusDeployed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+string(status), func(t *testing.T) {
			req := &changes.ChangeRequest{Status: status}
			err := req.MarkDeployed()
			if err == nil {
				t.Errorf("expected error marking change request deployed from status '%s'", status)
			}
		})
	}
}

func TestChangeRequest_BooleanChecks(t *testing.T) {
	tests := []struct {
		status    changes.ChangeStatus
		canApprove bool
		canReject  bool
		canDeploy  bool
	}{
		{changes.StatusPending, true, true, false},
		{changes.StatusApproved, false, false, true},
		{changes.StatusRejected, false, false, false},
		{changes.StatusDeployed, false, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			req := &changes.ChangeRequest{Status: tt.status}
			if req.CanApprove() != tt.canApprove {
				t.Errorf("CanApprove() for status %s expected %v, got %v", tt.status, tt.canApprove, req.CanApprove())
			}
			if req.CanReject() != tt.canReject {
				t.Errorf("CanReject() for status %s expected %v, got %v", tt.status, tt.canReject, req.CanReject())
			}
			if req.CanDeploy() != tt.canDeploy {
				t.Errorf("CanDeploy() for status %s expected %v, got %v", tt.status, tt.canDeploy, req.CanDeploy())
			}
		})
	}
}
