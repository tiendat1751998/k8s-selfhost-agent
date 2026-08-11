package search

import "context"

// Result represents a single search result.
type Result struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

// Repository defines data access ports for searching.
type Repository interface {
	Search(ctx context.Context, q string, searchType string) ([]Result, error)
}
