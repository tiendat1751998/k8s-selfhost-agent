package cluster

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockFleetHealthRepo struct {
	mu            sync.Mutex
	clusters      []fleet.Cluster
	healthCalls   map[string]string // clusterID -> healthStatus
	statusCalls   map[string]string // clusterID -> status
	nodeCalls     map[string]int    // clusterID -> nodes
	lastCheckMap  map[string]time.Time
	lastListCtx   context.Context
	lastHealthCtx context.Context
}

func newMockFleetHealthRepo(clusters []fleet.Cluster) *mockFleetHealthRepo {
	return &mockFleetHealthRepo{
		clusters:     clusters,
		healthCalls:  make(map[string]string),
		statusCalls:  make(map[string]string),
		nodeCalls:    make(map[string]int),
		lastCheckMap: make(map[string]time.Time),
	}
}

func (m *mockFleetHealthRepo) ListClusters(ctx context.Context) ([]fleet.Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastListCtx = ctx
	return m.clusters, nil
}

func (m *mockFleetHealthRepo) GetCluster(ctx context.Context, id string) (*fleet.Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clusters {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, errors.New("cluster not found")
}

func (m *mockFleetHealthRepo) RegisterCluster(ctx context.Context, c *fleet.Cluster) error {
	return nil
}

func (m *mockFleetHealthRepo) UpdateClusterStatus(ctx context.Context, id, status, version string, nodes int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCalls[id] = status
	m.nodeCalls[id] = nodes
	return nil
}

func (m *mockFleetHealthRepo) UpdateHealth(ctx context.Context, id, healthStatus string, lastCheck time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCalls[id] = healthStatus
	m.lastCheckMap[id] = lastCheck
	m.lastHealthCtx = ctx
	return nil
}

func (m *mockFleetHealthRepo) UpdateDiscoveredResources(ctx context.Context, id string, resources map[string]interface{}) error {
	return nil
}

func (m *mockFleetHealthRepo) RemoveCluster(ctx context.Context, id string) error {
	return nil
}

type mockDiscoveryPort struct {
	validateErr error
}

func (m *mockDiscoveryPort) DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error) {
	return map[string]interface{}{"nodes_count": 3}, nil
}

func (m *mockDiscoveryPort) ValidateConnectivity(ctx context.Context, kubeconfig []byte) error {
	return m.validateErr
}

type mockComputeHostRepoForHealth struct {
	hosts []docker.ComputeHost
}

func (m *mockComputeHostRepoForHealth) Create(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepoForHealth) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	return nil, nil
}

func (m *mockComputeHostRepoForHealth) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepoForHealth) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepoForHealth) Update(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepoForHealth) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockComputeHostRepoForHealth) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

// mockHTTPTransport intercepts HTTP requests to simulate node agent /health endpoints.
type mockHTTPTransport struct {
	healthyEndpoints map[string]bool
}

func (m *mockHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()
	for prefix, healthy := range m.healthyEndpoints {
		if strings.HasPrefix(urlStr, prefix) {
			if healthy {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(`{"status":"error"}`)),
				Header:     make(http.Header),
			}, nil
		}
	}
	return nil, errors.New("connection refused")
}

func TestHealthChecker_NonKubeconfig_Degraded(t *testing.T) {
	// Scenario: k8snode cluster has 3 node agents online, but K8s API (:6443) is unreachable.
	// Expected: health_status = "degraded", status = "degraded", nodes = 3
	c := fleet.Cluster{
		ID:           "cluster-k8snode-1",
		Name:         "k8snode",
		ImportMethod: "agent",
		Status:       "connected",
		HealthStatus: "healthy", // Initially fake healthy
		Nodes:        3,
		APIServerURL: "https://10.10.10.133:6443",
	}

	fleetRepo := newMockFleetHealthRepo([]fleet.Cluster{c})
	discovery := &mockDiscoveryPort{}

	hostRepo := &mockComputeHostRepoForHealth{
		hosts: []docker.ComputeHost{
			{ID: "host-1", Name: "node-1", Endpoint: "http://10.10.10.133:9100", Labels: map[string]string{"cluster": "k8snode"}},
			{ID: "host-2", Name: "node-2", Endpoint: "http://10.10.10.134:9100", Labels: map[string]string{"cluster": "k8snode"}},
			{ID: "host-3", Name: "node-3", Endpoint: "http://10.10.10.201:9100", Labels: map[string]string{"cluster": "k8snode"}},
		},
	}

	// Dial to :6443 always fails (API server unreachable)
	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("dial tcp: connection refused")
	}

	// Node agents :9100 are all healthy
	httpClient := &http.Client{
		Transport: &mockHTTPTransport{
			healthyEndpoints: map[string]bool{
				"http://10.10.10.133:9100": true,
				"http://10.10.10.134:9100": true,
				"http://10.10.10.201:9100": true,
			},
		},
	}

	checker := NewHealthChecker(fleetRepo, discovery, zap.NewNop(),
		WithComputeHostRepo(hostRepo),
		WithDialContext(dialFn),
		WithHTTPClient(httpClient),
	)

	checker.checkCluster(context.Background(), c)

	fleetRepo.mu.Lock()
	defer fleetRepo.mu.Unlock()

	assert.Equal(t, "degraded", fleetRepo.healthCalls["cluster-k8snode-1"])
	assert.Equal(t, "degraded", fleetRepo.statusCalls["cluster-k8snode-1"])
	assert.Equal(t, 3, fleetRepo.nodeCalls["cluster-k8snode-1"])
}

func TestHealthChecker_NonKubeconfig_Healthy(t *testing.T) {
	// Scenario: :6443 is reachable AND 3 node agents online
	// Expected: health_status = "healthy", status = "connected", nodes = 3
	c := fleet.Cluster{
		ID:           "cluster-k8snode-2",
		Name:         "k8snode",
		ImportMethod: "agent",
		Status:       "unknown",
		HealthStatus: "unknown",
		Nodes:        0,
		APIServerURL: "https://10.10.10.133:6443",
	}

	fleetRepo := newMockFleetHealthRepo([]fleet.Cluster{c})
	discovery := &mockDiscoveryPort{}

	hostRepo := &mockComputeHostRepoForHealth{
		hosts: []docker.ComputeHost{
			{ID: "host-1", Name: "node-1", Endpoint: "http://10.10.10.133:9100"},
			{ID: "host-2", Name: "node-2", Endpoint: "http://10.10.10.134:9100"},
		},
	}

	// Dial to :6443 succeeds
	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		clientConn, serverConn := net.Pipe()
		_ = serverConn.Close()
		return clientConn, nil
	}

	httpClient := &http.Client{
		Transport: &mockHTTPTransport{
			healthyEndpoints: map[string]bool{
				"http://10.10.10.133:9100": true,
				"http://10.10.10.134:9100": true,
			},
		},
	}

	checker := NewHealthChecker(fleetRepo, discovery, zap.NewNop(),
		WithComputeHostRepo(hostRepo),
		WithDialContext(dialFn),
		WithHTTPClient(httpClient),
	)

	checker.checkCluster(context.Background(), c)

	fleetRepo.mu.Lock()
	defer fleetRepo.mu.Unlock()

	assert.Equal(t, "healthy", fleetRepo.healthCalls["cluster-k8snode-2"])
	assert.Equal(t, "connected", fleetRepo.statusCalls["cluster-k8snode-2"])
	assert.Equal(t, 2, fleetRepo.nodeCalls["cluster-k8snode-2"])
}

func TestHealthChecker_NonKubeconfig_Offline(t *testing.T) {
	// Scenario: :6443 unreachable AND node agents unreachable
	// Expected: health_status = "unhealthy", status = "offline", nodes = 0
	c := fleet.Cluster{
		ID:           "cluster-dead-1",
		Name:         "dead-cluster",
		ImportMethod: "agent",
		Status:       "connected",
		HealthStatus: "healthy",
		Nodes:        2,
	}

	fleetRepo := newMockFleetHealthRepo([]fleet.Cluster{c})
	discovery := &mockDiscoveryPort{}

	hostRepo := &mockComputeHostRepoForHealth{
		hosts: []docker.ComputeHost{
			{ID: "host-1", Name: "dead-1", Endpoint: "http://10.10.10.50:9100", Labels: map[string]string{"cluster": "dead-cluster"}},
		},
	}

	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}

	httpClient := &http.Client{
		Transport: &mockHTTPTransport{
			healthyEndpoints: map[string]bool{
				"http://10.10.10.50:9100": false,
			},
		},
	}

	checker := NewHealthChecker(fleetRepo, discovery, zap.NewNop(),
		WithComputeHostRepo(hostRepo),
		WithDialContext(dialFn),
		WithHTTPClient(httpClient),
	)

	checker.checkCluster(context.Background(), c)

	fleetRepo.mu.Lock()
	defer fleetRepo.mu.Unlock()

	assert.Equal(t, "unhealthy", fleetRepo.healthCalls["cluster-dead-1"])
	assert.Equal(t, "offline", fleetRepo.statusCalls["cluster-dead-1"])
	assert.Equal(t, 0, fleetRepo.nodeCalls["cluster-dead-1"])
}

func TestHealthChecker_ImmediateCheckOnStart(t *testing.T) {
	c := fleet.Cluster{
		ID:           "cluster-imm-1",
		Name:         "k8snode",
		ImportMethod: "agent",
	}

	fleetRepo := newMockFleetHealthRepo([]fleet.Cluster{c})
	discovery := &mockDiscoveryPort{}

	checker := NewHealthChecker(fleetRepo, discovery, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with a very long interval (1 hour). It should check immediately!
	go checker.Start(ctx, 1*time.Hour)

	require.Eventually(t, func() bool {
		fleetRepo.mu.Lock()
		defer fleetRepo.mu.Unlock()
		_, checked := fleetRepo.healthCalls["cluster-imm-1"]
		return checked
	}, 2*time.Second, 50*time.Millisecond, "expected health check to run immediately on Start")
}

func TestHealthChecker_BackgroundContext_EnrichedWithPlatformAdminAndDefaultTenant(t *testing.T) {
	c := fleet.Cluster{
		ID:           "cluster-k8snode",
		Name:         "k8snode",
		TenantID:     "default-tenant",
		ImportMethod: "agent",
	}

	fleetRepo := newMockFleetHealthRepo([]fleet.Cluster{c})
	discovery := &mockDiscoveryPort{}

	checker := NewHealthChecker(fleetRepo, discovery, zap.NewNop())

	// Call checkAll with a bare context.Background() - simulating background daemon!
	checker.checkAll(context.Background())

	fleetRepo.mu.Lock()
	defer fleetRepo.mu.Unlock()

	// Verify ListClusters received platform_admin context
	require.NotNil(t, fleetRepo.lastListCtx, "ListClusters should have been called")
	assert.Equal(t, "platform_admin", tenancy.UserRoleFromContext(fleetRepo.lastListCtx), "ListClusters should receive platform_admin role")
	assert.Equal(t, "default-tenant", tenancy.TenantIDFromContext(fleetRepo.lastListCtx), "ListClusters should receive default-tenant")

	// Verify UpdateHealth received platform_admin context
	require.NotNil(t, fleetRepo.lastHealthCtx, "UpdateHealth should have been called")
	assert.Equal(t, "platform_admin", tenancy.UserRoleFromContext(fleetRepo.lastHealthCtx), "UpdateHealth should receive platform_admin role")
	assert.Equal(t, "default-tenant", tenancy.TenantIDFromContext(fleetRepo.lastHealthCtx), "UpdateHealth should receive default-tenant")
}
