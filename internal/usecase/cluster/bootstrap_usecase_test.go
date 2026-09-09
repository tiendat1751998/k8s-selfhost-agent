package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	clusterDomain "github.com/datdt/k8sselfhost/internal/domain/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

type testMetricsFetcher struct {
	data []byte
	err  error
}

func (m *testMetricsFetcher) FetchPodMetrics(ctx context.Context, clusterID string) ([]byte, error) {
	return m.data, m.err
}

func TestBootstrapUsecase_GetEssentialsStatus_NotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := NewBootstrapUsecase(repo, nil, zap.NewNop())

	status, err := uc.GetEssentialsStatus(context.Background(), "local")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "local", status.ClusterID)
	assert.False(t, status.Ready)
	assert.Len(t, status.Components, 3)

	for _, c := range status.Components {
		assert.False(t, c.Installed)
		assert.Equal(t, clusterDomain.ComponentStatusNotFound, c.Status)
	}
	assert.Empty(t, status.TaintsInfo)
}

func TestBootstrapUsecase_GetEssentialsStatus_Installed(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-server",
			Namespace: "kube-system",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "metrics-server", Image: "registry.k8s.io/metrics-server/metrics-server:v0.7.2"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local-path",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner: "rancher.io/local-path",
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-control-agent",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "agent", Image: "ghcr.io/datdt/k8s-control-agent:v1.0.0"},
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 1,
		},
	}

	nodeMaster := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master-1",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(dep, sc, ds, nodeMaster)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := NewBootstrapUsecase(repo, nil, zap.NewNop())

	status, err := uc.GetEssentialsStatus(context.Background(), "local")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "local", status.ClusterID)
	// Ready should be false because master node is tainted
	assert.False(t, status.Ready)

	// Verify metrics-server
	var metricsComp *clusterDomain.EssentialComponent
	for i := range status.Components {
		if status.Components[i].Name == clusterDomain.ComponentNameMetricsServer {
			metricsComp = &status.Components[i]
		}
	}
	require.NotNil(t, metricsComp)
	assert.True(t, metricsComp.Installed)
	assert.Equal(t, clusterDomain.ComponentStatusRunning, metricsComp.Status)
	assert.Equal(t, "v0.7.2", metricsComp.Version)

	// Verify StorageClass
	var scComp *clusterDomain.EssentialComponent
	for i := range status.Components {
		if status.Components[i].Name == clusterDomain.ComponentNameLocalStorage {
			scComp = &status.Components[i]
		}
	}
	require.NotNil(t, scComp)
	assert.True(t, scComp.Installed)
	assert.Equal(t, clusterDomain.ComponentStatusRunning, scComp.Status)
	assert.Equal(t, "rancher.io/local-path", scComp.Version)

	// Verify DaemonSet
	var dsComp *clusterDomain.EssentialComponent
	for i := range status.Components {
		if status.Components[i].Name == clusterDomain.ComponentNameControlAgent {
			dsComp = &status.Components[i]
		}
	}
	require.NotNil(t, dsComp)
	assert.True(t, dsComp.Installed)
	assert.Equal(t, clusterDomain.ComponentStatusRunning, dsComp.Status)
	assert.Equal(t, "v1.0.0", dsComp.Version)

	// Verify taints info
	require.Len(t, status.TaintsInfo, 1)
	assert.Equal(t, "master-1", status.TaintsInfo[0].NodeName)
	assert.True(t, status.TaintsInfo[0].Tainted)
}

func TestBootstrapUsecase_ExecuteBootstrap_All(t *testing.T) {
	nodeMaster := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-master",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(nodeMaster)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := NewBootstrapUsecase(repo, nil, zap.NewNop())

	req := clusterDomain.BootstrapRequest{
		InstallMetricsServer: true,
		InstallStorageClass:  true,
		DeployAgentDaemonSet: true,
		UntaintMasters:       true,
	}

	res, err := uc.ExecuteBootstrap(context.Background(), "local", req)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.True(t, res.Success)
	assert.Empty(t, res.Errors)
	assert.Contains(t, res.Installed, "metrics-server")
	assert.Contains(t, res.Installed, "local-path-storageclass")
	assert.Contains(t, res.Installed, "k8s-control-agent")
	assert.Contains(t, res.Installed, "untaint-node-master")
	assert.True(t, res.DurationMs >= 0)

	// Verify node untainted in fakeClient
	node, err := fakeClient.CoreV1().Nodes().Get(context.Background(), "node-master", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, node.Spec.Taints)
}

func TestBootstrapUsecase_GetPodMetrics(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := NewBootstrapUsecase(repo, nil, zap.NewNop())

	t.Run("returns empty list by default when metrics unavailable", func(t *testing.T) {
		data, err := uc.GetPodMetrics(context.Background(), "local")
		require.NoError(t, err)
		assert.Contains(t, string(data), "PodMetricsList")
		assert.Contains(t, string(data), "items")
	})

	t.Run("returns data when fetcher provides metrics", func(t *testing.T) {
		mockData := []byte(`{"kind":"PodMetricsList","items":[{"metadata":{"name":"pod-1"}}]}`)
		uc.SetMetricsFetcher(&testMetricsFetcher{data: mockData})

		data, err := uc.GetPodMetrics(context.Background(), "local")
		require.NoError(t, err)
		assert.Equal(t, mockData, data)
	})

	t.Run("returns empty list gracefully when fetcher fails", func(t *testing.T) {
		uc.SetMetricsFetcher(&testMetricsFetcher{err: errors.New("connection refused")})

		data, err := uc.GetPodMetrics(context.Background(), "local")
		require.NoError(t, err)
		assert.Contains(t, string(data), "PodMetricsList")
	})
}
