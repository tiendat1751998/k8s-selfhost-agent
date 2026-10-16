package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// DetectLogAnomaly inspects a raw log line for critical Kubernetes runtime/kernel signatures.
func DetectLogAnomaly(line string) (code string, matched bool) {
	if strings.Contains(line, "Out of memory: Killed process") {
		return "OOM_KILL", true
	}
	if strings.Contains(line, "PLEG is not healthy") {
		return "PLEG_UNHEALTHY", true
	}
	if strings.Contains(line, "nf_conntrack: table full") {
		return "CONNTRACK_FULL", true
	}
	if strings.Contains(line, "FailedCreatePodSandBox") {
		return "CNI_SANDBOX_FAILURE", true
	}
	return "", false
}

// SelfHealingWatchdog monitors local node health and triggers allowlisted remediation autonomously.
type SelfHealingWatchdog struct {
	checker                *K8sHealthChecker
	remediator             *NodeRemediator
	interval               time.Duration
	diskCooldown           time.Duration
	lastDiskClean          time.Time
	lastZombieReap         time.Time
	lastCNICleanup         time.Time
	pendingAnomalies       map[string]time.Time
	consecutiveKubeletFail int
	consecutiveCRIFail     int
	logWriter              *logengine.Writer
	nowFn                  func() time.Time
	mu                     sync.Mutex
}

// WatchdogOption configures SelfHealingWatchdog.
type WatchdogOption func(*SelfHealingWatchdog)

// WithWatchdogInterval sets the periodic check interval.
func WithWatchdogInterval(d time.Duration) WatchdogOption {
	return func(w *SelfHealingWatchdog) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithWatchdogLogWriter attaches an optional columnar logengine writer.
func WithWatchdogLogWriter(writer *logengine.Writer) WatchdogOption {
	return func(w *SelfHealingWatchdog) {
		w.logWriter = writer
	}
}

// WithWatchdogNowFunc injects a custom clock for deterministic testing.
func WithWatchdogNowFunc(fn func() time.Time) WatchdogOption {
	return func(w *SelfHealingWatchdog) {
		if fn != nil {
			w.nowFn = fn
		}
	}
}

// NewSelfHealingWatchdog constructs an autonomous watchdog.
func NewSelfHealingWatchdog(checker *K8sHealthChecker, remediator *NodeRemediator, opts ...WatchdogOption) *SelfHealingWatchdog {
	w := &SelfHealingWatchdog{
		checker:          checker,
		remediator:       remediator,
		interval:         30 * time.Second,
		diskCooldown:     15 * time.Minute,
		pendingAnomalies: make(map[string]time.Time),
		nowFn:            time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *SelfHealingWatchdog) now() time.Time {
	if w.nowFn != nil {
		return w.nowFn()
	}
	return time.Now()
}

// RecordLogLine inspects a log line for known anomalies and registers them with timestamps.
func (w *SelfHealingWatchdog) RecordLogLine(line string) {
	code, matched := DetectLogAnomaly(line)
	if !matched || code == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pendingAnomalies == nil {
		w.pendingAnomalies = make(map[string]time.Time)
	}
	w.pendingAnomalies[code] = w.now()
}

// HookPodTailer connects watchdog anomaly detection to live PodTailer log ingestion.
func (w *SelfHealingWatchdog) HookPodTailer(pt *PodTailer) {
	if w == nil || pt == nil {
		return
	}
	originalWriter := pt.writer
	pt.entryWriter = func(entry logengine.Entry) error {
		w.RecordLogLine(entry.Message)
		if entry.Raw != "" && entry.Raw != entry.Message {
			w.RecordLogLine(entry.Raw)
		}
		if originalWriter != nil {
			return originalWriter.Write(entry)
		}
		return nil
	}
}

// Start begins the background monitoring loop.
func (w *SelfHealingWatchdog) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce performs a single autonomous self-healing evaluation pass.
func (w *SelfHealingWatchdog) runOnce(ctx context.Context) {
	if w.checker == nil || w.remediator == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	health := w.checker.CheckHealth(ctx)
	now := w.now()

	// 1. Autonomous Disk & Inode Pressure Relief
	if health.DiskPressure || health.InodePressure {
		if now.Sub(w.lastDiskClean) >= w.diskCooldown {
			w.lastDiskClean = now
			slog.Warn("[SELF-HEAL] Disk/Inode pressure detected, executing cleanup")
			res, err := w.remediator.Execute(ctx, RemediationRequest{
				Action: "disk_pressure_cleanup",
				Reason: "Autonomous watchdog triggered by disk/inode pressure",
			})
			if err != nil {
				slog.Error("[SELF-HEAL] Disk pressure cleanup failed", slog.String("error", err.Error()))
			} else {
				msg := fmt.Sprintf("[SELF-HEAL] Disk pressure relief complete: %d bytes freed, %d inodes freed", res.BytesFreed, res.InodesFreed)
				slog.Info(msg)
				w.writeLogEntry("warn", msg)
			}
		}
	}

	// 2. Autonomous Kubelet Recovery
	if !health.KubeletHealthy {
		w.consecutiveKubeletFail++
		if w.consecutiveKubeletFail >= 3 {
			w.consecutiveKubeletFail = 0
			slog.Warn("[SELF-HEAL] Kubelet unresponsive for 3 consecutive checks, restarting kubelet")
			res, err := w.remediator.Execute(ctx, RemediationRequest{
				Action: "restart_service",
				Target: "kubelet",
				Reason: "Autonomous watchdog triggered: Kubelet failed 3 consecutive health checks",
			})
			if err != nil {
				slog.Error("[SELF-HEAL] Restarting kubelet failed", slog.String("error", err.Error()))
			} else {
				msg := fmt.Sprintf("[SELF-HEAL] Successfully restarted kubelet: %s", res.Details)
				slog.Info(msg)
				w.writeLogEntry("warn", msg)
			}
		}
	} else {
		w.consecutiveKubeletFail = 0
	}

	// 3. Autonomous CRI Socket Recovery
	if !health.ContainerdHealthy && (health.CRISocketPath == "/run/containerd/containerd.sock" || (!health.DockerHealthy && health.CRISocketPath == "")) {
		w.consecutiveCRIFail++
		if w.consecutiveCRIFail >= 3 {
			w.consecutiveCRIFail = 0
			slog.Warn("[SELF-HEAL] Containerd socket down for 3 consecutive checks, restarting containerd")
			res, err := w.remediator.Execute(ctx, RemediationRequest{
				Action: "restart_service",
				Target: "containerd",
				Reason: "Autonomous watchdog triggered: Containerd socket down for 3 consecutive checks",
			})
			if err != nil {
				slog.Error("[SELF-HEAL] Restarting containerd failed", slog.String("error", err.Error()))
			} else {
				msg := fmt.Sprintf("[SELF-HEAL] Successfully restarted containerd: %s", res.Details)
				slog.Info(msg)
				w.writeLogEntry("warn", msg)
			}
		}
	} else if health.ContainerdHealthy {
		w.consecutiveCRIFail = 0
	}

	// 4. Rule 3: Autonomous Zombie Process Reap
	if health.ZombieCount > 0 && now.Sub(w.lastZombieReap) >= 10*time.Minute {
		w.lastZombieReap = now
		slog.Warn("[SELF-HEAL] Zombie processes detected, executing reap", slog.Int("zombies", health.ZombieCount))
		res, err := w.remediator.Execute(ctx, RemediationRequest{
			Action: "reap_zombies",
			Reason: "autonomous_watchdog: zombie processes detected",
		})
		if err != nil {
			slog.Error("[SELF-HEAL] Zombie reaping failed", slog.String("error", err.Error()))
		} else {
			msg := fmt.Sprintf("[SELF-HEAL] Zombie reap complete: %s", res.Details)
			slog.Info(msg)
			w.writeLogEntry("warn", msg)
		}
	}

	// 5. Rule 4: Autonomous CNI IPAM Cleanup on CNI_SANDBOX_FAILURE anomaly
	if recordedAt, exists := w.pendingAnomalies["CNI_SANDBOX_FAILURE"]; exists {
		if now.Sub(recordedAt) <= 5*time.Minute && now.Sub(w.lastCNICleanup) >= 10*time.Minute {
			w.lastCNICleanup = now
			delete(w.pendingAnomalies, "CNI_SANDBOX_FAILURE")
			slog.Warn("[SELF-HEAL] CNI_SANDBOX_FAILURE anomaly detected, executing CNI IPAM cleanup")
			res, err := w.remediator.Execute(ctx, RemediationRequest{
				Action: "cni_ipam_cleanup",
				Reason: "autonomous_watchdog: CNI_SANDBOX_FAILURE log anomaly",
			})
			if err != nil {
				slog.Error("[SELF-HEAL] CNI IPAM cleanup failed", slog.String("error", err.Error()))
			} else {
				msg := fmt.Sprintf("[SELF-HEAL] CNI IPAM cleanup complete: %d inodes freed", res.InodesFreed)
				slog.Info(msg)
				w.writeLogEntry("warn", msg)
			}
		}
	}

	// 6. Rule 5: Autonomous Unhealthy App Container Restart (capped at max 2 per tick)
	restartedContainers := 0
	for _, c := range health.UnhealthyContainers {
		if restartedContainers >= 2 {
			break
		}
		target := strings.TrimSpace(c.Name)
		if target == "" {
			target = strings.TrimSpace(c.ID)
		}
		if target == "" {
			continue
		}
		slog.Warn("[SELF-HEAL] Unhealthy container detected, restarting", slog.String("target", target), slog.String("status", c.Status))
		res, err := w.remediator.Execute(ctx, RemediationRequest{
			Action: "restart_container",
			Target: target,
			Reason: "autonomous_watchdog: unhealthy container " + c.Status,
		})
		if err != nil {
			slog.Error("[SELF-HEAL] Restarting container failed", slog.String("target", target), slog.String("error", err.Error()))
		} else {
			msg := fmt.Sprintf("[SELF-HEAL] Successfully restarted container %s: %s", target, res.Details)
			slog.Info(msg)
			w.writeLogEntry("warn", msg)
		}
		restartedContainers++
	}
}

func (w *SelfHealingWatchdog) writeLogEntry(level, msg string) {
	if w.logWriter == nil {
		return
	}
	_ = w.logWriter.Write(logengine.Entry{
		Timestamp: time.Now().UTC(),
		Service:   "k8s-agent-self-heal",
		Level:     level,
		Message:   msg,
	})
	_ = w.logWriter.Flush()
}
