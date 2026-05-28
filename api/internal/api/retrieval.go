package api  

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

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