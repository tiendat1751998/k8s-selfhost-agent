package search

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/search"
)

type mockSearchRepo struct {
	results []search.Result
}

func (m *mockSearchRepo) Search(ctx context.Context, q string, searchType string) ([]search.Result, error) {
	return m.results, nil
}

func TestSearchUsecase_Search(t *testing.T) {
	expectedResults := []search.Result{
		{Type: "incident", Title: "Incident in Pod web-pod", Desc: "CrashLoopBackOff"},
	}

	repo := &mockSearchRepo{results: expectedResults}
	uc := NewUsecase(repo)

	// Search with query
	res, err := uc.Search(context.Background(), "web", "all")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 1 || res[0].Title != expectedResults[0].Title {
		t.Errorf("unexpected Search results: %+v", res)
	}

	// Search empty query
	resEmpty, err := uc.Search(context.Background(), "", "all")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resEmpty) != 0 {
		t.Errorf("expected empty search results, got: %+v", resEmpty)
	}
}
