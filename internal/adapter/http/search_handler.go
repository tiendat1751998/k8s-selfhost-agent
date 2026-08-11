package http

import (
	"net/http"

	searchUsecase "github.com/datdt/k8sselfhost/internal/usecase/search"
)

// SearchHandler handles global search HTTP requests.
type SearchHandler struct {
	usecase *searchUsecase.Usecase
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(usecase *searchUsecase.Usecase) *SearchHandler {
	return &SearchHandler{usecase: usecase}
}

// Search handles GET /api/v1/search
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "all"
	}

	results, err := h.usecase.Search(r.Context(), q, searchType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed", err)
		return
	}

	writeJSON(w, http.StatusOK, results)
}
