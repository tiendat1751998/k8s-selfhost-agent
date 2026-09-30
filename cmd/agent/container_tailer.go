package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// ContainerTailer continuously discovers Docker containers and streams their logs into logengine.Writer.
type ContainerTailer struct {
	cli              DockerClientInterface
	writer           *logengine.Writer
	logger           *slog.Logger
	pollInterval     time.Duration
	tailLines        string
	mu               sync.Mutex
	activeStreams    map[string]context.CancelFunc
	completedStreams map[string]bool
	wg               sync.WaitGroup
	stopped          bool
}

// NewContainerTailer creates a new ContainerTailer.
func NewContainerTailer(cli DockerClientInterface, writer *logengine.Writer, logger *slog.Logger) *ContainerTailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContainerTailer{
		cli:              cli,
		writer:           writer,
		logger:           logger,
		pollInterval:     5 * time.Second,
		tailLines:        "100",
		activeStreams:    make(map[string]context.CancelFunc),
		completedStreams: make(map[string]bool),
	}
}

// Start begins the discovery and streaming loop until ctx is cancelled.
func (t *ContainerTailer) Start(ctx context.Context) {
	if t.cli == nil || t.writer == nil {
		return
	}

	t.logger.Info("Starting continuous container log tailer")
	t.pollOnce(ctx)

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-ticker.C:
			t.pollOnce(ctx)
		}
	}
}

// Stop terminates all active container log streams and waits for goroutines to exit cleanly.
func (t *ContainerTailer) Stop() {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.stopped = true
	for id, cancel := range t.activeStreams {
		cancel()
		delete(t.activeStreams, id)
	}
	t.mu.Unlock()

	t.wg.Wait()
	t.logger.Info("Container log tailer stopped")
}

func (t *ContainerTailer) pollOnce(ctx context.Context) {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	containers, err := t.cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Debug("ContainerList poll encountered warning", slog.String("error", err.Error()))
		}
		return
	}

	activeNow := make(map[string]bool, len(containers))
	for _, c := range containers {
		cID := c.ID
		activeNow[cID] = true

		serviceName := extractContainerServiceName(c)

		t.mu.Lock()
		if t.stopped {
			t.mu.Unlock()
			return
		}
		if _, exists := t.activeStreams[cID]; !exists && !t.completedStreams[cID] {
			streamCtx, cancel := context.WithCancel(ctx)
			t.activeStreams[cID] = cancel
			t.wg.Add(1)
			go t.streamContainerLogs(streamCtx, cID, serviceName)
		}
		t.mu.Unlock()
	}

	// Prune stopped containers
	t.mu.Lock()
	for id, cancel := range t.activeStreams {
		if !activeNow[id] {
			cancel()
			delete(t.activeStreams, id)
		}
	}
	for id := range t.completedStreams {
		if !activeNow[id] {
			delete(t.completedStreams, id)
		}
	}
	t.mu.Unlock()
}

func extractContainerServiceName(c types.Container) string {
	if swarmSvc, ok := c.Labels["com.docker.swarm.service.name"]; ok && swarmSvc != "" {
		return swarmSvc
	}
	if composeSvc, ok := c.Labels["com.docker.compose.service"]; ok && composeSvc != "" {
		return composeSvc
	}
	if len(c.Names) > 0 {
		clean := strings.TrimPrefix(c.Names[0], "/")
		if clean != "" {
			return clean
		}
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

func (t *ContainerTailer) streamContainerLogs(ctx context.Context, containerID, serviceName string) {
	defer t.wg.Done()
	defer func() {
		t.mu.Lock()
		delete(t.activeStreams, containerID)
		t.completedStreams[containerID] = true
		t.mu.Unlock()
	}()

	reader, err := t.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       t.tailLines,
	})
	if err != nil {
		return
	}
	defer reader.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = reader.Close()
		case <-done:
		}
	}()
	defer close(done)

	bufReader := bufio.NewReader(reader)
	header, err := bufReader.Peek(8)
	isStdCopy := err == nil && len(header) == 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0

	if isStdCopy {
		stdoutReader, stdoutWriter := io.Pipe()
		stderrReader, stderrWriter := io.Pipe()

		var scanWg sync.WaitGroup
		scanStream := func(r io.Reader) {
			defer scanWg.Done()
			scanner := bufio.NewScanner(r)
			// Strictly bound line buffer < 64KB
			scanner.Buffer(make([]byte, 4096), 64*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				entry := parseLogLine(line, serviceName)
				if t.writer != nil {
					_ = t.writer.Write(logengine.Entry{
						Timestamp: entry.Timestamp,
						Service:   entry.Service,
						Level:     entry.Level,
						Message:   entry.Message,
						Raw:       entry.Raw,
					})
				}
			}
		}

		scanWg.Add(2)
		go scanStream(stdoutReader)
		go scanStream(stderrReader)

		_, _ = stdcopy.StdCopy(stdoutWriter, stderrWriter, bufReader)
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		scanWg.Wait()
	} else {
		scanner := bufio.NewScanner(bufReader)
		scanner.Buffer(make([]byte, 4096), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			entry := parseLogLine(line, serviceName)
			if t.writer != nil {
				_ = t.writer.Write(logengine.Entry{
					Timestamp: entry.Timestamp,
					Service:   entry.Service,
					Level:     entry.Level,
					Message:   entry.Message,
					Raw:       entry.Raw,
				})
			}
		}
	}
}
