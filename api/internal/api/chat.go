package api

type ChatRequest struct {
	RepoID       string 	 `json:"repo_id"`
	Question     string      `json:"question"`
	SessionID    string      `json:"session_id"`
}