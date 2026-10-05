package incident

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// CreateCommand defines inputs required to create an incident.
type CreateCommand struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Description string            `json:"description"`
	ClusterID   string            `json:"cluster_id"`
	ServiceName string            `json:"service_name"`
	ClusterName string            `json:"cluster_name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	PodName     string            `json:"pod_name,omitempty"`
	Type        string            `json:"type,omitempty"`
	TenantID    string            `json:"tenant_id,omitempty"`
	RawData     map[string]string `json:"raw_data,omitempty"`
}

// MitigateCommand defines inputs to transition an incident to mitigating/mitigated.
type MitigateCommand struct {
	Operator string          `json:"operator"`
	Notes    string          `json:"notes"`
	Status   incident.Status `json:"status"`
}

// ResolveCommand defines inputs to transition an incident to resolved.
type ResolveCommand struct {
	Operator string `json:"operator"`
	Notes    string `json:"notes"`
}

// Usecase coordinates business operations for incidents.
type Usecase struct {
	repo incident.Repository
}

// NewUsecase constructs a new incident usecase.
func NewUsecase(repo incident.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// CreateIncident validates input invariants and persists a new incident.
func (u *Usecase) CreateIncident(ctx context.Context, cmd CreateCommand) (*incident.Incident, error) {
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return nil, errors.NewValidation("title", "must not be empty")
	}

	sevStr := strings.ToLower(strings.TrimSpace(cmd.Severity))
	severity := incident.Severity(sevStr)
	if !severity.IsValid() {
		return nil, errors.NewValidation("severity", fmt.Sprintf("invalid severity: %s", cmd.Severity))
	}

	description := strings.TrimSpace(cmd.Description)
	if description == "" {
		return nil, errors.NewValidation("description", "must not be empty")
	}

	clusterID := strings.TrimSpace(cmd.ClusterID)
	if clusterID == "" {
		clusterID = strings.TrimSpace(cmd.ClusterName)
	}
	if clusterID == "" {
		return nil, errors.NewValidation("cluster_id", "must not be empty")
	}

	serviceName := strings.TrimSpace(cmd.ServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(cmd.PodName)
	}
	if serviceName == "" {
		return nil, errors.NewValidation("service_name", "must not be empty")
	}

	namespace := strings.TrimSpace(cmd.Namespace)
	if namespace == "" {
		namespace = "default"
	}

	var incType incident.Type
	if strings.TrimSpace(cmd.Type) != "" {
		incType = incident.Type(strings.TrimSpace(cmd.Type))
		if !incType.IsValid() {
			return nil, errors.NewValidation("type", fmt.Sprintf("invalid incident type: %s", cmd.Type))
		}
	} else {
		incType = incident.TypeServiceUnhealthy
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		if tenancy.UserRoleFromContext(ctx) == "platform_admin" && cmd.TenantID != "" {
			tenantID = strings.TrimSpace(cmd.TenantID)
		} else {
			tenantID = "default-tenant"
		}
	}

	now := time.Now().UTC()
	rawData := make(map[string]string)
	for k, v := range cmd.RawData {
		rawData[k] = v
	}
	rawData["title"] = title
	rawData["description"] = description
	rawData["cluster_id"] = clusterID
	rawData["service_name"] = serviceName
	rawData["tenant_id"] = tenantID

	inc := &incident.Incident{
		Title:       title,
		Description: description,
		ClusterID:   clusterID,
		ServiceName: serviceName,
		TenantID:    tenantID,
		ClusterName: clusterID,
		Namespace:   namespace,
		PodName:     serviceName,
		Type:        incType,
		Status:      incident.StatusDetected,
		Severity:    severity,
		Message:     title + ": " + description,
		RawData:     rawData,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := u.repo.Create(ctx, inc); err != nil {
		return nil, err
	}

	return inc, nil
}

// MitigateIncident transitions an incident to mitigating or mitigated state with operator notes.
func (u *Usecase) MitigateIncident(ctx context.Context, id string, cmd MitigateCommand) (*incident.Incident, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.NewValidation("id", "must not be empty")
	}

	inc, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	targetStatus := cmd.Status
	if targetStatus != incident.StatusMitigated && targetStatus != incident.StatusMitigating {
		targetStatus = incident.StatusMitigating
	}

	if err := inc.Mitigate(cmd.Operator, cmd.Notes, targetStatus); err != nil {
		return nil, err
	}

	if err := u.repo.Update(ctx, inc); err != nil {
		return nil, err
	}

	return inc, nil
}

// ResolveIncident transitions an incident to resolved state and sets resolved_at.
func (u *Usecase) ResolveIncident(ctx context.Context, id string, cmd ResolveCommand) (*incident.Incident, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.NewValidation("id", "must not be empty")
	}

	inc, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := inc.Resolve(cmd.Operator, cmd.Notes); err != nil {
		return nil, err
	}

	if err := u.repo.Update(ctx, inc); err != nil {
		return nil, err
	}

	return inc, nil
}

// GetIncident retrieves an incident by ID subject to tenant isolation.
func (u *Usecase) GetIncident(ctx context.Context, id string) (*incident.Incident, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.NewValidation("id", "must not be empty")
	}
	return u.repo.GetByID(ctx, id)
}

// ListIncidents retrieves incidents matching filter criteria.
func (u *Usecase) ListIncidents(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	return u.repo.List(ctx, filter)
}
