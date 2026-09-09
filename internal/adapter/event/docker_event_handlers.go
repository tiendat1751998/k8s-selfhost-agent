package event

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

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

