package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RepoSummaryResponse is the JSON shape returned per repository by GET /repos.
type RepoSummaryResponse struct {
	RepoID      string `json:"repo_id"`
	RepoURL     string `json:"repo_url"`
	IngestedAt  string `json:"ingested_at"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
}

// ListRepos handles GET /repos - returns every ingested repository with basic
// stats, newest first, so the frontend can show a "recent repos" list without
// relying on client-side storage.
func (h *RepoHandler) ListRepos(c *gin.Context) {

	summaries, err := h.store.ListRepositoriesWithStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]RepoSummaryResponse, 0, len(summaries))

	for _, s := range summaries {
		response = append(response, RepoSummaryResponse{
			RepoID:      s.ID,
			RepoURL:     s.RepoURL,
			IngestedAt:  s.CreatedAt.Format(time.RFC3339),
			FileCount:   s.FileCount,
			SymbolCount: s.SymbolCount,
		})
	}

	c.JSON(http.StatusOK, response)
}
