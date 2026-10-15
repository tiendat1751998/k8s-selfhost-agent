package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type mockComputeHostRepoScalability struct {
	hosts []docker.ComputeHost
}

func (m *mockComputeHostRepoScalability) Create(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepoScalability) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	return nil, nil
}

func (m *mockComputeHostRepoScalability) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepoScalability) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}

func (m *mockComputeHostRepoScalability) Update(ctx context.Context, host *docker.ComputeHost) error {
	return nil
}

func (m *mockComputeHostRepoScalability) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockComputeHostRepoScalability) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

func TestNewCollector_HTTPTransportTuning(t *testing.T) {
	c := NewCollector(nil, nil, nil, zap.NewNop())
	if c.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected httpClient.Transport to be *http.Transport, got %T", c.httpClient.Transport)
	}

	if transport.MaxIdleConns != 512 {
		t.Errorf("expected MaxIdleConns=512, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 4 {
		t.Errorf("expected MaxIdleConnsPerHost=4, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxConnsPerHost != 8 {
		t.Errorf("expected MaxConnsPerHost=8, got %d", transport.MaxConnsPerHost)
	}
	if transport.IdleConnTimeout != 30*time.Second {
		t.Errorf("expected IdleConnTimeout=30s, got %v", transport.IdleConnTimeout)
	}
}

func TestCollector_ScrapeAllAgents_BoundedConcurrency(t *testing.T) {
	const totalHosts = 200
	hosts := make([]docker.ComputeHost, totalHosts)
	for i := 0; i < totalHosts; i++ {
		hosts[i] = docker.ComputeHost{
			ID:       fmt.Sprintf("host-%d", i),
			Name:     fmt.Sprintf("node-%d", i),
			Endpoint: fmt.Sprintf("tcp://10.0.0.%d:2375", (i%250)+1),
		}
	}

	repo := &mockComputeHostRepoScalability{hosts: hosts}

	var activeScrapes atomic.Int64
	var maxObserved atomic.Int64

	// Create test server with delay to observe concurrency
	server := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			current := activeScrapes.Add(1)
			for {
				oldMax := maxObserved.Load()
				if current <= oldMax || maxObserved.CompareAndSwap(oldMax, current) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			activeScrapes.Add(-1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		}),
	}

	collector := NewCollector(nil, repo, nil, zap.NewNop(), WithHTTPClient(server))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collector.scrapeAllAgents(ctx)

	observed := maxObserved.Load()
	if observed > 64 {
		t.Errorf("expected max concurrent scrapes <= 64, observed %d", observed)
	}
	if observed == 0 {
		t.Errorf("expected at least 1 scrape, observed %d", observed)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
