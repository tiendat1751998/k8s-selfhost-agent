package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
)

func TestLogHandler_Search_ChronologicalAscendingOrder_WithAliases(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	t0 := now.Add(-10 * time.Minute)
	t1 := now.Add(-5 * time.Minute)
	t2 := now.Add(-1 * time.Minute)

	fake := &fakeLoggingService{
		searchResult: &logging.LogSearchResult{
			Entries: []logging.LogEntry{
				{Timestamp: t0, ContainerName: "postgres", Message: "database system is ready to accept connections"},
				{Timestamp: t1, ContainerName: "postgres", Message: "checkpoint complete"},
				{Timestamp: t2, ContainerName: "postgres", Message: "autovacuum completed"},
			},
			TotalCount: 3,
		},
	}

	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?container=postgres&limit=10", nil)
	ctx := withTenantContext(req.Context(), "tenant-1", "viewer")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.HandleSearch(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result logging.LogSearchResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Entries, 3)

	// Verify entries are strictly chronological (ascending: oldest first, newest last)
	require.True(t, result.Entries[0].Timestamp.Before(result.Entries[1].Timestamp), "entry 0 must be older than entry 1")
	require.True(t, result.Entries[1].Timestamp.Before(result.Entries[2].Timestamp), "entry 1 must be older than entry 2")
	require.Equal(t, "database system is ready to accept connections", result.Entries[0].Message)
	require.Equal(t, "autovacuum completed", result.Entries[2].Message)
}

func TestLogHandler_Search_ChronologicalAscendingOrder_DirectQuery(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	t0 := now.Add(-10 * time.Minute)
	t1 := now.Add(-5 * time.Minute)
	t2 := now.Add(-1 * time.Minute)

	// Simulate repository returning descending rows (newest first)
	fake := &fakeLoggingService{
		searchResult: &logging.LogSearchResult{
			Entries: []logging.LogEntry{
				{Timestamp: t2, ContainerName: "my-service", Message: "newest entry"},
				{Timestamp: t1, ContainerName: "my-service", Message: "middle entry"},
				{Timestamp: t0, ContainerName: "my-service", Message: "oldest entry"},
			},
			TotalCount: 3,
		},
	}

	h := NewLogHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search?service=my-service&limit=10", nil)
	ctx := withTenantContext(req.Context(), "tenant-1", "viewer")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.HandleSearch(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result logging.LogSearchResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Entries, 3)

	// Verify entries are sorted ascending chronologically
	require.True(t, result.Entries[0].Timestamp.Before(result.Entries[1].Timestamp))
	require.True(t, result.Entries[1].Timestamp.Before(result.Entries[2].Timestamp))
	require.Equal(t, "oldest entry", result.Entries[0].Message)
	require.Equal(t, "newest entry", result.Entries[2].Message)
}
