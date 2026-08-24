package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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

// DockerClientInterface abstracts Docker SDK client methods needed for log reading.
type DockerClientInterface interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
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
		// 1. Try Docker client
		if dockerSrc := newDockerLogSource(); dockerSrc != nil {
			s.sources = append(s.sources, dockerSrc)
		}
		// 2. File log source
		if s.logDir != "" {
			s.sources = append(s.sources, &FileLogSource{logDir: s.logDir})
		}
		// 3. Journalctl log source (Linux only)
		if journalSrc := newJournalctlLogSource(); journalSrc != nil {
			s.sources = append(s.sources, journalSrc)
		}
	}

	return s
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

// QueryLogs queries logs across sources matching the given filters.
func (s *LogServer) QueryLogs(ctx context.Context, app string, tail int, sinceStr, untilStr, query, level string) ([]LogEntry, error) {
	s.mu.RLock()
	sources := s.sources
	s.mu.RUnlock()

	sinceTime, err := parseTimeParam(sinceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid since parameter: %w", err)
	}

	untilTime, err := parseTimeParam(untilStr)
	if err != nil {
		return nil, fmt.Errorf("invalid until parameter: %w", err)
	}

	var allEntries []LogEntry
	for _, src := range sources {
		entries, err := src.GetLogs(ctx, app, tail, sinceTime, untilTime, query, level)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	// Sort chronologically
	sort.SliceStable(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Tail filtering if specified and positive
	if tail > 0 && len(allEntries) > tail {
		allEntries = allEntries[len(allEntries)-tail:]
	}

	return allEntries, nil
}

// SearchLogs executes search query across logs.
func (s *LogServer) SearchLogs(ctx context.Context, req LogSearchRequest) ([]LogEntry, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	// Query with all lines and then limit
	entries, err := s.QueryLogs(ctx, req.App, 0, req.Since, req.Until, req.Query, req.Level)
	if err != nil {
		return nil, err
	}

	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries, nil
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

// formatLogEntry formats a LogEntry into a line string.
func formatLogEntry(e LogEntry) string {
	if e.Raw != "" {
		return e.Raw
	}
	ts := e.Timestamp.UTC().Format(time.RFC3339Nano)
	if e.Service != "" {
		if e.Level != "" {
			return fmt.Sprintf("%s [%s] [%s] %s", ts, e.Service, strings.ToUpper(e.Level), e.Message)
		}
		return fmt.Sprintf("%s [%s] %s", ts, e.Service, e.Message)
	}
	if e.Level != "" {
		return fmt.Sprintf("%s [%s] %s", ts, strings.ToUpper(e.Level), e.Message)
	}
	return fmt.Sprintf("%s %s", ts, e.Message)
}

// -----------------------------------------------------------------------------
// Helper parsing functions
// -----------------------------------------------------------------------------

// parseLogLine parses timestamp, level, message from raw log line.
func parseLogLine(rawLine string, defaultService string) LogEntry {
	rawLine = strings.TrimRight(rawLine, "\r\n")
	entry := LogEntry{
		Service: defaultService,
		Raw:     rawLine,
	}
	if strings.TrimSpace(rawLine) == "" {
		entry.Timestamp = time.Now().UTC()
		entry.Level = "info"
		return entry
	}

	// Try extracting timestamp from beginning of line (e.g. Docker format: "2026-08-24T14:30:00.123456789Z message")
	fields := strings.SplitN(rawLine, " ", 2)
	parsedTime := false
	if len(fields) >= 1 {
		tsCandidate := strings.Trim(fields[0], "[]")
		if t, err := time.Parse(time.RFC3339Nano, tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse(time.RFC3339, tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		} else if t, err := time.Parse("2006-01-02 15:04:05", tsCandidate); err == nil {
			entry.Timestamp = t.UTC()
			parsedTime = true
			if len(fields) > 1 {
				entry.Message = fields[1]
			}
		}
	}

	if !parsedTime {
		entry.Timestamp = time.Now().UTC()
		entry.Message = rawLine
	}

	entry.Level = detectLogLevel(rawLine)
	return entry
}

// detectLogLevel classifies log level from content.
func detectLogLevel(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "err") ||
		strings.Contains(lower, "fatal") || strings.Contains(lower, "crit") ||
		strings.Contains(lower, "panic") || strings.Contains(lower, "level=error") ||
		strings.Contains(lower, `"level":"error"`) {
		return "error"
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") ||
		strings.Contains(lower, "level=warn") || strings.Contains(lower, `"level":"warn"`) {
		return "warn"
	}
	if strings.Contains(lower, "debug") || strings.Contains(lower, "trace") ||
		strings.Contains(lower, "level=debug") || strings.Contains(lower, `"level":"debug"`) {
		return "debug"
	}
	return "info"
}

// matchLevel checks if detected level satisfies the requested filter level.
func matchLevel(detectedLevel, filterLevel string) bool {
	filterLevel = strings.TrimSpace(strings.ToLower(filterLevel))
	if filterLevel == "" {
		return true
	}
	detectedLevel = strings.TrimSpace(strings.ToLower(detectedLevel))

	switch filterLevel {
	case "error", "err", "fatal", "critical":
		return detectedLevel == "error" || detectedLevel == "fatal" || detectedLevel == "critical"
	case "warn", "warning":
		return detectedLevel == "error" || detectedLevel == "fatal" || detectedLevel == "critical" || detectedLevel == "warn"
	case "info":
		return detectedLevel == "error" || detectedLevel == "warn" || detectedLevel == "info"
	case "debug", "trace":
		return true
	default:
		return detectedLevel == filterLevel || strings.Contains(detectedLevel, filterLevel)
	}
}

// matchTime checks if timestamp is within [since, until] window.
func matchTime(ts time.Time, sinceTime, untilTime *time.Time) bool {
	if sinceTime != nil && ts.Before(*sinceTime) {
		return false
	}
	if untilTime != nil && ts.After(*untilTime) {
		return false
	}
	return true
}

// matchQuery checks if query substring is in message or raw line.
func matchQuery(raw, msg, query string) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(raw), query) || strings.Contains(strings.ToLower(msg), query)
}

// parseTimeParam parses ISO timestamp or relative duration (e.g. "15m", "1h", "24h").
func parseTimeParam(val string) (*time.Time, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil, nil
	}

	// Try relative duration
	if d, err := parseRelativeDuration(val); err == nil {
		t := time.Now().UTC().Add(-d)
		return &t, nil
	}

	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try "2006-01-02T15:04:05"
	if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try "2006-01-02 15:04:05"
	if t, err := time.Parse("2006-01-02 15:04:05", val); err == nil {
		tUTC := t.UTC()
		return &tUTC, nil
	}

	// Try unix timestamp
	if sec, err := strconv.ParseInt(val, 10, 64); err == nil {
		t := time.Unix(sec, 0).UTC()
		return &t, nil
	}

	return nil, fmt.Errorf("unrecognized time format: %q", val)
}

// parseRelativeDuration parses durations including day ('d') and week ('w') suffixes.
func parseRelativeDuration(val string) (time.Duration, error) {
	val = strings.TrimSpace(val)
	if strings.HasSuffix(val, "d") {
		numStr := strings.TrimSuffix(val, "d")
		days, err := strconv.Atoi(numStr)
		if err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	if strings.HasSuffix(val, "w") {
		numStr := strings.TrimSuffix(val, "w")
		weeks, err := strconv.Atoi(numStr)
		if err == nil {
			return time.Duration(weeks) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(val)
}

// -----------------------------------------------------------------------------
// Log Sources Implementations
// -----------------------------------------------------------------------------

// DockerLogSource reads container logs from the Docker socket / engine.
type DockerLogSource struct {
	cli DockerClientInterface
}

func newDockerLogSource() *DockerLogSource {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close()
		return nil
	}
	return &DockerLogSource{cli: cli}
}

// NewDockerLogSourceWithClient creates a DockerLogSource with a custom/mock client.
func NewDockerLogSourceWithClient(cli DockerClientInterface) *DockerLogSource {
	return &DockerLogSource{cli: cli}
}

func (d *DockerLogSource) ListServices(ctx context.Context) ([]string, error) {
	if d.cli == nil {
		return nil, nil
	}
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var services []string
	seen := make(map[string]bool)
	for _, c := range containers {
		// Swarm service name label
		if swarmSvc, ok := c.Labels["com.docker.swarm.service.name"]; ok && swarmSvc != "" {
			if !seen[swarmSvc] {
				seen[swarmSvc] = true
				services = append(services, swarmSvc)
			}
		}
		// Compose service name label
		if composeSvc, ok := c.Labels["com.docker.compose.service"]; ok && composeSvc != "" {
			if !seen[composeSvc] {
				seen[composeSvc] = true
				services = append(services, composeSvc)
			}
		}
		// Container names
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if cleanName != "" && !seen[cleanName] {
				seen[cleanName] = true
				services = append(services, cleanName)
			}
		}
	}
	return services, nil
}

func (d *DockerLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	if d.cli == nil {
		return nil, nil
	}

	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	tailStr := "100"
	if tail == 0 {
		tailStr = "all"
	} else if tail > 0 {
		tailStr = strconv.Itoa(tail)
	}

	var sinceStr string
	if sinceTime != nil {
		sinceStr = sinceTime.Format(time.RFC3339Nano)
	}
	var untilStr string
	if untilTime != nil {
		untilStr = untilTime.Format(time.RFC3339Nano)
	}

	var allEntries []LogEntry
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		swarmSvc := c.Labels["com.docker.swarm.service.name"]
		composeSvc := c.Labels["com.docker.compose.service"]

		// If app specified, check match
		if app != "" {
			matched := strings.EqualFold(name, app) ||
				strings.Contains(strings.ToLower(name), strings.ToLower(app)) ||
				strings.EqualFold(swarmSvc, app) ||
				strings.EqualFold(composeSvc, app) ||
				strings.HasPrefix(c.ID, app)
			if !matched {
				continue
			}
		}

		serviceName := swarmSvc
		if serviceName == "" {
			serviceName = composeSvc
		}
		if serviceName == "" {
			serviceName = name
		}

		logsReader, err := d.cli.ContainerLogs(ctx, c.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       tailStr,
			Since:      sinceStr,
			Until:      untilStr,
			Timestamps: true,
		})
		if err != nil {
			continue
		}

		entries := readDockerLogStream(logsReader, serviceName, sinceTime, untilTime, query, level)
		_ = logsReader.Close()
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func readDockerLogStream(reader io.Reader, serviceName string, sinceTime, untilTime *time.Time, query string, level string) []LogEntry {
	var entries []LogEntry
	var buf bytes.Buffer
	_, err := stdcopy.StdCopy(&buf, &buf, reader)
	var scanner *bufio.Scanner
	if err == nil && buf.Len() > 0 {
		scanner = bufio.NewScanner(&buf)
	} else {
		scanner = bufio.NewScanner(reader)
	}

	for scanner.Scan() {
		line := scanner.Text()
		entry := parseLogLine(line, serviceName)
		if !matchTime(entry.Timestamp, sinceTime, untilTime) {
			continue
		}
		if !matchLevel(entry.Level, level) {
			continue
		}
		if !matchQuery(entry.Raw, entry.Message, query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// FileLogSource reads local .log files in log directory.
type FileLogSource struct {
	logDir string
}

func (f *FileLogSource) ListServices(ctx context.Context) ([]string, error) {
	if f.logDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(f.logDir)
	if err != nil {
		return nil, nil
	}

	var services []string
	for _, e := range entries {
		if e.IsDir() {
			services = append(services, e.Name())
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".log") {
			svcName := strings.TrimSuffix(name, ".log")
			services = append(services, svcName)
		}
	}
	return services, nil
}

func (f *FileLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	if f.logDir == "" {
		return nil, nil
	}
	dirEntries, err := os.ReadDir(f.logDir)
	if err != nil {
		return nil, nil
	}

	var allEntries []LogEntry
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		fileName := e.Name()
		if !strings.HasSuffix(fileName, ".log") {
			continue
		}
		svcName := strings.TrimSuffix(fileName, ".log")

		if app != "" && !strings.EqualFold(svcName, app) && !strings.Contains(strings.ToLower(svcName), strings.ToLower(app)) {
			continue
		}

		filePath := filepath.Join(f.logDir, fileName)
		fileEntries, err := readLogFile(filePath, svcName, sinceTime, untilTime, query, level)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, fileEntries...)
	}

	return allEntries, nil
}

func readLogFile(path, service string, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		entry := parseLogLine(line, service)
		if !matchTime(entry.Timestamp, sinceTime, untilTime) {
			continue
		}
		if !matchLevel(entry.Level, level) {
			continue
		}
		if !matchQuery(entry.Raw, entry.Message, query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// JournalctlLogSource queries systemd journalctl if available.
type JournalctlLogSource struct {
	available bool
}

func newJournalctlLogSource() *JournalctlLogSource {
	_, err := exec.LookPath("journalctl")
	if err != nil {
		return nil
	}
	return &JournalctlLogSource{available: true}
}

func (j *JournalctlLogSource) ListServices(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (j *JournalctlLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	if !j.available || app == "" {
		return nil, nil
	}

	args := []string{"-u", app, "--no-pager", "-o", "short-iso"}
	if tail > 0 {
		args = append(args, "-n", strconv.Itoa(tail))
	}
	if sinceTime != nil {
		args = append(args, "--since", sinceTime.Format("2006-01-02 15:04:05"))
	}
	if untilTime != nil {
		args = append(args, "--until", untilTime.Format("2006-01-02 15:04:05"))
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var entries []LogEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		entry := parseLogLine(line, app)
		if !matchLevel(entry.Level, level) {
			continue
		}
		if !matchQuery(entry.Raw, entry.Message, query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// MemoryLogSource is an in-memory log source for testing and fast mocking.
type MemoryLogSource struct {
	mu   sync.RWMutex
	logs map[string][]LogEntry
}

// NewMemoryLogSource creates an empty MemoryLogSource.
func NewMemoryLogSource() *MemoryLogSource {
	return &MemoryLogSource{
		logs: make(map[string][]LogEntry),
	}
}

// AddEntry adds a single log entry.
func (m *MemoryLogSource) AddEntry(service string, timestamp time.Time, level, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := LogEntry{
		Timestamp: timestamp.UTC(),
		Service:   service,
		Message:   message,
		Level:     strings.ToLower(level),
		Raw:       fmt.Sprintf("%s [%s] %s", timestamp.UTC().Format(time.RFC3339Nano), level, message),
	}
	m.logs[service] = append(m.logs[service], entry)
}

func (m *MemoryLogSource) ListServices(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var services []string
	for svc := range m.logs {
		services = append(services, svc)
	}
	sort.Strings(services)
	return services, nil
}

func (m *MemoryLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allEntries []LogEntry
	for svc, entries := range m.logs {
		if app != "" && !strings.EqualFold(svc, app) && !strings.Contains(strings.ToLower(svc), strings.ToLower(app)) {
			continue
		}
		for _, e := range entries {
			if !matchTime(e.Timestamp, sinceTime, untilTime) {
				continue
			}
			if !matchLevel(e.Level, level) {
				continue
			}
			if !matchQuery(e.Raw, e.Message, query) {
				continue
			}
			allEntries = append(allEntries, e)
		}
	}
	return allEntries, nil
}
