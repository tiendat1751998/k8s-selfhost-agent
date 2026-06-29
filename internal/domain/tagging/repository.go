package tagging

import "context"

// Repository defines data access for the tagging system.
type Repository interface {
	ListTags(ctx context.Context) ([]Tag, error)
	CreateTag(ctx context.Context, tag *Tag) error
	DeleteTag(ctx context.Context, id string) error
	TagResource(ctx context.Context, resourceID, tagID string) error
	UntagResource(ctx context.Context, resourceID, tagID string) error
	GetResourceTags(ctx context.Context, resourceID string) ([]Tag, error)
}
