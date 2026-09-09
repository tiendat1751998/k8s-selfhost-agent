package ecosystem

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
)

// GetSummary returns aggregated health and category counts for the tenant's ecosystem tools.
func (u *detectorUsecase) GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	tools, err := u.GetAll(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	summary := &ecosystem.EcosystemSummary{
		Total:      len(tools),
		Healthy:    0,
		Degraded:   0,
		ByCategory: make(map[string]int),
	}

	for _, t := range tools {
		summary.ByCategory[t.Category]++
		if t.Health == ecosystem.HealthHealthy {
			summary.Healthy++
		} else if t.Health == ecosystem.HealthDegraded || t.Status == ecosystem.StatusUnreachable {
			summary.Degraded++
		}
	}

	return summary, nil
}

