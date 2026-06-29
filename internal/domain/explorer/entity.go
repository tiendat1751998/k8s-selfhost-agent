// Package explorer provides domain entities for the resource explorer.
package explorer

// Resource represents a Kubernetes resource with metadata.
type Resource struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"` // Pod | Deployment | Service | Node | Ingress | PVC
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Cluster   string            `json:"cluster"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels,omitempty"`
	Age       string            `json:"age"`
}
