package cluster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/domain/cluster"
)

func TestBootstrapRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     cluster.BootstrapRequest
		wantErr bool
	}{
		{
			name:    "empty request fails",
			req:     cluster.BootstrapRequest{},
			wantErr: true,
		},
		{
			name: "install metrics server only",
			req: cluster.BootstrapRequest{
				InstallMetricsServer: true,
			},
			wantErr: false,
		},
		{
			name: "install storage class only",
			req: cluster.BootstrapRequest{
				InstallStorageClass: true,
			},
			wantErr: false,
		},
		{
			name: "untaint masters only",
			req: cluster.BootstrapRequest{
				UntaintMasters: true,
			},
			wantErr: false,
		},
		{
			name: "deploy agent daemonset only",
			req: cluster.BootstrapRequest{
				DeployAgentDaemonSet: true,
			},
			wantErr: false,
		},
		{
			name: "all actions selected",
			req: cluster.BootstrapRequest{
				InstallMetricsServer: true,
				InstallStorageClass:  true,
				UntaintMasters:       true,
				DeployAgentDaemonSet: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewClusterEssentialsStatus(t *testing.T) {
	t.Run("empty components not ready", func(t *testing.T) {
		status := cluster.NewClusterEssentialsStatus("c1", nil, nil)
		assert.Equal(t, "c1", status.ClusterID)
		assert.False(t, status.Ready)
	})

	t.Run("all components running and not tainted is ready", func(t *testing.T) {
		comps := []cluster.EssentialComponent{
			{
				Name:      cluster.ComponentNameMetricsServer,
				Installed: true,
				Version:   "v0.7.2",
				Status:    cluster.ComponentStatusRunning,
				Namespace: "kube-system",
			},
			{
				Name:      cluster.ComponentNameLocalStorage,
				Installed: true,
				Version:   "v0.0.28",
				Status:    cluster.ComponentStatusRunning,
				Namespace: "kube-system",
			},
		}
		taints := []cluster.NodeTaintInfo{
			{NodeName: "node-1", Tainted: false},
		}

		status := cluster.NewClusterEssentialsStatus("c1", comps, taints)
		assert.True(t, status.Ready)
		assert.Len(t, status.Components, 2)
		assert.Len(t, status.TaintsInfo, 1)
	})

	t.Run("pending component makes ready false", func(t *testing.T) {
		comps := []cluster.EssentialComponent{
			{
				Name:      cluster.ComponentNameMetricsServer,
				Installed: true,
				Status:    cluster.ComponentStatusPending,
			},
		}
		status := cluster.NewClusterEssentialsStatus("c1", comps, nil)
		assert.False(t, status.Ready)
	})

	t.Run("tainted node makes ready false", func(t *testing.T) {
		comps := []cluster.EssentialComponent{
			{
				Name:      cluster.ComponentNameMetricsServer,
				Installed: true,
				Status:    cluster.ComponentStatusRunning,
			},
		}
		taints := []cluster.NodeTaintInfo{
			{NodeName: "master-1", Tainted: true, Taints: "node-role.kubernetes.io/control-plane:NoSchedule"},
		}
		status := cluster.NewClusterEssentialsStatus("c1", comps, taints)
		assert.False(t, status.Ready)
	})
}

func TestNewBootstrapResult(t *testing.T) {
	res := cluster.NewBootstrapResult(true, []string{"metrics-server"}, nil, 120)
	assert.True(t, res.Success)
	assert.Equal(t, []string{"metrics-server"}, res.Installed)
	assert.Empty(t, res.Errors)
	assert.Equal(t, int64(120), res.DurationMs)

	resNil := cluster.NewBootstrapResult(false, nil, nil, 0)
	assert.False(t, resNil.Success)
	assert.NotNil(t, resNil.Installed)
	assert.NotNil(t, resNil.Errors)
}
