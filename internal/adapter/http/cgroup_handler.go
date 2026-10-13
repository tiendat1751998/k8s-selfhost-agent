package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
)

// Hardware Bottleneck & Kernel Failure Mode Notes:
// 1. Kernel Context Switches & TLB Flushes:
//    High-frequency cgroup inotify polling causes kernel thread preemption and cross-CPU TLB shootdowns.
//    The handler passes buffered telemetry without blocking kernel scheduler rings.
// 2. RLIMIT_MEMLOCK & eBPF Map Limits:
//    Scaling eBPF maps allocates unevictable kernel pages locked under RLIMIT_MEMLOCK. If limits are reached,
//    new conntrack entries fail with -EPERM or -ENOMEM, dropping traffic at line rate.
// 3. cgroup v2 memory.high Sleep Penalty:
//    Exceeding memory.high forces synchronous proportional delays in alloc_pages(). The handler surfaces
//    these hidden latency stalls so operators can adjust memory.high.factor = 0 or elevate container limits.
// 4. Kernel Slab Inflation:
//    Build workloads inflate dentry/inode caches, triggering false Kubelet memory pressure evictions.
//    Tuning vm.vfs_cache_pressure = 150 forces cache reclamation before pod eviction.

// resizeConntrackRequest defines the input payload for manual or automated eBPF conntrack resizing.
type resizeConntrackRequest struct {
	Capacity int64 `json:"capacity"`
}

// CGroupEventsResponse wraps audited cgroup v2 memory events alongside mitigation recommendations.
type CGroupEventsResponse struct {
	Events          []domain.CGroupV2MemoryEvent     `json:"events"`
	Recommendations []usecase.CGroupRecommendation `json:"recommendations"`
}

// CGroupHandler handles HTTP requests for cgroup v2 kernel telemetry and eBPF auto-scaling.
type CGroupHandler struct {
	scalerUsecase *usecase.CGroupScalerUsecase
	interceptor   domain.KernelInterceptorManager
}

// NewCGroupHandler constructs a new CGroupHandler instance.
func NewCGroupHandler(
	scalerUsecase *usecase.CGroupScalerUsecase,
	interceptor domain.KernelInterceptorManager,
) *CGroupHandler {
	return &CGroupHandler{
		scalerUsecase: scalerUsecase,
		interceptor:   interceptor,
	}
}

// RegisterRoutes mounts all kernel and eBPF routes onto the provided chi Router.
func (h *CGroupHandler) RegisterRoutes(r chi.Router) {
	r.Route("/nodes/{nodeName}/kernel", func(nodeSub chi.Router) {
		h.RegisterNodeKernelRoutes(nodeSub)
	})
	r.Route("/clusters/{clusterId}/ebpf", func(clusterSub chi.Router) {
		h.RegisterClusterEBPFRoutes(clusterSub)
	})
}

// RegisterNodeKernelRoutes registers routes under /nodes/{nodeName}/kernel.
func (h *CGroupHandler) RegisterNodeKernelRoutes(r chi.Router) {
	r.Get("/cgroup/events", h.GetCGroupEvents)
	r.Post("/slab/tune", h.TuneSlabCache)
}

// RegisterClusterEBPFRoutes registers routes under /clusters/{clusterId}/ebpf.
func (h *CGroupHandler) RegisterClusterEBPFRoutes(r chi.Router) {
	r.Get("/maps", h.GetEBPFMaps)
	r.Post("/conntrack/resize", h.ResizeConntrackMap)
}

// GetCGroupEvents handles GET /api/v1/nodes/{nodeName}/kernel/cgroup/events
func (h *CGroupHandler) GetCGroupEvents(w http.ResponseWriter, r *http.Request) {
	nodeName := strings.TrimSpace(chi.URLParam(r, "nodeName"))
	if nodeName == "" {
		http.Error(w, `{"error":"nodeName parameter is required"}`, http.StatusBadRequest)
		return
	}

	events, recommendations, err := h.scalerUsecase.AuditCGroupThrottling(r.Context(), nodeName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	resp := CGroupEventsResponse{
		Events:          events,
		Recommendations: recommendations,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// TuneSlabCache handles POST /api/v1/nodes/{nodeName}/kernel/slab/tune
func (h *CGroupHandler) TuneSlabCache(w http.ResponseWriter, r *http.Request) {
	nodeName := strings.TrimSpace(chi.URLParam(r, "nodeName"))
	if nodeName == "" {
		http.Error(w, `{"error":"nodeName parameter is required"}`, http.StatusBadRequest)
		return
	}

	report, err := h.scalerUsecase.TuneKernelSlabCache(r.Context(), nodeName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

// GetEBPFMaps handles GET /api/v1/clusters/{clusterId}/ebpf/maps
func (h *CGroupHandler) GetEBPFMaps(w http.ResponseWriter, r *http.Request) {
	clusterID := strings.TrimSpace(chi.URLParam(r, "clusterId"))
	if clusterID == "" {
		http.Error(w, `{"error":"clusterId parameter is required"}`, http.StatusBadRequest)
		return
	}

	maps, err := h.scalerUsecase.MonitorAndScaleEBPFMaps(r.Context(), clusterID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(maps)
}

// ResizeConntrackMap handles POST /api/v1/clusters/{clusterId}/ebpf/conntrack/resize
func (h *CGroupHandler) ResizeConntrackMap(w http.ResponseWriter, r *http.Request) {
	clusterID := strings.TrimSpace(chi.URLParam(r, "clusterId"))
	if clusterID == "" {
		http.Error(w, `{"error":"clusterId parameter is required"}`, http.StatusBadRequest)
		return
	}

	var req resizeConntrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "malformed json payload"})
		return
	}

	if req.Capacity <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "capacity must be positive integer"})
		return
	}

	if err := h.interceptor.AutoResizeEBPFConntrackMap(r.Context(), clusterID, req.Capacity); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"cluster_id":   clusterID,
		"new_capacity": req.Capacity,
		"message":      "eBPF conntrack map dynamically resized without dropping active TCP connections",
	})
}
