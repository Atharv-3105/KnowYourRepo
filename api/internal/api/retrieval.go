package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxSearchQueryChars bounds a semantic search query - it's embedded by the
// sidecar (a real provider call against a free-tier quota), and nothing a
// person types into a search box needs to be longer than this.
const maxSearchQueryChars = 300

type SearchRequest struct {
	Query  string `json:"query"`
	RepoID string `json:"repo_id"`
}

// Search handles POST /search - semantic retrieval for the command palette,
// returning the same context package the chat pipeline builds internally.
func (h *RepoHandler) Search(c *gin.Context) {

	var req SearchRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" || req.RepoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query and repo_id are required"})
		return
	}
	if len(req.Query) > maxSearchQueryChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is too long"})
		return
	}

	if !h.requireRepoExists(c, req.RepoID) {
		return
	}

	results, err := h.hybridRetriever.Search(c.Request.Context(), req.RepoID, req.Query)
	if err != nil {
		// Semantic search depends on an embedding quota that can run out
		// for the day; identifier search over the symbol index doesn't.
		h.logger.Warn("search_semantic_failed_using_lexical", "repo_id", req.RepoID, "error", err)

		results, err = h.hybridRetriever.LexicalSearch(c.Request.Context(), req.RepoID, req.Query)
		if err != nil {
			respondInternalError(c, h.logger, "search_failed", err, "repo_id", req.RepoID)
			return
		}
	}

	c.JSON(http.StatusOK, h.contextBuilder.Build(req.Query, results))
}
