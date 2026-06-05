package sidecar 

type EmbedBatchItem struct {
	ID       string 		`json:"id"`
	Text     string         `json:"text"`
	Metadata   map[string]interface{}	`json:"metadata"`
}


type EmbedBatchRequest struct {
	Items       []EmbedBatchItem		`json:"items"`
}

type EmbedResponse struct {
	Success  bool    `json:"success"`
}

type SearchRequest struct {
	Query   string `json:"query"`
	RepoID  string  `json:"repo_id"`
	Limit   int     `json:"limit"`
}

type SearchResult   struct {
	ID       string        `json:"id"`
	Document    string     `json:"document"`
	Metadata    map[string]interface{}    `json:"metadata"`
	Distance    float64			`json:"distance"`
}