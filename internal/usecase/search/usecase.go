package search

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/search"
)

// Usecase coordinates search logic.
type Usecase struct {
	repo search.Repository
}

// NewUsecase creates a new Search Usecase.
func NewUsecase(repo search.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// Search processes global search queries.
func (u *Usecase) Search(ctx context.Context, q string, searchType string) ([]search.Result, error) {
	if q == "" {
		return []search.Result{}, nil
	}
	return u.repo.Search(ctx, q, searchType)
}
