package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerHandler_GetSwarmInfo_Inactive(t *testing.T) {
	t.Run("returns 200 with inactive response when repo returns error", func(t *testing.T) {
		repo := &testDockerRepo{
			swarmInfoErr: errors.New("This node is not a swarm manager"),
		}
		handler := NewDockerHandler(repo)

		r := chi.NewRouter()
		r.Route("/docker", handler.RegisterRoutes)

		req := httptest.NewRequest(http.MethodGet, "/docker/swarm", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp InactiveSwarmResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.False(t, resp.Active)
		assert.Equal(t, "inactive", resp.Status)
		assert.Equal(t, 0, resp.Nodes)
		assert.Equal(t, 0, resp.Managers)
		assert.Equal(t, "Docker Swarm inactive or unavailable on host", resp.Message)
	})

	t.Run("returns 200 with inactive response when repo is nil", func(t *testing.T) {
		handler := NewDockerHandler(nil)

		r := chi.NewRouter()
		r.Route("/docker", handler.RegisterRoutes)

		req := httptest.NewRequest(http.MethodGet, "/docker/swarm", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp InactiveSwarmResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.False(t, resp.Active)
		assert.Equal(t, "inactive", resp.Status)
		assert.Equal(t, 0, resp.Nodes)
		assert.Equal(t, 0, resp.Managers)
		assert.Equal(t, "Docker Swarm inactive or unavailable on host", resp.Message)
	})
}
