package tools

import (
	"context"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/agent"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

//This tool will feed recent conversation history as grounding context to the LLM can resolve follow-up questions
//that refer back to earlier turns instead of naming new code directly
type MemoryTool struct{}

func NewMemoryTool() *MemoryTool {
	return &MemoryTool{}
}

func(t *MemoryTool) Name() agent.ToolName{
	return agent.ToolMemory
}

func(t *MemoryTool) Execute(ctx context.Context, req agent.ToolRequest) (agent.ToolResult, error) {

	history := strings.TrimSpace(req.History)
	if history == "" {
		return agent.ToolResult{
			Tool: agent.ToolMemory,
		}, nil 
	}

	result := retrieval.RetrievalResult{
		Symbol: "conversation_history",
		Document: history,
	}

	return agent.ToolResult{
		Tool: agent.ToolMemory,
		Results: []retrieval.RetrievalResult{result},
	}, nil 
}