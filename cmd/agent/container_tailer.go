package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

const maxSeenAtLastTS = 10000

// ContainerTailer continuously discovers Docker containers and streams their logs into logengine.Writer.
type ContainerTailer struct {
	cli              DockerClientInterface
	writer           *logengine.Writer
	logger           *slog.Logger
	pollInterval     time.Duration
	eofRetryInterval time.Duration
	tailLines        string
	newBackOffFn     func() backoff.BackOff
	sleepFn          func(ctx context.Context, d time.Duration) error
	mu               sync.Mutex
	activeStreams    map[string]context.CancelFunc
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
		eofRetryInterval: 200 * time.Millisecond,
		tailLines:        "100",
		activeStreams:    make(map[string]context.CancelFunc),
	}
}

func (t *ContainerTailer) newBackOff() backoff.BackOff {
	if t.newBackOffFn != nil {
		return t.newBackOffFn()
	}
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	bo.MaxInterval = 5 * time.Second
	return bo
}

func (t *ContainerTailer) sleep(ctx context.Context, d time.Duration) error {
	if t.sleepFn != nil {
		return t.sleepFn(ctx, d)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
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
		if _, exists := t.activeStreams[cID]; !exists {
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
		t.mu.Unlock()
	}()

	var lastTimestamp time.Time
	seenAtLastTS := make(map[string]bool)
	bo := t.newBackOff()

	for ctx.Err() == nil {
		opts := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
		}
		if lastTimestamp.IsZero() {
			tail := t.tailLines
			if tail == "" || tail == "all" {
				tail = "100"
			}
			opts.Tail = tail
		} else {
			opts.Since = lastTimestamp.Format(time.RFC3339Nano)
		}

		reader, err := t.cli.ContainerLogs(ctx, containerID, opts)
		if err != nil {
			delay := bo.NextBackOff()
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
			if sleepErr := t.sleep(ctx, delay); sleepErr != nil {
				return
			}
			continue
		}

		t.readContainerLogStream(ctx, reader, serviceName, &lastTimestamp, seenAtLastTS)
		if closeErr := reader.Close(); closeErr != nil {
			t.logger.Debug("error closing container logs reader", slog.String("error", closeErr.Error()))
		}
		bo.Reset()

		// Robust EOF recovery with non-busy polling: when log stream reaches EOF (e.g. rotation),
		// sleep briefly before reconnecting with Since: lastTimestamp.
		if sleepErr := t.sleep(ctx, t.eofRetryInterval); sleepErr != nil {
			return
		}
	}
}

func (t *ContainerTailer) readContainerLogStream(
	ctx context.Context,
	reader io.ReadCloser,
	serviceName string,
	lastTimestamp *time.Time,
	seenAtLastTS map[string]bool,
) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if closeErr := reader.Close(); closeErr != nil {
				t.logger.Debug("error closing container logs reader on cancel", slog.String("error", closeErr.Error()))
			}
		case <-done:
		}
	}()
	defer close(done)

	bufReader := bufio.NewReader(reader)
	header, err := bufReader.Peek(8)
	isStdCopy := err == nil && len(header) == 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0

	var overflowLogged bool
	var lineMu sync.Mutex
	processLine := func(line string) {
		entry := parseLogLine(line, serviceName)

		lineMu.Lock()
		defer lineMu.Unlock()

		if !lastTimestamp.IsZero() {
			if entry.Timestamp.Before(*lastTimestamp) {
				return
			}
			if entry.Timestamp.Equal(*lastTimestamp) && seenAtLastTS[entry.Raw] {
				return
			}
		}
		if entry.Timestamp.After(*lastTimestamp) {
			*lastTimestamp = entry.Timestamp
			clear(seenAtLastTS)
			overflowLogged = false
		}
		if len(seenAtLastTS) >= maxSeenAtLastTS {
			if !overflowLogged {
				t.logger.Warn("seenAtLastTS deduplication map reached capacity, resetting map",
					slog.String("service", serviceName),
					slog.Int("cap", maxSeenAtLastTS))
				overflowLogged = true
			}
			clear(seenAtLastTS)
		}
		seenAtLastTS[entry.Raw] = true

		if t.writer != nil {
			if writeErr := t.writer.Write(logengine.Entry{
				Timestamp: entry.Timestamp,
				Service:   entry.Service,
				Level:     entry.Level,
				Message:   entry.Message,
				Raw:       entry.Raw,
			}); writeErr != nil {
				t.logger.Debug("failed to write log entry", slog.String("error", writeErr.Error()))
			}
		}
	}

	if isStdCopy {
		stdoutReader, stdoutWriter := io.Pipe()
		stderrReader, stderrWriter := io.Pipe()

		var scanWg sync.WaitGroup
		scanStream := func(r io.Reader) {
			defer scanWg.Done()
			scanner := bufio.NewScanner(r)
			scanner.Buffer(make([]byte, 4096), 64*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				processLine(line)
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
			processLine(line)
		}
	}
}
