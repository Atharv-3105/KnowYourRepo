package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SymbolResponse struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Language  string `json:"language"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// ListSymbols handles GET /symbols/:repoID?search= - the repo's real,
// complete symbol index (every row in `symbols` for this repo, joined to
// its file), optionally filtered by a case-insensitive substring match on
// name. Unlike GET /architecture/:repoID's `components`, which is a
// heuristic subset matched by name suffix, this returns every symbol.
func (h *RepoHandler) ListSymbols(c *gin.Context) {

	repoID := c.Param("repoID")
	search := c.Query("search")

	symbols, err := h.store.ListSymbols(c.Request.Context(), repoID, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]SymbolResponse, 0, len(symbols))
	for _, sym := range symbols {
		response = append(response, SymbolResponse{
			Name:      sym.Name,
			Type:      sym.Type,
			Language:  sym.Language,
			FilePath:  sym.FilePath,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
		})
	}

	c.JSON(http.StatusOK, response)
}
