package deployment

// Application represents a running workload, either a Kubernetes Deployment or a Docker Swarm Service.
type Application struct {
	Name      string `json:"name"`
	Team      string `json:"team"`
	Env       string `json:"env"`
	Image     string `json:"image"`
	Target    string `json:"target"` // Cluster name
	Namespace string `json:"namespace"`
	Type      string `json:"type"` // "kubernetes" or "swarm"
	Replicas  int    `json:"replicas"`
	Status    string `json:"status"` // "healthy", "degraded", "down"
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	Port      int    `json:"port"`
	NetType   string `json:"netType"`
	Volume    string `json:"volume"`
}
