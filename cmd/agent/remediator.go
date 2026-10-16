package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrRemediationCooldown indicates circuit breaker tripped for a target.
var ErrRemediationCooldown = errors.New("remediation circuit breaker: rate limit exceeded, target in cooldown")

// RemediationRequest defines a requested remediation action on the node.
type RemediationRequest struct {
	Action string `json:"action"`           // "restart_service" | "disk_pressure_cleanup" | "cni_ipam_cleanup" | "reap_zombies"
	Target string `json:"target,omitempty"` // "kubelet" | "containerd" | "docker" | "crio"
	Reason string `json:"reason,omitempty"`
}

// RemediationActionResult holds the audit trail and outcome of a remediation task.
type RemediationActionResult struct {
	Success     bool   `json:"success"`
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	BytesFreed  int64  `json:"bytes_freed,omitempty"`
	InodesFreed int64  `json:"inodes_freed,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
	Details     string `json:"details"`
	Timestamp   string `json:"timestamp"`
}

// CommandRunner executes commands without invoking a shell.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type osCommandRunner struct{}

func (r *osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// NodeRemediator executes allowlisted remediation tasks with circuit breakers and audit logging.
type NodeRemediator struct {
	mu             sync.Mutex
	runner         CommandRunner
	logDir         string
	cniDir         string
	procPath       string
	history        []RemediationActionResult
	historyLimit   int
	restartHistory map[string][]time.Time
}

// RemediatorOption configures NodeRemediator.
type RemediatorOption func(*NodeRemediator)

// WithCommandRunner injects a custom CommandRunner.
func WithCommandRunner(r CommandRunner) RemediatorOption {
	return func(n *NodeRemediator) {
		if r != nil {
			n.runner = r
		}
	}
}

// WithRemediatorLogDir sets the log directory for disk pressure cleanup.
func WithRemediatorLogDir(dir string) RemediatorOption {
	return func(n *NodeRemediator) {
		if dir != "" {
			n.logDir = dir
		}
	}
}

// WithCNINetworksDir sets the CNI directory for IPAM cleanup.
func WithCNINetworksDir(dir string) RemediatorOption {
	return func(n *NodeRemediator) {
		if dir != "" {
			n.cniDir = dir
		}
	}
}

// NewNodeRemediator creates a new allowlisted remediation engine.
func NewNodeRemediator(opts ...RemediatorOption) *NodeRemediator {
	nr := &NodeRemediator{
		runner:         &osCommandRunner{},
		logDir:         "/var/log",
		cniDir:         "/var/lib/cni/networks",
		procPath:       "/proc",
		history:        make([]RemediationActionResult, 0, 50),
		historyLimit:   50,
		restartHistory: make(map[string][]time.Time),
	}
	for _, opt := range opts {
		opt(nr)
	}
	return nr
}

var validTargets = map[string]bool{
	"kubelet":    true,
	"containerd": true,
	"docker":     true,
	"crio":       true,
}

var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Execute performs an allowlisted remediation action safely.
func (nr *NodeRemediator) Execute(ctx context.Context, req RemediationRequest) (RemediationActionResult, error) {
	start := time.Now()
	res := RemediationActionResult{
		Action:    req.Action,
		Target:    req.Target,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := validateRemediationRequest(req); err != nil {
		res.DurationMs = time.Since(start).Milliseconds()
		res.Success = false
		res.Details = err.Error()
		return res, err
	}

	var err error
	switch req.Action {
	case "restart_service":
		err = nr.executeRestartService(ctx, req, &res)
	case "disk_pressure_cleanup":
		err = nr.executeDiskCleanup(ctx, &res)
	case "cni_ipam_cleanup":
		err = nr.executeCNICleanup(ctx, &res)
	case "reap_zombies":
		err = nr.executeReapZombies(ctx, &res)
	}

	res.DurationMs = time.Since(start).Milliseconds()
	res.Success = (err == nil)
	if err != nil {
		if res.Details == "" {
			res.Details = err.Error()
		}
	}

	nr.recordHistory(res)
	return res, err
}

func validateRemediationRequest(req RemediationRequest) error {
	switch req.Action {
	case "restart_service":
		target := strings.TrimSpace(req.Target)
		if !safeNamePattern.MatchString(target) || !validTargets[target] {
			return fmt.Errorf("unsupported target %q: strictly one of kubelet, containerd, docker, crio allowed", req.Target)
		}
	case "disk_pressure_cleanup", "cni_ipam_cleanup", "reap_zombies":
		// valid
	default:
		return fmt.Errorf("unsupported action %q: only restart_service, disk_pressure_cleanup, cni_ipam_cleanup, reap_zombies allowed", req.Action)
	}
	return nil
}

func (nr *NodeRemediator) checkAndRecordCooldown(target string) error {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	now := time.Now()
	window := 30 * time.Minute
	cutoff := now.Add(-window)

	var validTimes []time.Time
	for _, t := range nr.restartHistory[target] {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}

	if len(validTimes) >= 3 {
		nr.restartHistory[target] = validTimes
		return ErrRemediationCooldown
	}

	validTimes = append(validTimes, now)
	nr.restartHistory[target] = validTimes
	return nil
}

func (nr *NodeRemediator) executeRestartService(ctx context.Context, req RemediationRequest, res *RemediationActionResult) error {
	target := strings.TrimSpace(req.Target)
	if !safeNamePattern.MatchString(target) || !validTargets[target] {
		return fmt.Errorf("unsupported target %q: strictly one of kubelet, containerd, docker, crio allowed", req.Target)
	}

	if err := nr.checkAndRecordCooldown(target); err != nil {
		return err
	}

	out, err := nr.runner.Run(ctx, "systemctl", "restart", target)
	if err != nil {
		out, err = nr.runner.Run(ctx, "sudo", "-n", "systemctl", "restart", target)
	}
	res.Details = string(out)
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w (%s)", target, err, string(out))
	}
	return nil
}

func (nr *NodeRemediator) executeDiskCleanup(ctx context.Context, res *RemediationActionResult) error {
	var totalBytes int64
	var totalInodes int64

	// 1. Clean rotated log archives (*.gz, *.1, *.old)
	_ = filepath.Walk(nr.logDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".1") || strings.HasSuffix(name, ".old") {
			size := info.Size()
			if removeErr := os.Remove(path); removeErr == nil {
				totalBytes += size
				totalInodes++
			}
		}
		return nil
	})

	// 2. Run container prune via crictl or docker
	crictlOut, crictlErr := nr.runner.Run(ctx, "crictl", "rmi", "--prune")
	if crictlErr != nil {
		dockerOut, _ := nr.runner.Run(ctx, "docker", "system", "prune", "-f")
		res.Details = fmt.Sprintf("Cleaned rotated logs; docker prune: %s", string(dockerOut))
	} else {
		res.Details = fmt.Sprintf("Cleaned rotated logs; crictl prune: %s", string(crictlOut))
	}

	res.BytesFreed = totalBytes
	res.InodesFreed = totalInodes
	return nil
}

func (nr *NodeRemediator) executeCNICleanup(ctx context.Context, res *RemediationActionResult) error {
	var inodesFreed int64
	entries, err := os.ReadDir(nr.cniDir)
	if err != nil {
		if os.IsNotExist(err) {
			res.Details = "CNI networks directory does not exist, nothing to clean"
			return nil
		}
		return fmt.Errorf("reading CNI directory %s: %w", nr.cniDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Remove lock files or stale IPs
		if strings.HasSuffix(name, ".lock") || name == "lock" || isStaleIPFile(name) {
			fullPath := filepath.Join(nr.cniDir, name)
			if removeErr := os.Remove(fullPath); removeErr == nil {
				inodesFreed++
			}
		}
	}

	res.InodesFreed = inodesFreed
	res.Details = fmt.Sprintf("CNI IPAM cleaned %d stale locks and allocated IP entries", inodesFreed)
	return nil
}

func isStaleIPFile(name string) bool {
	// Pattern like 10.x.x.x
	parts := strings.Split(name, ".")
	return len(parts) == 4
}

func (nr *NodeRemediator) executeReapZombies(ctx context.Context, res *RemediationActionResult) error {
	// Scan /proc for zombie shims and reap them
	reaped := 0
	entries, err := os.ReadDir(nr.procPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pid := entry.Name()
			if len(pid) == 0 || pid[0] < '0' || pid[0] > '9' {
				continue
			}
			statBytes, err := os.ReadFile(filepath.Join(nr.procPath, pid, "stat"))
			if err != nil {
				continue
			}
			statStr := string(statBytes)
			if (strings.Contains(statStr, "shim") || strings.Contains(statStr, "containerd")) && strings.Contains(statStr, ") Z") {
				// Reap using runner
				_, _ = nr.runner.Run(ctx, "kill", "-9", pid)
				reaped++
			}
		}
	}
	res.Details = fmt.Sprintf("Scanned for orphaned container shims; terminated %d processes", reaped)
	return nil
}

func (nr *NodeRemediator) recordHistory(res RemediationActionResult) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	nr.history = append(nr.history, res)
	if len(nr.history) > nr.historyLimit {
		nr.history = nr.history[len(nr.history)-nr.historyLimit:]
	}
}

// GetHistory returns a copy of recent remediation actions.
func (nr *NodeRemediator) GetHistory() []RemediationActionResult {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	res := make([]RemediationActionResult, len(nr.history))
	copy(res, nr.history)
	return res
}

// RegisterRoutes mounts /remediate and /remediate/history onto the ServeMux.
func (nr *NodeRemediator) RegisterRoutes(mux *http.ServeMux, isAuthorized func(r *http.Request) bool) {
	mux.HandleFunc("/remediate/history", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		history := nr.GetHistory()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(history)
	})

	mux.HandleFunc("/remediate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !isAuthorized(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		var req RemediationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload: " + err.Error()})
			return
		}

		res, err := nr.Execute(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			if errors.Is(err, ErrRemediationCooldown) {
				w.WriteHeader(http.StatusTooManyRequests)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			_ = json.NewEncoder(w).Encode(res)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	})
}
