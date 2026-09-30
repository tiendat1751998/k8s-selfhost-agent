package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
)

func TestLogHandler_Search_NodeParam(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		expectedNode string
	}{
		{
			name:         "node query param",
			queryString:  "?node=k8smater133",
			expectedNode: "k8smater133",
		},
		{
			name:         "node_name query param fallback",
			queryString:  "?node_name=worker-node-1",
			expectedNode: "worker-node-1",
		},
		{
			name:         "host query param fallback",
			queryString:  "?host=host-compute-02",
			expectedNode: "host-compute-02",
		},
		{
			name:         "node takes precedence over node_name and host",
			queryString:  "?node=primary-node&node_name=secondary-node&host=tertiary-node",
			expectedNode: "primary-node",
		},
		{
			name:         "no node param provided",
			queryString:  "?namespace=default",
			expectedNode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLoggingService{
				searchResult: &logging.LogSearchResult{
					Entries:    []logging.LogEntry{},
					TotalCount: 0,
					HasMore:    false,
				},
			}
			h := NewLogHandler(fake)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/search"+tt.queryString, nil)
			req = req.WithContext(withTenantContext(req.Context(), "tenant-test", "viewer"))
			w := httptest.NewRecorder()

			h.HandleSearch(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
			}

			fake.mu.RLock()
			defer fake.mu.RUnlock()

			actualNode := ""
			if fake.lastFilter.Attributes != nil {
				actualNode = fake.lastFilter.Attributes["node"]
			}

			if actualNode != tt.expectedNode {
				t.Errorf("expected node attribute %q, got %q", tt.expectedNode, actualNode)
			}
		})
	}
}

func TestLogHandler_Histogram_NodeParam(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		expectedNode string
	}{
		{
			name:         "node query param",
			queryString:  "?node=k8smater133",
			expectedNode: "k8smater133",
		},
		{
			name:         "node_name query param fallback",
			queryString:  "?node_name=worker-node-1",
			expectedNode: "worker-node-1",
		},
		{
			name:         "host query param fallback",
			queryString:  "?host=host-compute-02",
			expectedNode: "host-compute-02",
		},
		{
			name:         "no node param provided",
			queryString:  "?namespace=default",
			expectedNode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLoggingService{
				histogramRes: []logging.LogAggregationBucket{},
			}
			h := NewLogHandler(fake)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/histogram"+tt.queryString, nil)
			req = req.WithContext(withTenantContext(req.Context(), "tenant-test", "viewer"))
			w := httptest.NewRecorder()

			h.HandleHistogram(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
			}

			fake.mu.RLock()
			defer fake.mu.RUnlock()

			actualNode := ""
			if fake.lastFilter.Attributes != nil {
				actualNode = fake.lastFilter.Attributes["node"]
			}

			if actualNode != tt.expectedNode {
				t.Errorf("expected node attribute %q, got %q", tt.expectedNode, actualNode)
			}
		})
	}
}
