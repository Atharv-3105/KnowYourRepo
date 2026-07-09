package architecture

type Summary struct {
	RepoID 		string 			`json:"repo_id"`

	Statistics 	Statistics 		`json:"statistics"`
	Languages 	[]string 		`json:"languages"`
	EntryPoints	[]EntryPoint 	`json:"entrypoints"`
	Components	[]Component		`json:"components"`
}

//Statistics contains stats info about the repository
type Statistics struct {
	FileCount		int 		`json:"file_count"`
	SymbolCount		int			`json:"symbol_count"`
	CallEdges       int         `json:"call_edges"`
}

//Component represents an imp architectural building block
type Component struct {
	Name      string      `json:"name"`
	FilePath  string      `json:"file_path"`
	Type      string      `json:"type"`
	Language  string      `json:"language"`
	StartLine  int        `json:"start_line"`
	EndLine    int        `json:"end_line"`
}

//EntryPoint represents an executable entrypoint of the repo
type EntryPoint  struct {
	Name     	string       `json:"name"`
	FilePath	string 	  	 `json:"file_path"`
	Language    string    	 `json:"language"`
	Type        string    	 `json:"type"`
}

