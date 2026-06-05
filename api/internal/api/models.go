package api

type CreateRepoRequest struct {
	RepoURL  string 	`json:"repo_url"`
}

type CreateRepoResponse  struct{
	Success  bool 		`json:"success"`
	Message  string     `json:"message"`
	RepoID   string     `json:"repo_id"`
}