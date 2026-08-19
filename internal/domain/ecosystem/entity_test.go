package ecosystem_test

import (
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
)

func TestDetectedTool_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tool    ecosystem.DetectedTool
		wantErr bool
	}{
		{
			name: "valid tool",
			tool: ecosystem.DetectedTool{
				Name:        "ArgoCD",
				Category:    ecosystem.CategoryGitOps,
				Status:      ecosystem.StatusDetected,
				Version:     "v2.10.0",
				Endpoint:    "https://argocd.example.com",
				Source:      ecosystem.SourceSettings,
				Health:      ecosystem.HealthHealthy,
				LastChecked: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			tool: ecosystem.DetectedTool{
				Category: ecosystem.CategoryGitOps,
			},
			wantErr: true,
		},
		{
			name: "missing category",
			tool: ecosystem.DetectedTool{
				Name: "ArgoCD",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
