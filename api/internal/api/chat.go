package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	RepoID       string 	 `json:"repo_id"`
	Question     string      `json:"question"`
}

type ChatResponse struct {
	Answer     string       `json:"answer"`
}

func (h *RepoHandler) Chat (c *gin.Context) {

	var req ChatRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return 
	}

	//Hybrid Retrieval
	results, err := h.hybridRetriever.Search(
		c.Request.Context(),
		req.RepoID,
		req.Question,
	)

	if err != nil {
		
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return 
	}

	fmt.Println("Retrieved Results: ", len(results))

	//Perform RAG
	answer, err := h.ragService.AnswerQuestion(
		c.Request.Context(),
		req.Question,
		results,
	)
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
		ChatResponse{
			Answer: answer,
		},
	)

}