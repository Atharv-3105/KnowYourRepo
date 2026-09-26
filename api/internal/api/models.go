package api

type CreateRepoRequest struct {
	RepoURL string `json:"repo_url"`
}

type CreateRepoResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	RepoID  string `json:"repo_id"`
	// Cached is true when the repository was already fully ingested and is
	// returned as-is instead of re-running the pipeline.
	Cached bool `json:"cached,omitempty"`
}
