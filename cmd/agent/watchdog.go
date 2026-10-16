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
	consecutiveKubeletFail int
	consecutiveCRIFail     int
	logWriter              *logengine.Writer
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

// NewSelfHealingWatchdog constructs an autonomous watchdog.
func NewSelfHealingWatchdog(checker *K8sHealthChecker, remediator *NodeRemediator, opts ...WatchdogOption) *SelfHealingWatchdog {
	w := &SelfHealingWatchdog{
		checker:      checker,
		remediator:   remediator,
		interval:     30 * time.Second,
		diskCooldown: 15 * time.Minute,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
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
	now := time.Now()

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
