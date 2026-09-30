package main

import (
	"bufio"
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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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
	logDir    string
	sources   []LogSource
	engineSrc *EngineLogSource
	dockerCli DockerClientInterface
	mu        sync.RWMutex
}

// WithDockerClient configures a custom or mock DockerClientInterface for live streaming.
func WithDockerClient(cli DockerClientInterface) LogServerOption {
	return func(s *LogServer) {
		s.dockerCli = cli
	}
}

// SetDockerClient updates or assigns the DockerClientInterface on LogServer.
func (s *LogServer) SetDockerClient(cli DockerClientInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dockerCli = cli
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
			if eng, ok := src.(*EngineLogSource); ok {
				s.engineSrc = eng
			}
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
			if s.dockerCli == nil {
				s.dockerCli = dockerSrc.cli
			}
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
	if eng, ok := src.(*EngineLogSource); ok {
		s.engineSrc = eng
	}
}

// BlockCount returns total indexed blocks in the embedded columnar log engine.
func (e *EngineLogSource) BlockCount() int {
	if e == nil || e.reader == nil {
		return 0
	}
	return e.reader.BlockCount()
}

// HandleEngineStatus handles GET /logs/status reporting columnar engine health and blocks.
func (s *LogServer) HandleEngineStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	eng := s.engineSrc
	s.mu.RUnlock()

	blocks := 0
	if eng != nil {
		blocks = eng.BlockCount()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"engine": "k8s-agent embedded logengine",
		"blocks": blocks,
		"status": "healthy",
	})
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


// HandleTailLogs handles GET /logs/tail?service=<name>&tail=<n>
func (s *LogServer) HandleTailLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cli := s.dockerCli
	s.mu.RUnlock()

	if cli == nil {
		http.Error(w, "docker client unavailable", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	service := q.Get("service")
	if service == "" {
		service = q.Get("app")
	}
	if service == "" {
		service = q.Get("container")
	}

	tailStr := q.Get("tail")
	if tailStr == "" || tailStr == "0" {
		tailStr = "100"
	}

	containers, err := cli.ContainerList(r.Context(), container.ListOptions{All: true})
	if err != nil {
		http.Error(w, "failed to list containers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var targetContainerID string
	for _, c := range containers {
		cID := c.ID
		cName := cID
		if len(c.Names) > 0 {
			cName = strings.TrimPrefix(c.Names[0], "/")
		}
		swarmSvc := c.Labels["com.docker.swarm.service.name"]
		composeSvc := c.Labels["com.docker.compose.service"]

		if service == "" || service == cID || (len(cID) > 12 && service == cID[:12]) ||
			service == cName || service == swarmSvc || service == composeSvc {
			targetContainerID = cID
			break
		}
	}

	if targetContainerID == "" {
		http.Error(w, fmt.Sprintf("service %q not found", service), http.StatusNotFound)
		return
	}

	reader, err := cli.ContainerLogs(r.Context(), targetContainerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       tailStr,
	})
	if err != nil {
		http.Error(w, "failed to stream logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-r.Context().Done():
			_ = reader.Close()
		case <-done:
		}
	}()
	defer close(done)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, hasFlusher := w.(http.Flusher)

	bufReader := bufio.NewReader(reader)
	header, peekErr := bufReader.Peek(8)
	isStdCopy := peekErr == nil && len(header) == 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0

	var scanner *bufio.Scanner
	if isStdCopy {
		pr, pw := io.Pipe()
		defer pr.Close()
		go func() {
			_, _ = stdcopy.StdCopy(pw, pw, bufReader)
			_ = pw.Close()
		}()
		scanner = bufio.NewScanner(pr)
	} else {
		scanner = bufio.NewScanner(bufReader)
	}

	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		if r.Context().Err() != nil {
			return
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			return
		}
		if hasFlusher {
			flusher.Flush()
		}
	}
}
