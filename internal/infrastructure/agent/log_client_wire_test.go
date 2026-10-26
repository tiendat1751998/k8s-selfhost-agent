package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogClient_AgentWireProtocol_RoundTrip verifies lossless bidirectional K8s metadata
// between Central Backend LogClient and k8s-agent HTTP server endpoints.
func TestLogClient_AgentWireProtocol_RoundTrip(t *testing.T) {
	expectedToken := "agent-wire-token-xyz"
	now := time.Now().UTC().Truncate(time.Millisecond)

	var receivedSearchReq map[string]any
	var receivedGetParams map[string]string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+expectedToken, r.Header.Get("Authorization"))

		switch r.URL.Path {
		case "/logs/search":
			assert.Equal(t, http.MethodPost, r.Method)
			var raw map[string]any
			err := json.NewDecoder(r.Body).Decode(&raw)
			require.NoError(t, err)
			receivedSearchReq = raw

			// Simulate agent response with full K8s metadata using agent JSON tags
			resp := map[string]any{
				"results": []map[string]any{
					{
						"timestamp": now.Format(time.RFC3339Nano),
						"service":   "billing-svc",
						"message":   "panic: nil pointer dereference",
						"level":     "error",
						"raw":       "2026-10-26T00:00:00Z [billing-svc] [ERROR] panic: nil pointer dereference",
						"namespace": "finance",
						"pod":       "billing-worker-7b98d",
						"container": "billing-svc",
						"stream":    "stderr",
					},
				},
				"total": 1,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case "/logs":
			assert.Equal(t, http.MethodGet, r.Method)
			q := r.URL.Query()
			receivedGetParams = map[string]string{
				"app":       q.Get("app"),
				"tail":      q.Get("tail"),
				"level":     q.Get("level"),
				"namespace": q.Get("namespace"),
				"pod":       q.Get("pod"),
				"container": q.Get("container"),
				"stream":    q.Get("stream"),
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("2026-10-26T00:00:00Z [billing-svc] [ERROR] panic: nil pointer dereference\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	client := NewAgentLogClient()

	t.Run("POST /logs/search sends K8s metadata and parses response losslessly", func(t *testing.T) {
		searchReq := SearchLogsRequest{
			Query:         "panic",
			Level:         "error",
			Limit:         25,
			Namespace:     "finance",
			PodName:       "billing-worker-7b98d",
			ContainerName: "billing-svc",
			Stream:        "stderr",
		}

		results, err := client.SearchNodeLogs(context.Background(), mockServer.URL, expectedToken, searchReq)
		require.NoError(t, err)

		// 1. Verify JSON payload wire tags match agent expectations (namespace, pod, container, stream)
		require.NotNil(t, receivedSearchReq)
		assert.Equal(t, "finance", receivedSearchReq["namespace"])
		assert.Equal(t, "billing-worker-7b98d", receivedSearchReq["pod"])
		assert.Equal(t, "billing-svc", receivedSearchReq["container"])
		assert.Equal(t, "stderr", receivedSearchReq["stream"])
		assert.Equal(t, "panic", receivedSearchReq["query"])

		// 2. Verify deserialized LogSearchResult holds full K8s metadata
		require.Len(t, results, 1)
		res := results[0]
		assert.Equal(t, "finance", res.Namespace)
		assert.Equal(t, "billing-worker-7b98d", res.PodName)
		assert.Equal(t, "billing-svc", res.ContainerName)
		assert.Equal(t, "stderr", res.Stream)
		assert.Equal(t, "error", res.Level)
		assert.Equal(t, "panic: nil pointer dereference", res.Message)
	})

	t.Run("GET /logs propagates K8s query params correctly", func(t *testing.T) {
		_, err := client.GetNodeLogs(
			context.Background(), mockServer.URL, expectedToken,
			"billing-svc", "100", "", "", "panic", "error",
			"finance", "billing-worker-7b98d", "billing-svc", "stderr",
		)
		require.NoError(t, err)

		require.NotNil(t, receivedGetParams)
		assert.Equal(t, "finance", receivedGetParams["namespace"])
		assert.Equal(t, "billing-worker-7b98d", receivedGetParams["pod"])
		assert.Equal(t, "billing-svc", receivedGetParams["container"])
		assert.Equal(t, "stderr", receivedGetParams["stream"])
		assert.Equal(t, "billing-svc", receivedGetParams["app"])
	})
}

type wireMockHostRepo struct {
	hosts []docker.ComputeHost
}

func (m *wireMockHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *wireMockHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	for _, h := range m.hosts {
		if h.ID == id {
			return &h, nil
		}
	}
	return nil, fmt.Errorf("host not found")
}
func (m *wireMockHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *wireMockHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *wireMockHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *wireMockHostRepo) Delete(ctx context.Context, id string) error                 { return nil }
func (m *wireMockHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

type wireMockLogClient struct {
	clusterResults []LogSearchResult
	lastReq        SearchLogsRequest
}

func (m *wireMockLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string, k8sMeta ...string) (string, error) {
	return "", nil
}
func (m *wireMockLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req SearchLogsRequest) ([]LogSearchResult, error) {
	return nil, nil
}
func (m *wireMockLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req SearchLogsRequest) ([]LogSearchResult, error) {
	m.lastReq = req
	return m.clusterResults, nil
}
func (m *wireMockLogClient) ListNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return nil, nil
}
func (m *wireMockLogClient) GetNodeServices(ctx context.Context, hostEndpoint, authToken string) ([]string, error) {
	return nil, nil
}
func (m *wireMockLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	return nil
}

// TestDistributedAgentLogRepo_K8sAndNonK8sFilters verifies that DistributedAgentLogRepo:
// 1. Populates K8s fields and maps them into domainLogging.LogEntry
// 2. Preserves non-K8s service filtering fallback without clobbering
func TestDistributedAgentLogRepo_K8sAndNonK8sFilters(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	hosts := []docker.ComputeHost{
		{ID: "node-k8s", Name: "k8smasterdeb", Endpoint: "http://10.10.10.60:9100", Status: "connected"},
	}
	hostRepo := &wireMockHostRepo{hosts: hosts}

	t.Run("K8s multi-dimensional query populates searchReq and maps results", func(t *testing.T) {
		client := &wireMockLogClient{
			clusterResults: []LogSearchResult{
				{
					Timestamp:     now,
					NodeID:        "node-k8s",
					NodeName:      "k8smasterdeb",
					Service:       "traefik-ingress",
					Message:       "error routing request",
					Level:         "error",
					Namespace:     "kube-system",
					PodName:       "traefik-ingress-xyz",
					ContainerName: "traefik",
					Stream:        "stderr",
				},
			},
		}

		repo := NewDistributedAgentLogRepo(hostRepo, client)

		filter := domainLogging.LogFilter{
			TenantID:      "default",
			ClusterID:     "cluster-mtay50kl",
			Namespace:     "kube-system",
			PodName:       "traefik-ingress-xyz",
			ContainerName: "traefik",
			Stream:        "stderr",
			LogLevel:      domainLogging.LogLevelError,
			SearchText:    "routing",
			Limit:         50,
		}

		res, err := repo.QueryLogs(ctx, filter)
		require.NoError(t, err)

		// Verify searchReq sent to cluster
		assert.Equal(t, "kube-system", client.lastReq.Namespace)
		assert.Equal(t, "traefik-ingress-xyz", client.lastReq.PodName)
		assert.Equal(t, "traefik", client.lastReq.ContainerName)
		assert.Equal(t, "stderr", client.lastReq.Stream)
		assert.Equal(t, "", client.lastReq.App, "App should be empty for K8s queries to avoid clobbering pod name")

		// Verify domain LogEntry returned has K8s metadata mapped from agent response
		require.Len(t, res.Entries, 1)
		entry := res.Entries[0]
		assert.Equal(t, "kube-system", entry.Namespace)
		assert.Equal(t, "traefik-ingress-xyz", entry.PodName)
		assert.Equal(t, "traefik", entry.ContainerName)
		assert.Equal(t, "stderr", entry.Stream)
		assert.Equal(t, domainLogging.LogLevelError, entry.LogLevel)
		assert.Equal(t, "error routing request", entry.Message)
		assert.Equal(t, "traefik-ingress-xyz", entry.Attributes["pod"])
		assert.Equal(t, "traefik", entry.Attributes["container"])
		assert.Equal(t, "stderr", entry.Attributes["stream"])
	})

	t.Run("Non-K8s systemd/docker query uses App fallback without breaking", func(t *testing.T) {
		client := &wireMockLogClient{
			clusterResults: []LogSearchResult{
				{
					Timestamp: now,
					NodeID:    "node-k8s",
					NodeName:  "k8smasterdeb",
					Service:   "postgresql.service",
					Message:   "database system is ready to accept connections",
					Level:     "info",
				},
			},
		}

		repo := NewDistributedAgentLogRepo(hostRepo, client)

		// When user selects a systemd unit or container on non-K8s:
		// log_handler sets ContainerName="postgresql.service", Namespace="", PodName=""
		filter := domainLogging.LogFilter{
			TenantID:      "default",
			ClusterID:     "cluster-mtay50kl",
			ContainerName: "postgresql.service",
			ServiceName:   "postgresql.service",
			LogLevel:      domainLogging.LogLevelInfo,
			Limit:         50,
		}

		res, err := repo.QueryLogs(ctx, filter)
		require.NoError(t, err)

		// Verify searchReq has App fallback and empty K8s fields
		assert.Equal(t, "postgresql.service", client.lastReq.App)
		assert.Equal(t, "", client.lastReq.Namespace)
		assert.Equal(t, "", client.lastReq.PodName)
		assert.Equal(t, "", client.lastReq.ContainerName)

		require.Len(t, res.Entries, 1)
		entry := res.Entries[0]
		assert.Equal(t, "postgresql.service", entry.ContainerName)
		assert.Equal(t, "stdout", entry.Stream)
	})
}
