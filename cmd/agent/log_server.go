package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a single parsed log line with metadata.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"`
	Raw       string    `json:"raw,omitempty"`
}

// LogSearchRequest represents the search payload for POST /logs/search.
type LogSearchRequest struct {
	Query string `json:"query"`
	App   string `json:"app,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Level string `json:"level,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// LogSearchResponse represents the search response for POST /logs/search.
type LogSearchResponse struct {
	Results []LogEntry `json:"results"`
	Total   int        `json:"total"`
}

// LogServicesResponse represents the services list response for GET /logs/services.
type LogServicesResponse struct {
	Services []string `json:"services"`
}

// LogsResponse represents the structured response for GET /logs?format=json.
type LogsResponse struct {
	Service string   `json:"service,omitempty"`
	Count   int      `json:"count"`
	Lines   []string `json:"lines"`
	Logs    string   `json:"logs"`
}

// LogSource defines the interface for backend log providers (Docker, local files, journalctl, memory).
type LogSource interface {
	ListServices(ctx context.Context) ([]string, error)
	GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error)
}

// LogServer manages log reading and HTTP handling on k8s-agent.
type LogServer struct {
	logDir  string
	sources []LogSource
	mu      sync.RWMutex
}

// LogServerOption configures LogServer.
type LogServerOption func(*LogServer)

// WithLogDir sets the directory for file-based log scanning.
func WithLogDir(dir string) LogServerOption {
	return func(s *LogServer) {
		if dir != "" {
			s.logDir = dir
		}
	}
}

// WithLogSource adds a custom log source provider.
func WithLogSource(src LogSource) LogServerOption {
	return func(s *LogServer) {
		if src != nil {
			s.sources = append(s.sources, src)
		}
	}
}

// NewLogServer creates a new LogServer with automatic Docker and filesystem log source discovery.
func NewLogServer(opts ...LogServerOption) *LogServer {
	s := &LogServer{
		logDir: "/var/log",
	}

	for _, opt := range opts {
		opt(s)
	}

	// If no custom sources are injected, register default providers
	if len(s.sources) == 0 {
		if dockerSrc := newDockerLogSource(); dockerSrc != nil {
			s.sources = append(s.sources, dockerSrc)
		}
		if s.logDir != "" {
			s.sources = append(s.sources, &FileLogSource{logDir: s.logDir})
		}
		if journalSrc := newJournalctlLogSource(); journalSrc != nil {
			s.sources = append(s.sources, journalSrc)
		}
	}

	return s
}

// AddSource registers an additional LogSource provider alongside existing ones.
func (s *LogServer) AddSource(src LogSource) {
	if src == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources = append(s.sources, src)
}

// ListServices returns all unique service names available across all log sources.
func (s *LogServer) ListServices(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	sources := s.sources
	s.mu.RUnlock()

	serviceSet := make(map[string]bool)
	for _, src := range sources {
		services, err := src.ListServices(ctx)
		if err != nil {
			continue
		}
		for _, svc := range services {
			svc = strings.TrimSpace(svc)
			if svc != "" {
				serviceSet[svc] = true
			}
		}
	}

	result := make([]string, 0, len(serviceSet))
	for svc := range serviceSet {
		result = append(result, svc)
	}
	sort.Strings(result)
	return result, nil
}

// HandleGetLogs handles GET /logs
func (s *LogServer) HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	app := q.Get("app")
	if app == "" {
		app = q.Get("service")
	}

	tailStr := q.Get("tail")
	tail := 100
	if tailStr == "all" || tailStr == "0" {
		tail = 0
	} else if tailStr != "" {
		if t, err := strconv.Atoi(tailStr); err == nil && t >= 0 {
			tail = t
		}
	}

	sinceStr := q.Get("since")
	untilStr := q.Get("until")
	queryStr := q.Get("q")
	if queryStr == "" {
		queryStr = q.Get("query")
	}
	levelStr := q.Get("level")
	format := strings.ToLower(q.Get("format"))

	entries, err := s.QueryLogs(r.Context(), app, tail, sinceStr, untilStr, queryStr, levelStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if format == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		lines := make([]string, len(entries))
		var buf strings.Builder
		for i, e := range entries {
			line := formatLogEntry(e)
			lines[i] = line
			buf.WriteString(line)
			buf.WriteString("\n")
		}

		resp := LogsResponse{
			Service: app,
			Count:   len(entries),
			Lines:   lines,
			Logs:    buf.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Default: text/plain
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	for _, e := range entries {
		_, _ = fmt.Fprintln(w, formatLogEntry(e))
	}
}

// HandleGetServices handles GET /logs/services
func (s *LogServer) HandleGetServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.ListServices(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LogServicesResponse{Services: services})
}

// HandleSearchLogs handles POST /logs/search
func (s *LogServer) HandleSearchLogs(w http.ResponseWriter, r *http.Request) {
	var req LogSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	results, err := s.SearchLogs(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LogSearchResponse{
		Results: results,
		Total:   len(results),
	})
}
