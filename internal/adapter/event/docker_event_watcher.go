// Package event provides event watchers for Kubernetes and Docker incident detection.
package event

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

// DockerBroadcaster sends messages to real-time subscribers (e.g. WebSocket clients).
type DockerBroadcaster interface {
	Broadcast(msgType string, data interface{})
}

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

func (w *DockerEventWatcher) handleOOMEvent(ctx context.Context, msg events.Message) {
	containerID := msg.Actor.ID
	containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	serviceName := ExtractServiceName(msg.Actor.Attributes, containerName)
	namespace := ExtractNamespace(msg.Actor.Attributes, w.defaultNamespace)

	exitCode := msg.Actor.Attributes["exitCode"]
	if exitCode == "" {
		exitCode = "137"
	}

	w.logger.Warn("Docker container OOMKilled detected",
		zap.String("service", serviceName),
		zap.String("container", containerName),
		zap.String("container_id", containerID),
		zap.String("exit_code", exitCode),
	)

	logs := w.fetchLogs(ctx, containerID)

	message := fmt.Sprintf("Container '%s' (service: '%s') was OOMKilled (exit code: %s). Memory limit exceeded.", containerName, serviceName, exitCode)
	inc, err := incident.New(w.clusterName, namespace, serviceName, incident.TypeOOMKilled, incident.SeverityCritical, message)
	if err != nil {
		w.logger.Error("Failed to instantiate OOMKilled incident", zap.Error(err))
		return
	}

	w.enrichIncident(inc, msg, containerID, containerName, serviceName, exitCode, "OOMKilled", logs)

	if err := w.persistAndBroadcast(ctx, inc); err != nil {
		w.logger.Error("Failed to persist OOMKilled incident", zap.Error(err))
	}
}

func (w *DockerEventWatcher) handleDieEvent(ctx context.Context, msg events.Message) {
	exitCodeStr := msg.Actor.Attributes["exitCode"]
	exitCode, err := strconv.Atoi(exitCodeStr)
	// Normal exit (exit 0) is not a crash
	if err == nil && exitCode == 0 {
		return
	}

	containerID := msg.Actor.ID
	containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	serviceName := ExtractServiceName(msg.Actor.Attributes, containerName)
	namespace := ExtractNamespace(msg.Actor.Attributes, w.defaultNamespace)

	serviceKey := namespace + "/" + serviceName
	crashCount := w.crashTracker.RecordCrash(serviceKey, time.Now().UTC())

	w.logger.Info("Docker container crashed",
		zap.String("service", serviceName),
		zap.String("container", containerName),
		zap.String("exit_code", exitCodeStr),
		zap.Int("crash_streak_5m", crashCount),
	)

	if crashCount < w.crashThreshold {
		return
	}

	w.logger.Warn("CrashLoopBackOff threshold exceeded for service",
		zap.String("service", serviceName),
		zap.Int("crashes", crashCount),
		zap.Int("threshold", w.crashThreshold),
	)

	logs := w.fetchLogs(ctx, containerID)

	message := fmt.Sprintf("Service '%s' is in CrashLoopBackOff: %d crashes within 5m window (container: '%s', exit code: %s)", serviceName, crashCount, containerName, exitCodeStr)
	inc, err := incident.New(w.clusterName, namespace, serviceName, incident.TypeCrashLoopBackOff, incident.SeverityHigh, message)
	if err != nil {
		w.logger.Error("Failed to instantiate CrashLoopBackOff incident", zap.Error(err))
		return
	}

	w.enrichIncident(inc, msg, containerID, containerName, serviceName, exitCodeStr, "CrashLoopBackOff", logs)
	inc.AddRawData("crash_count", strconv.Itoa(crashCount))

	if err := w.persistAndBroadcast(ctx, inc); err != nil {
		w.logger.Error("Failed to persist CrashLoopBackOff incident", zap.Error(err))
	}
}

func (w *DockerEventWatcher) handleHealthStatusEvent(ctx context.Context, msg events.Message) {
	containerID := msg.Actor.ID
	containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	serviceName := ExtractServiceName(msg.Actor.Attributes, containerName)
	namespace := ExtractNamespace(msg.Actor.Attributes, w.defaultNamespace)

	w.logger.Warn("Docker container health check failed (unhealthy)",
		zap.String("service", serviceName),
		zap.String("container", containerName),
		zap.String("container_id", containerID),
	)

	logs := w.fetchLogs(ctx, containerID)

	message := fmt.Sprintf("Service '%s' container '%s' failed health check (unhealthy)", serviceName, containerName)
	inc, err := incident.New(w.clusterName, namespace, serviceName, incident.TypeServiceUnhealthy, incident.SeverityHigh, message)
	if err != nil {
		w.logger.Error("Failed to instantiate ServiceUnhealthy incident", zap.Error(err))
		return
	}

	w.enrichIncident(inc, msg, containerID, containerName, serviceName, "", "HealthCheckFailed", logs)
	inc.AddRawData("health_status", "unhealthy")

	if err := w.persistAndBroadcast(ctx, inc); err != nil {
		w.logger.Error("Failed to persist ServiceUnhealthy incident", zap.Error(err))
	}
}

func (w *DockerEventWatcher) handleStartEvent(ctx context.Context, msg events.Message) {
	containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	serviceName := ExtractServiceName(msg.Actor.Attributes, containerName)
	namespace := ExtractNamespace(msg.Actor.Attributes, w.defaultNamespace)

	recovered, err := isServiceRecovered(ctx, w.dockerClient, serviceName)
	if err != nil {
		w.logger.Debug("Error checking service recovery", zap.String("service", serviceName), zap.Error(err))
		return
	}

	if !recovered {
		return
	}

	w.logger.Info("Service recovered and healthy, auto-resolving open incidents",
		zap.String("service", serviceName),
		zap.String("namespace", namespace),
	)

	if err := w.autoResolveService(ctx, namespace, serviceName); err != nil {
		w.logger.Error("Error auto-resolving incidents for service", zap.String("service", serviceName), zap.Error(err))
	}
}

func (w *DockerEventWatcher) autoResolveService(ctx context.Context, namespace, serviceName string) error {
	if w.incRepo == nil {
		return nil
	}

	typesToCheck := []incident.Type{
		incident.TypeCrashLoopBackOff,
		incident.TypeOOMKilled,
		incident.TypeServiceUnhealthy,
	}

	for _, incType := range typesToCheck {
		activeInc, err := w.incRepo.GetByPodAndType(ctx, namespace, serviceName, incType)
		if err != nil {
			w.logger.Warn("Failed to check active incident for auto-resolve",
				zap.String("service", serviceName),
				zap.String("type", string(incType)),
				zap.Error(err),
			)
			continue
		}

		if activeInc != nil && activeInc.Status != incident.StatusResolved && activeInc.Status != incident.StatusFailed {
			if err := activeInc.MarkResolved(); err != nil {
				w.logger.Warn("Failed to mark incident resolved", zap.String("id", activeInc.ID), zap.Error(err))
				continue
			}

			if err := w.incRepo.Update(ctx, activeInc); err != nil {
				w.logger.Error("Failed to update resolved incident in repo", zap.String("id", activeInc.ID), zap.Error(err))
				continue
			}

			w.logger.Info("Auto-resolved incident successfully",
				zap.String("incident_id", activeInc.ID),
				zap.String("service", serviceName),
				zap.String("type", string(incType)),
			)

			if w.broadcaster != nil {
				w.broadcaster.Broadcast("incident_resolved", activeInc)
			}
		}
	}

	// Reset in-memory crash history
	serviceKey := namespace + "/" + serviceName
	w.crashTracker.Reset(serviceKey)
	return nil
}

func (w *DockerEventWatcher) enrichIncident(
	inc *incident.Incident,
	msg events.Message,
	containerID string,
	containerName string,
	serviceName string,
	exitCode string,
	reason string,
	logs string,
) {
	inc.AddRawData("container_id", containerID)
	inc.AddRawData("container_name", containerName)
	inc.AddRawData("service_name", serviceName)
	if img := msg.Actor.Attributes["image"]; img != "" {
		inc.AddRawData("image", img)
	}
	if exitCode != "" {
		inc.AddRawData("exit_code", exitCode)
	}
	if reason != "" {
		inc.AddRawData("reason", reason)
	}
	if logs != "" {
		inc.AddRawData("logs", logs)
	}
	if msg.Time > 0 {
		inc.AddRawData("event_time", time.Unix(msg.Time, msg.TimeNano).Format(time.RFC3339))
	}

	// Enrich with swarm/compose metadata from labels
	for k, v := range msg.Actor.Attributes {
		if strings.HasPrefix(k, "com.docker.") || k == "app" || k == "service" || k == "node_id" {
			inc.AddRawData(k, v)
		}
	}
}

func (w *DockerEventWatcher) fetchLogs(ctx context.Context, containerID string) string {
	if w.dockerClient == nil || containerID == "" || w.logTailLines <= 0 {
		return ""
	}

	logCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	reader, err := w.dockerClient.ContainerLogs(logCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(w.logTailLines),
	})
	if err != nil {
		return fmt.Sprintf("[Log fetch error: %v]", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Sprintf("[Log read error: %v]", err)
	}
	if len(data) == 0 {
		return ""
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	_, copyErr := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, bytes.NewReader(data))
	if copyErr == nil && (stdoutBuf.Len() > 0 || stderrBuf.Len() > 0) {
		out := stdoutBuf.String()
		if stderrBuf.Len() > 0 {
			if out != "" {
				out += "\n--- stderr ---\n" + stderrBuf.String()
			} else {
				out = stderrBuf.String()
			}
		}
		return strings.TrimSpace(out)
	}

	// Fallback for TTY or non-multiplexed logs
	return strings.TrimSpace(string(data))
}

func (w *DockerEventWatcher) persistAndBroadcast(ctx context.Context, inc *incident.Incident) error {
	if w.incRepo != nil {
		// Deduplication check: do not recreate if active incident exists
		existing, err := w.incRepo.GetByPodAndType(ctx, inc.Namespace, inc.PodName, inc.Type)
		if err != nil {
			w.logger.Warn("Failed to check for existing incident", zap.Error(err))
		}
		if existing != nil && existing.Status != incident.StatusResolved && existing.Status != incident.StatusFailed {
			w.logger.Debug("Active incident already exists, deduplicating",
				zap.String("service", inc.PodName),
				zap.String("type", string(inc.Type)),
				zap.String("existing_id", existing.ID),
			)
			return nil
		}

		if err := w.incRepo.Create(ctx, inc); err != nil {
			return fmt.Errorf("creating incident in repository: %w", err)
		}
	}

	if w.broadcaster != nil {
		w.broadcaster.Broadcast("incident", inc)
	}

	if w.handler != nil {
		if err := w.handler(ctx, inc); err != nil {
			w.logger.Warn("Incident handler callback returned error", zap.Error(err))
		}
	}

	return nil
}

// ExtractServiceName extracts the canonical service name from Docker labels or container name.
func ExtractServiceName(labels map[string]string, containerName string) string {
	if labels != nil {
		if s, ok := labels["com.docker.swarm.service.name"]; ok && s != "" {
			return s
		}
		if s, ok := labels["com.docker.compose.service"]; ok && s != "" {
			return s
		}
		if s, ok := labels["app"]; ok && s != "" {
			return s
		}
		if s, ok := labels["service"]; ok && s != "" {
			return s
		}
	}

	name := strings.TrimPrefix(containerName, "/")
	if name != "" {
		return name
	}
	return "unknown"
}

// ExtractNamespace extracts the stack/project namespace from Docker labels or returns defaultNS.
func ExtractNamespace(labels map[string]string, defaultNS string) string {
	if labels != nil {
		if ns, ok := labels["com.docker.stack.namespace"]; ok && ns != "" {
			return ns
		}
		if ns, ok := labels["com.docker.compose.project"]; ok && ns != "" {
			return ns
		}
		if ns, ok := labels["namespace"]; ok && ns != "" {
			return ns
		}
	}
	if defaultNS != "" {
		return defaultNS
	}
	return "default"
}

func isServiceRecovered(ctx context.Context, cli DockerEventAPI, serviceName string) (bool, error) {
	if cli == nil || serviceName == "" {
		return false, nil
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return false, fmt.Errorf("listing containers for recovery check: %w", err)
	}

	var matchingContainers []container.Summary
	for _, c := range containers {
		cName := ""
		if len(c.Names) > 0 {
			cName = strings.TrimPrefix(c.Names[0], "/")
		}
		svc := ExtractServiceName(c.Labels, cName)
		if svc == serviceName || cName == serviceName {
			matchingContainers = append(matchingContainers, c)
		}
	}

	if len(matchingContainers) == 0 {
		return false, nil
	}

	for _, c := range matchingContainers {
		if c.State != "running" {
			return false, nil
		}
		if strings.Contains(strings.ToLower(c.Status), "unhealthy") {
			return false, nil
		}
	}

	return true, nil
}
