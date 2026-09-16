package main

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

type dockerTailerManager struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

var globalTailerManager = &dockerTailerManager{
	cancels: make(map[string]context.CancelFunc),
}

func (m *dockerTailerManager) cancel(cID string) {
	m.mu.Lock()
	cancel, exists := m.cancels[cID]
	if exists {
		delete(m.cancels, cID)
	}
	m.mu.Unlock()
	if exists && cancel != nil {
		cancel()
	}
}

func (m *dockerTailerManager) attach(
	ctx context.Context,
	dockerClient *dockerclient.Client,
	logAggregator *logging.LogAggregator,
	centralizedLogs *adapthttp.LogHandler,
	log *zap.Logger,
	cID, cName string,
) {
	m.mu.Lock()
	if _, exists := m.cancels[cID]; exists {
		m.mu.Unlock()
		return
	}
	tailerCtx, cancel := context.WithCancel(ctx)
	m.cancels[cID] = cancel
	m.mu.Unlock()

	go func(id, name string, tailerCtx context.Context) {
		defer func() {
			m.mu.Lock()
			delete(m.cancels, id)
			m.mu.Unlock()
		}()

		cleanName := strings.TrimPrefix(name, "/")
		reader, logErr := dockerClient.ContainerLogs(tailerCtx, id, container.LogsOptions{
			ShowStdout: true, ShowStderr: true, Follow: true, Tail: "200", Timestamps: true,
		})
		if logErr != nil {
			log.Debug("Failed to open Docker log stream", zap.String("container", cleanName), zap.Error(logErr))
			return
		}
		defer reader.Close()

		// Guarantee reader closure upon context cancellation
		done := make(chan struct{})
		go func() {
			select {
			case <-tailerCtx.Done():
				_ = reader.Close()
			case <-done:
			}
		}()
		defer close(done)

		stdoutReader, stdoutWriter := io.Pipe()
		stderrReader, stderrWriter := io.Pipe()

		streamPipe := func(r io.Reader, stream, defaultLvl string) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || line == "-- No entries --" {
					continue
				}
				entryTS, msg := parseDockerLogLine(line)
				if msg == "" || msg == "-- No entries --" {
					continue
				}
				lvl := detectLogLevel(msg, defaultLvl)
				logAggregator.Ingest(logging.LogEntry{
					Timestamp: entryTS, Namespace: "docker", Pod: cleanName, Container: cleanName,
					Service: cleanName, Node: "standalone-host", Stream: stream, Level: lvl, Message: msg,
				})
				if centralizedLogs != nil {
					traceID := extractTraceID(msg)
					_ = centralizedLogs.Ingest(ctx, []domainLogging.LogEntry{{
						Timestamp: entryTS, TenantID: "default-tenant", ClusterID: "default", Namespace: "docker",
						PodName: cleanName, ContainerName: cleanName, Stream: stream,
						LogLevel: domainLogging.LogLevel(strings.ToLower(lvl)), Message: msg,
						TraceID:  traceID,
						Attributes: map[string]string{"service": cleanName},
					}})
				}
			}
		}

		go streamPipe(stdoutReader, "stdout", "INFO")
		go streamPipe(stderrReader, "stderr", "WARN")

		_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
	}(cID, cName, tailerCtx)
}

func startDockerLogStreamer(
	ctx context.Context,
	dockerClient *dockerclient.Client,
	logAggregator *logging.LogAggregator,
	centralizedLogs *adapthttp.LogHandler,
	log *zap.Logger,
) {
	// 1. Discover and attach existing running containers
	go func() {
		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			log.Warn("Failed to list Docker containers for log streaming", zap.Error(err))
		} else {
			for _, c := range containers {
				cID := c.ID
				cName := cID
				if len(cID) > 12 {
					cName = cID[:12]
				}
				if len(c.Names) > 0 {
					cName = strings.TrimPrefix(c.Names[0], "/")
				}
				globalTailerManager.attach(ctx, dockerClient, logAggregator, centralizedLogs, log, cID, cName)
			}
		}

		// 2. Dynamically watch Docker container events for "start" and "restart"
		f := filters.NewArgs()
		f.Add("type", "container")
		f.Add("event", "start")
		f.Add("event", "restart")

		msgCh, errCh := dockerClient.Events(ctx, events.ListOptions{Filters: f})
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errCh:
				if !ok {
					return
				}
				if err != nil && ctx.Err() == nil {
					log.Debug("Docker log streamer event stream error", zap.Error(err))
				}
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				if msg.Action == "start" || msg.Action == "restart" {
					cID := msg.Actor.ID
					if cID == "" {
						cID = msg.ID
					}
					cName := msg.Actor.Attributes["name"]
					if cName == "" {
						cName = cID
						if len(cName) > 12 {
							cName = cName[:12]
						}
					}
					if msg.Action == "restart" {
						globalTailerManager.cancel(cID)
					}
					globalTailerManager.attach(ctx, dockerClient, logAggregator, centralizedLogs, log, cID, cName)
				}
			}
		}
	}()
}

func parseDockerLogLine(line string) (time.Time, string) {
	if idx := strings.IndexByte(line, ' '); idx >= 19 && idx <= 35 {
		rawTS := line[:idx]
		if t, err := time.Parse(time.RFC3339Nano, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
		if t, err := time.Parse(time.RFC3339, rawTS); err == nil {
			return t.UTC(), strings.TrimSpace(line[idx+1:])
		}
	}
	return time.Now().UTC(), line
}

func detectLogLevel(msg string, defaultLvl string) string {
	u := strings.ToUpper(msg)
	switch {
	case strings.Contains(u, "ERROR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC"):
		return "ERROR"
	case strings.Contains(u, "WARN"):
		return "WARN"
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
		return "DEBUG"
	case strings.Contains(u, "INFO"):
		return "INFO"
	default:
		return defaultLvl
	}
}

// extractTraceID extracts distributed trace IDs from logfmt, json, or standard logs.
func extractTraceID(msg string) string {
	patterns := []string{
		`"trace_id":`,
		`"traceId":`,
		`"traceID":`,
		`trace_id=`,
		`traceId=`,
		`traceID=`,
		`trace-id=`,
		`trace_id:`,
		`traceId:`,
	}

	for _, p := range patterns {
		idx := strings.Index(msg, p)
		if idx == -1 {
			lowerP := strings.ToLower(p)
			if lowerP != p {
				idx = strings.Index(strings.ToLower(msg), lowerP)
			}
		}
		if idx == -1 {
			continue
		}

		rest := strings.TrimSpace(msg[idx+len(p):])
		if len(rest) == 0 {
			continue
		}

		if rest[0] == '"' || rest[0] == '\'' {
			quote := rest[0]
			rest = rest[1:]
			end := strings.IndexByte(rest, quote)
			if end != -1 {
				val := strings.TrimSpace(rest[:end])
				if val != "" {
					return val
				}
			}
			continue
		}

		end := strings.IndexAny(rest, " \t\r\n,;{}]")
		var val string
		if end != -1 {
			val = strings.TrimSpace(rest[:end])
		} else {
			val = strings.TrimSpace(rest)
		}
		if val != "" {
			return val
		}
	}
	return ""
}
