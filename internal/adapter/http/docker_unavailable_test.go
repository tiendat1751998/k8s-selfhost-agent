package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type unavailableDockerRepo struct {
	domain.Repository
	err error
}

func (r *unavailableDockerRepo) ListContainers(ctx context.Context) ([]domain.Container, error) {
	return nil, r.err
}

func (r *unavailableDockerRepo) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return nil, r.err
}

func (r *unavailableDockerRepo) ListServices(ctx context.Context) ([]domain.Service, error) {
	return nil, r.err
}

func TestDockerHandler_GracefulDegradationOnDaemonUnavailable(t *testing.T) {
	errPatterns := []string{
		"error during connect: Get http://docker/containers/json: dial tcp 127.0.0.1:2375: connectex: connection refused",
		"open //./pipe/docker_engine: The system cannot find the file specified.",
		"dial unix /var/run/docker.sock: connect: no such file or directory",
		"Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
		"This node is not a swarm manager. Use \"docker swarm init\" or \"docker swarm join\" to connect this node to swarm",
		"docker client not initialized",
		"open \\\\.\\pipe\\docker_engine: The system cannot find the file specified.",
	}

	for _, pattern := range errPatterns {
		repo := &unavailableDockerRepo{err: errors.New(pattern)}
		h := NewDockerHandler(repo)

		// Test ListContainers
		recContainers := httptest.NewRecorder()
		reqContainers := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers", nil)
		h.ListContainers(recContainers, reqContainers)
		if recContainers.Code != http.StatusOK {
			t.Errorf("ListContainers with error %q returned code %d, expected 200", pattern, recContainers.Code)
		}
		var contResp struct {
			Data []any `json:"data"`
		}
		if err := json.NewDecoder(recContainers.Body).Decode(&contResp); err != nil || len(contResp.Data) != 0 {
			t.Errorf("expected empty data array for ListContainers, got %+v", contResp)
		}

		// Test ListNodes
		recNodes := httptest.NewRecorder()
		reqNodes := httptest.NewRequest(http.MethodGet, "/api/v1/docker/nodes", nil)
		h.ListNodes(recNodes, reqNodes)
		if recNodes.Code != http.StatusOK {
			t.Errorf("ListNodes with error %q returned code %d, expected 200", pattern, recNodes.Code)
		}

		// Test ListServices
		recServices := httptest.NewRecorder()
		reqServices := httptest.NewRequest(http.MethodGet, "/api/v1/docker/services", nil)
		h.ListServices(recServices, reqServices)
		if recServices.Code != http.StatusOK {
			t.Errorf("ListServices with error %q returned code %d, expected 200", pattern, recServices.Code)
		}
	}
}
