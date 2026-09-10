package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// DockerClientInterface abstracts Docker SDK client methods needed for log reading.
type DockerClientInterface interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
}

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
		if swarmSvc, ok := c.Labels["com.docker.swarm.service.name"]; ok && swarmSvc != "" && !seen[swarmSvc] {
			seen[swarmSvc] = true
			services = append(services, swarmSvc)
		}
		if composeSvc, ok := c.Labels["com.docker.compose.service"]; ok && composeSvc != "" && !seen[composeSvc] {
			seen[composeSvc] = true
			services = append(services, composeSvc)
		}
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

	var sinceStr, untilStr string
	if sinceTime != nil {
		sinceStr = sinceTime.Format(time.RFC3339Nano)
	}
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
		if !matchTime(entry.Timestamp, sinceTime, untilTime) || !matchLevel(entry.Level, level) || !matchQuery(entry.Raw, entry.Message, query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// FileLogSource reads local .log files in log directory and can pipe to log engine.
type FileLogSource struct {
	logDir string
	writer *logengine.Writer
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
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".log") {
			services = append(services, strings.TrimSuffix(name, ".log"))
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
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		svcName := strings.TrimSuffix(e.Name(), ".log")
		if app != "" && !strings.EqualFold(svcName, app) && !strings.Contains(strings.ToLower(svcName), strings.ToLower(app)) {
			continue
		}

		filePath := filepath.Join(f.logDir, e.Name())
		fileEntries, err := readLogFile(filePath, svcName, sinceTime, untilTime, query, level)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, fileEntries...)
	}

	return allEntries, nil
}

// IngestToWriter pipes all existing .log files into the columnar logengine.Writer safely.
func (f *FileLogSource) IngestToWriter(ctx context.Context, w *logengine.Writer) error {
	if f.logDir == "" || w == nil {
		return nil
	}
	dirEntries, err := os.ReadDir(f.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log dir %s: %w", f.logDir, err)
	}

	var writeErrors int
	for _, e := range dirEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		svcName := strings.TrimSuffix(e.Name(), ".log")
		filePath := filepath.Join(f.logDir, e.Name())

		err := func() error {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				entry := parseLogLine(scanner.Text(), svcName)
				if err := w.Write(logengine.Entry{
					Timestamp: entry.Timestamp,
					Service:   entry.Service,
					Level:     entry.Level,
					Message:   entry.Message,
					Raw:       entry.Raw,
				}); err != nil {
					writeErrors++
				}
			}
			return scanner.Err()
		}()
		if err != nil && ctx.Err() != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	if writeErrors > 0 {
		return fmt.Errorf("encountered %d write errors during ingestion", writeErrors)
	}
	return nil
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
		entry := parseLogLine(scanner.Text(), service)
		if !matchTime(entry.Timestamp, sinceTime, untilTime) || !matchLevel(entry.Level, level) || !matchQuery(entry.Raw, entry.Message, query) {
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
	if _, err := exec.LookPath("journalctl"); err != nil {
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
		entry := parseLogLine(scanner.Text(), app)
		if !matchLevel(entry.Level, level) || !matchQuery(entry.Raw, entry.Message, query) {
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
	return &MemoryLogSource{logs: make(map[string][]LogEntry)}
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
			if !matchTime(e.Timestamp, sinceTime, untilTime) || !matchLevel(e.Level, level) || !matchQuery(e.Raw, e.Message, query) {
				continue
			}
			allEntries = append(allEntries, e)
		}
	}
	return allEntries, nil
}

// EngineLogSource wraps the embedded columnar log engine as a LogSource.
type EngineLogSource struct {
	writer *logengine.Writer
	reader *logengine.Reader
	dict   *logengine.LabelDictionary
}

// NewEngineLogSource creates a new EngineLogSource backed by logengine.
func NewEngineLogSource(engineDir string) (*EngineLogSource, error) {
	dict := logengine.NewLabelDictionary()
	w, err := logengine.NewWriter(engineDir, dict)
	if err != nil {
		return nil, err
	}
	r, err := logengine.NewReader(engineDir, dict)
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	return &EngineLogSource{writer: w, reader: r, dict: dict}, nil
}

func (e *EngineLogSource) Writer() *logengine.Writer { return e.writer }
func (e *EngineLogSource) Reader() *logengine.Reader { return e.reader }

func (e *EngineLogSource) Close() error {
	if e.writer != nil {
		return e.writer.Close()
	}
	return nil
}

func (e *EngineLogSource) ListServices(ctx context.Context) ([]string, error) {
	if e.dict == nil {
		return nil, nil
	}
	return e.dict.Export(), nil
}

func (e *EngineLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	if e.reader == nil {
		return nil, nil
	}
	var since, until time.Time
	if sinceTime != nil {
		since = *sinceTime
	}
	if untilTime != nil {
		until = *untilTime
	}
	entries, err := e.reader.Search(ctx, logengine.QueryParams{
		Service: app,
		Level:   level,
		Query:   query,
		Since:   since,
		Until:   until,
		Limit:   tail,
	})
	if err != nil {
		return nil, err
	}
	results := make([]LogEntry, len(entries))
	for i, en := range entries {
		results[i] = LogEntry{
			Timestamp: en.Timestamp,
			Service:   en.Service,
			Level:     en.Level,
			Message:   en.Message,
			Raw:       formatLogEntry(LogEntry{Timestamp: en.Timestamp, Service: en.Service, Level: en.Level, Message: en.Message}),
		}
	}
	return results, nil
}
