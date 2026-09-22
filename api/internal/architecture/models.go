package architecture

type Summary struct {
	RepoID string `json:"repo_id"`

	Statistics       Statistics    `json:"statistics"`
	Languages        []string      `json:"languages"`
	EntryPoints      []EntryPoint  `json:"entrypoints"`
	Components       []Component   `json:"components"`
	NarrativeSummary string        `json:"narrative_summary"`
	Concepts         []Concept     `json:"concepts"`
	ReadingPath      []ReadingStep `json:"reading_path"`
}

//Statistics contains stats info about the repository
type Statistics struct {
	FileCount   int `json:"file_count"`
	SymbolCount int `json:"symbol_count"`
	CallEdges   int `json:"call_edges"`
}

//Component represents an imp architectural building block
type Component struct {
	Name      string `json:"name"`
	FilePath  string `json:"file_path"`
	Type      string `json:"type"`
	Language  string `json:"language"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

//EntryPoint represents an executable entrypoint of the repo
type EntryPoint struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Language string `json:"language"`
	Type     string `json:"type"`
}

// Concept is a notable pattern/convention/domain term the LLM-generated
// overview flagged as worth explaining to a newcomer - see
// architecture.Service.GenerateOverview. Symbol/FilePath/StartLine/EndLine
// are optional (zero values when absent) - set only when the concept was
// resolved to a real, currently-indexed symbol, never trusted directly
// from the LLM's own claimed location.
type Concept struct {
	Term        string `json:"term"`
	Explanation string `json:"explanation"`
	Symbol      string `json:"symbol,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
}
