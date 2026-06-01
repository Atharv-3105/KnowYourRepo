package api  

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SearchRequest struct {
	Query   string    	`json:"query"`
}

func (h *RepoHandler) ExpandSymbolContext(c *gin.Context) {

	symbol := c.Param("symbol")

	edges, err := h.graphRetriever.ExpandContext(context.Background(), symbol)

	if err != nil {

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)

		return 
	}

	c.JSON(
		http.StatusOK,
		edges,
	)
}

//Endpoint for Search
func(h *RepoHandler) Search(c *gin.Context) {

	var req SearchRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
	}

	results, err := h.hybridRetriever.Search(c.Request.Context(), req.Query)

	if err != nil {

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return 
	}

	ctxPackage := h.contextBuilder.Build(req.Query, results)

	c.JSON(
		http.StatusOK,
		ctxPackage,
	)


}