package cluster

import "errors"

// Standard component names for cluster essentials.
const (
	ComponentNameMetricsServer = "metrics-server"
	ComponentNameLocalStorage  = "local-storage"
	ComponentNameControlAgent  = "k8s-control-agent"

	ComponentStatusRunning  = "Running"
	ComponentStatusPending  = "Pending"
	ComponentStatusNotFound = "NotFound"
)

// EssentialComponent describes an essential cluster add-on and its runtime state.
type EssentialComponent struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Status    string `json:"status"` // "Running", "Pending", "NotFound"
	Namespace string `json:"namespace"`
}

// NodeTaintInfo represents whether a node is tainted and details of its taints.
type NodeTaintInfo struct {
	NodeName string `json:"node_name"`
	Tainted  bool   `json:"tainted"`
	Taints   string `json:"taints,omitempty"`
}

// ClusterEssentialsStatus summarizes the bootstrap state of cluster essentials.
type ClusterEssentialsStatus struct {
	ClusterID  string               `json:"cluster_id"`
	Components []EssentialComponent `json:"components"`
	TaintsInfo []NodeTaintInfo      `json:"taints_info"`
	Ready      bool                 `json:"ready"`
}

// BootstrapRequest specifies which bootstrap actions should be executed.
type BootstrapRequest struct {
	InstallMetricsServer bool `json:"install_metrics_server"`
	InstallStorageClass  bool `json:"install_storage_class"`
	UntaintMasters       bool `json:"untaint_masters"`
	DeployAgentDaemonSet bool `json:"deploy_agent_daemonset"`
}

// Validate checks that at least one action was requested.
func (r *BootstrapRequest) Validate() error {
	if !r.InstallMetricsServer && !r.InstallStorageClass && !r.UntaintMasters && !r.DeployAgentDaemonSet {
		return errors.New("at least one bootstrap action must be selected")
	}
	return nil
}

// BootstrapResult summarizes the outcome of the bootstrap operation.
type BootstrapResult struct {
	Success    bool     `json:"success"`
	Installed  []string `json:"installed"`
	Errors     []string `json:"errors"`
	DurationMs int64    `json:"duration_ms"`
}

// NewClusterEssentialsStatus constructs a ClusterEssentialsStatus and computes the overall Ready state.
func NewClusterEssentialsStatus(clusterID string, components []EssentialComponent, taintsInfo []NodeTaintInfo) *ClusterEssentialsStatus {
	ready := computeReady(components, taintsInfo)
	return &ClusterEssentialsStatus{
		ClusterID:  clusterID,
		Components: components,
		TaintsInfo: taintsInfo,
		Ready:      ready,
	}
}

func computeReady(components []EssentialComponent, taintsInfo []NodeTaintInfo) bool {
	if len(components) == 0 {
		return false
	}
	for _, c := range components {
		if !c.Installed || c.Status != ComponentStatusRunning {
			return false
		}
	}
	for _, t := range taintsInfo {
		if t.Tainted {
			return false
		}
	}
	return true
}

// NewBootstrapResult creates a new BootstrapResult with safe slice defaults.
func NewBootstrapResult(success bool, installed, errs []string, durationMs int64) *BootstrapResult {
	if installed == nil {
		installed = make([]string, 0)
	}
	if errs == nil {
		errs = make([]string, 0)
	}
	return &BootstrapResult{
		Success:    success,
		Installed:  installed,
		Errors:     errs,
		DurationMs: durationMs,
	}
}
