package agent

import "github.com/atharva-3105/KnowYourRepo/internal/retrieval"

type ToolName string

const (
	ToolSemantic     ToolName = "semantic"     //Will be the HybridRetriever.Search
	ToolGraph        ToolName = "graph"        //Will be the raw call_edges lookup by symbol name
	ToolArchitecture ToolName = "architecture" //will be the architecture.Service.BuildSummary
	ToolMemory       ToolName = "memory"       //Will be chat.Store history
)

// ToolRequest struct will carry every thing a tool needs to run, Our Tools are stateless
// with respect to a single question
type ToolRequest struct {
	RepoID  string
	Query   string
	History string
}

type ToolResult struct {
	Tool    ToolName
	Results []retrieval.RetrievalResult
}