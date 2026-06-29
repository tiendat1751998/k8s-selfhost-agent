package tagging

import "time"

// Tag represents a key-value tag applied to resources.
type Tag struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Category  string    `json:"category"` // e.g., 'environment', 'business_unit', 'owner'
	CreatedAt time.Time `json:"created_at"`
}

// ResourceTag represents a tag applied to a specific resource.
type ResourceTag struct {
	ResourceID string `json:"resource_id"`
	TagID      string `json:"tag_id"`
}
