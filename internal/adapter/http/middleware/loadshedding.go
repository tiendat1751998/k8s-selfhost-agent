package middleware

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

// SystemCapacityMonitor provides current resource saturation readings.
type SystemCapacityMonitor interface {
	CurrentCPUUsage() float64
	ActiveLoad() int64
}

// AtomicCapacityMonitor is a thread-safe, atomic implementation of SystemCapacityMonitor.
type AtomicCapacityMonitor struct {
	cpuBits    atomic.Uint64
	activeLoad atomic.Int64
}

// NewAtomicCapacityMonitor creates an AtomicCapacityMonitor with initial values.
func NewAtomicCapacityMonitor(initialCPU float64, initialLoad int64) *AtomicCapacityMonitor {
	m := &AtomicCapacityMonitor{}
	m.SetCPUUsage(initialCPU)
	m.SetActiveLoad(initialLoad)
	return m
}

// SetCPUUsage updates the CPU utilization percentage atomically.
func (m *AtomicCapacityMonitor) SetCPUUsage(pct float64) {
	m.cpuBits.Store(math.Float64bits(pct))
}

// CurrentCPUUsage reads the current CPU utilization percentage.
func (m *AtomicCapacityMonitor) CurrentCPUUsage() float64 {
	return math.Float64frombits(m.cpuBits.Load())
}

// SetActiveLoad updates the active request count atomically.
func (m *AtomicCapacityMonitor) SetActiveLoad(load int64) {
	m.activeLoad.Store(load)
}

// ActiveLoad reads the active load count.
func (m *AtomicCapacityMonitor) ActiveLoad() int64 {
	return m.activeLoad.Load()
}

// Global default capacity monitor used when none is explicitly configured.
var defaultGlobalMonitor = NewAtomicCapacityMonitor(0.0, 0)

// GetGlobalCapacityMonitor returns the global capacity monitor instance.
func GetGlobalCapacityMonitor() *AtomicCapacityMonitor {
	return defaultGlobalMonitor
}

// LoadSheddingConfig defines parameters for adaptive load shedding.
type LoadSheddingConfig struct {
	CPUThresholdPercent float64
	ActiveLoadThreshold int64
	RetryAfterSeconds   int
	Monitor             SystemCapacityMonitor
	IsCriticalFunc      func(r *http.Request) bool
}

// DefaultLoadSheddingConfig returns standard defaults: 90% CPU, 5s retry after.
func DefaultLoadSheddingConfig() LoadSheddingConfig {
	return LoadSheddingConfig{
		CPUThresholdPercent: 90.0,
		ActiveLoadThreshold: 0, // 0 = disabled by default
		RetryAfterSeconds:   5,
		Monitor:             defaultGlobalMonitor,
		IsCriticalFunc:      DefaultIsCriticalEndpoint,
	}
}

// DefaultIsCriticalEndpoint identifies essential system operations that must bypass load shedding.
func DefaultIsCriticalEndpoint(r *http.Request) bool {
	path := strings.ToLower(r.URL.Path)

	// Liveness, Readiness, Ping
	if path == "/healthz" || path == "/readyz" || path == "/livez" || path == "/ping" {
		return true
	}

	// Auth & Identity
	if strings.HasPrefix(path, "/api/v1/auth") || strings.HasPrefix(path, "/auth") || strings.HasPrefix(path, "/api/v1/users") {
		return true
	}

	// Billing
	if strings.HasPrefix(path, "/api/v1/billing") || strings.HasPrefix(path, "/billing") {
		return true
	}

	// Deployments & Progressive delivery promotions & Gitops
	if strings.HasPrefix(path, "/api/v1/deployments") ||
		strings.HasPrefix(path, "/api/v1/promotions") ||
		strings.HasPrefix(path, "/api/v1/gitops") {
		return true
	}

	// Disaster recovery, backup restore, high-priority resilience
	if strings.HasPrefix(path, "/api/v1/backup/restore") ||
		strings.HasPrefix(path, "/api/v1/dr") ||
		strings.Contains(path, "/restore") {
		return true
	}

	// Admission controller webhooks
	if strings.HasPrefix(path, "/validate") || strings.HasPrefix(path, "/api/v1/admission") {
		return true
	}

	return false
}

// AdaptiveLoadShedding constructs a load shedding HTTP middleware.
func AdaptiveLoadShedding(cfg LoadSheddingConfig) func(http.Handler) http.Handler {
	if cfg.CPUThresholdPercent <= 0 {
		cfg.CPUThresholdPercent = 90.0
	}
	if cfg.RetryAfterSeconds <= 0 {
		cfg.RetryAfterSeconds = 5
	}
	if cfg.Monitor == nil {
		cfg.Monitor = defaultGlobalMonitor
	}
	if cfg.IsCriticalFunc == nil {
		cfg.IsCriticalFunc = DefaultIsCriticalEndpoint
	}

	var inFlight int64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			currentInFlight := atomic.AddInt64(&inFlight, 1)
			defer atomic.AddInt64(&inFlight, -1)

			cpuUsage := cfg.Monitor.CurrentCPUUsage()
			activeLoad := cfg.Monitor.ActiveLoad()
			if activeLoad == 0 {
				activeLoad = currentInFlight
			}

			isOverloaded := cpuUsage >= cfg.CPUThresholdPercent ||
				(cfg.ActiveLoadThreshold > 0 && activeLoad >= cfg.ActiveLoadThreshold)

			if isOverloaded {
				if cfg.IsCriticalFunc(r) {
					// Critical operations pass through to maintain platform availability
					next.ServeHTTP(w, r)
					return
				}

				// Non-critical request shed to prevent cascading failure
				retryStr := strconv.Itoa(cfg.RetryAfterSeconds)
				w.Header().Set("Retry-After", retryStr)
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusServiceUnavailable)

				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"type":     "https://k8sselfhost.io/errors/system-overload",
					"title":    "System Overloaded",
					"status":   http.StatusServiceUnavailable,
					"detail":   "System overload detected. Non-critical request shed to preserve core platform stability.",
					"code":     promotion.ErrCodeSystemOverloadLoadShedding,
					"instance": r.URL.Path,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
