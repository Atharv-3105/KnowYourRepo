package api 

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func(h *RepoHandler) GetJobStatus(c *gin.Context) {

	jobID := c.Param("id")

	job, err := h.store.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return 
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id": job.ID,
		"repo_url": job.RepoURL,
		"status": job.Status,
		"error_message": job.ErrorMessage,
	})
}

