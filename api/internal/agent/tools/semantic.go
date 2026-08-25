package tools

import (
	"context"

	"github.com/atharva-3105/KnowYourRepo/internal/agent"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

type SemanticTool struct {
	retriever *retrieval.HybridRetriever
}

func NewSemanticTool(retriever *retrieval.HybridRetriever) *SemanticTool {
	return &SemanticTool{
		retriever: retriever,
	}
}

func(t *SemanticTool) Name() agent.ToolName {return agent.ToolSemantic}

func(t *SemanticTool) Execute(ctx context.Context, req agent.ToolRequest) (agent.ToolResult, error) {

	results, err := t.retriever.Search(ctx, req.RepoID, req.Query)
	if err != nil {
		return agent.ToolResult{}, err 
	}

	return agent.ToolResult{
		Tool: agent.ToolSemantic,
		Results: results,
	}, nil 
}