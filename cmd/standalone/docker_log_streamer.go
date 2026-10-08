package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type dockerLogClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

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
	dockerClient dockerLogClient,
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
					entries := []domainLogging.LogEntry{{
						Timestamp: entryTS, TenantID: "default-tenant", ClusterID: "default", Namespace: "docker",
						PodName: cleanName, ContainerName: cleanName, Stream: stream,
						LogLevel: domainLogging.LogLevel(strings.ToLower(lvl)), Message: msg,
						TraceID:  traceID,
						Attributes: map[string]string{"service": cleanName},
					}}
					if err := centralizedLogs.Ingest(ctx, entries); err != nil {
						logger.Get().Debug("log ingestion warning", zap.Error(err))
					}
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
	dockerClient dockerLogClient,
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

		// 2. Dynamically watch Docker container events for "start" and "restart" with backoff
		f := filters.NewArgs()
		f.Add("type", "container")
		f.Add("event", "start")
		f.Add("event", "restart")

		bo := backoff.NewExponentialBackOff()
		bo.InitialInterval = 200 * time.Millisecond
		bo.MaxInterval = 10 * time.Second

		_, _ = backoff.Retry(ctx, func() (struct{}, error) {
			msgCh, errCh := dockerClient.Events(ctx, events.ListOptions{Filters: f})
			if msgCh == nil || errCh == nil {
				return struct{}{}, errors.New("docker events returned nil channels")
			}
			for {
				select {
				case <-ctx.Done():
					return struct{}{}, backoff.Permanent(ctx.Err())
				case err, ok := <-errCh:
					if !ok {
						return struct{}{}, errors.New("docker events error channel closed")
					}
					if err != nil {
						return struct{}{}, err
					}
				case msg, ok := <-msgCh:
					if !ok {
						return struct{}{}, errors.New("docker events message channel closed")
					}
					bo.Reset()
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
		}, backoff.WithBackOff(bo), backoff.WithMaxElapsedTime(0), backoff.WithNotify(func(err error, d time.Duration) {
			if ctx.Err() == nil {
				log.Warn("Docker log streamer event stream error, re-subscribing", zap.Error(err), zap.Duration("backoff", d))
			}
		}))
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
