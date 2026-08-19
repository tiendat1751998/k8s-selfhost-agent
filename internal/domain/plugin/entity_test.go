package plugin_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/plugin"
)

func TestPlugin_Validate(t *testing.T) {
	tests := []struct {
		name    string
		p       plugin.Plugin
		wantErr bool
	}{
		{
			name: "valid plugin",
			p: plugin.Plugin{
				Name:     "prometheus-lens",
				Category: plugin.CategoryMonitoring,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			p: plugin.Plugin{
				Name:     "",
				Category: plugin.CategoryDevtools,
			},
			wantErr: true,
		},
		{
			name: "whitespace name",
			p: plugin.Plugin{
				Name:     "   ",
				Category: plugin.CategoryDevtools,
			},
			wantErr: true,
		},
		{
			name: "valid with empty category (defaults later)",
			p: plugin.Plugin{
				Name: "security-scanner",
			},
			wantErr: false,
		},
		{
			name: "invalid category",
			p: plugin.Plugin{
				Name:     "bad-cat",
				Category: "nonexistent",
			},
			wantErr: true,
		},
		{
			name: "all valid categories",
			p: plugin.Plugin{
				Name:     "integration-tool",
				Category: plugin.CategoryIntegration,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
