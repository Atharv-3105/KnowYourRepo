package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *RepoHandler) GetCallGraph (c *gin.Context) {

	rows, err := h.store.DB().QueryContext(
			context.Background(),
			`
			SELECT caller_symbol, callee_symbol
			FROM call_edges
			`,
	)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
	}

	defer rows.Close()

	var edges []map[string]string

	for rows.Next() {

		var caller string 
		var callee string 

		err := rows.Scan(&caller, &callee)

		if err != nil {
			continue
		}

		edges = append(edges, 
					map[string]string{
						"caller": caller,
						"callee": callee,
					})
	}

	c.JSON(http.StatusOK, edges)
}