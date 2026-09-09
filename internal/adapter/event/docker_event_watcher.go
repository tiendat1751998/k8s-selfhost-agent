// Package event provides event watchers for Kubernetes and Docker incident detection.
package event

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

// DockerEventAPI defines the subset of Docker API required for event watching and incident enrichment.
type DockerEventAPI interface {
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
}

// CrashTracker tracks crashes per service within a sliding time window.
type CrashTracker struct {
	mu      sync.Mutex
	crashes map[string][]time.Time
	window  time.Duration
}

// NewCrashTracker creates a new CrashTracker with the specified sliding window duration.
func NewCrashTracker(window time.Duration) *CrashTracker {
	return &CrashTracker{
		crashes: make(map[string][]time.Time),
		window:  window,
	}
}

// RecordCrash registers a crash timestamp for a service key and returns the total crashes in the current window.
func (ct *CrashTracker) RecordCrash(serviceKey string, t time.Time) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	cutoff := t.Add(-ct.window)
	existing := ct.crashes[serviceKey]
	var active []time.Time
	for _, ts := range existing {
		if ts.After(cutoff) {
			active = append(active, ts)
		}
	}
	active = append(active, t)
	ct.crashes[serviceKey] = active
	return len(active)
}

// Reset clears the recorded crashes for a service key.
func (ct *CrashTracker) Reset(serviceKey string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.crashes, serviceKey)
}

// GetCount returns the number of crashes for a service key in the current window.
func (ct *CrashTracker) GetCount(serviceKey string, now time.Time) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	cutoff := now.Add(-ct.window)
	count := 0
	for _, ts := range ct.crashes[serviceKey] {
		if ts.After(cutoff) {
			count++
		}
	}
	return count
}

// DockerEventWatcher subscribes to real Docker daemon events, creates incidents, enriches them with logs, and auto-resolves.
type DockerEventWatcher struct {
	dockerClient     DockerEventAPI
	incRepo          incident.Repository
	broadcaster      DockerBroadcaster
	handler          IncidentHandler
	logger           *zap.Logger
	clusterName      string
	defaultNamespace string
	crashTracker     *CrashTracker
	crashThreshold   int
	logTailLines     int
	mu               sync.RWMutex
	running          bool
}

// DockerWatcherOption configures a DockerEventWatcher instance.
type DockerWatcherOption func(*DockerEventWatcher)

// WithDockerClusterName sets the cluster name on created incidents.
func WithDockerClusterName(name string) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		if name != "" {
			w.clusterName = name
		}
	}
}

// WithDockerDefaultNamespace sets the default namespace when container labels lack one.
func WithDockerDefaultNamespace(ns string) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		if ns != "" {
			w.defaultNamespace = ns
		}
	}
}

// WithDockerIncidentHandler sets an optional callback handler for detected incidents.
func WithDockerIncidentHandler(h IncidentHandler) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		w.handler = h
	}
}

// WithDockerCrashWindow sets the sliding window duration for crash loop detection.
func WithDockerCrashWindow(window time.Duration) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		if window > 0 {
			w.crashTracker = NewCrashTracker(window)
		}
	}
}

// WithDockerCrashThreshold sets the number of crashes within the window needed to trigger CrashLoopBackOff.
func WithDockerCrashThreshold(threshold int) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		if threshold > 0 {
			w.crashThreshold = threshold
		}
	}
}

// WithDockerLogTailLines sets the number of log lines to extract from dead containers.
func WithDockerLogTailLines(lines int) DockerWatcherOption {
	return func(w *DockerEventWatcher) {
		if lines > 0 {
			w.logTailLines = lines
		}
	}
}

// NewDockerEventWatcher creates a new DockerEventWatcher.
func NewDockerEventWatcher(
	dockerClient DockerEventAPI,
	incRepo incident.Repository,
	broadcaster DockerBroadcaster,
	logger *zap.Logger,
	opts ...DockerWatcherOption,
) *DockerEventWatcher {
	if logger == nil {
		logger = zap.NewNop()
	}

	w := &DockerEventWatcher{
		dockerClient:     dockerClient,
		incRepo:          incRepo,
		broadcaster:      broadcaster,
		logger:           logger.Named("docker-event-watcher"),
		clusterName:      "fleet-primary",
		defaultNamespace: "default",
		crashTracker:     NewCrashTracker(5 * time.Minute),
		crashThreshold:   3,
		logTailLines:     50,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Start begins listening to the Docker event stream. It blocks until ctx is cancelled.
func (w *DockerEventWatcher) Start(ctx context.Context) error {
	if w.dockerClient == nil {
		w.logger.Warn("Docker client is nil, DockerEventWatcher will not start")
		return nil
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("docker event watcher is already running")
	}
	w.running = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	w.logger.Info("Starting real Docker event-driven incident watcher",
		zap.String("cluster", w.clusterName),
		zap.Int("crash_threshold", w.crashThreshold),
		zap.Int("log_tail_lines", w.logTailLines),
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Docker event watcher stopped")
			return nil
		default:
		}

		if err := w.watchEvents(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("Docker event watch stream error, reconnecting in 5s", zap.Error(err))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (w *DockerEventWatcher) watchEvents(ctx context.Context) error {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("event", "die")
	f.Add("event", "oom")
	f.Add("event", "health_status")
	f.Add("event", "start")

	msgCh, errCh := w.dockerClient.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errCh:
			if !ok {
				return fmt.Errorf("docker events error channel closed")
			}
			if err != nil && err != io.EOF && ctx.Err() == nil {
				return fmt.Errorf("docker events stream error: %w", err)
			}
			return nil
		case msg, ok := <-msgCh:
			if !ok {
				return fmt.Errorf("docker events channel closed")
			}
			w.ProcessEvent(ctx, msg)
		}
	}
}

// ProcessEvent handles a single Docker event message.
func (w *DockerEventWatcher) ProcessEvent(ctx context.Context, msg events.Message) {
	if string(msg.Type) != "container" && msg.Type != "" {
		return
	}

	action := string(msg.Action)
	switch {
	case action == "oom" || strings.HasPrefix(action, "oom"):
		w.handleOOMEvent(ctx, msg)

	case action == "die" || strings.HasPrefix(action, "die"):
		w.handleDieEvent(ctx, msg)

	case action == "health_status: unhealthy" || (strings.HasPrefix(action, "health_status") && msg.Actor.Attributes["health_status"] == "unhealthy"):
		w.handleHealthStatusEvent(ctx, msg)

	case action == "start" || strings.HasPrefix(action, "start"):
		w.handleStartEvent(ctx, msg)
	}
}

