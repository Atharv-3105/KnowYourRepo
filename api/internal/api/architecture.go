package api 

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func(h *RepoHandler) GetArchitecture(c *gin.Context) {

	repoID := c.Param("repoID")

	if repoID == "" {

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "repo_id is required",
		})

		return 
	}

	h.logger.Info("architecture_request_received", "repo_id", repoID)

	summary, err := h.architectureService.BuildSummary(c.Request.Context(), repoID)

	if err != nil {

		h.logger.Error("architecture_request_failed", "repo_id", repoID, "error", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})

		return 
	}
	
	h.logger.Info("architecture_request_completed", "repo_id", repoID)

	c.JSON(http.StatusOK, summary)
	
}