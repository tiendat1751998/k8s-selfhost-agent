// Package catalog provides domain entities for the service catalog.
package catalog

import "time"

// ServiceTemplate represents a service catalog entry.
type ServiceTemplate struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Icon           string            `json:"icon"`
	Description    string            `json:"description"`
	Category       string            `json:"category"`
	Tags           []string          `json:"tags"`
	DeployCount    int               `json:"deploy_count"`
	TemplateConfig map[string]string `json:"template_config,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}
