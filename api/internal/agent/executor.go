package agent

import (
	"context"
	"log/slog"

	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

//Executor agent will run a plan against the registered tools and merges results into a single retrieval-result slice that the existing
//ContextBuilder /rag pipeline already know how to act on.
//A failing tool is logged and skipped rather than failing the whole request - the agent should degrade gracefully when one tool is unavailable
type Executor struct {
	tools   map[ToolName]Tool 
	logger  *slog.Logger 
}

func NewExecutor(tools map[ToolName]Tool, logger *slog.Logger) *Executor {
	return &Executor{
		tools: tools,
		logger: logger,
	}
}

func(e *Executor) Execute(ctx context.Context, plan []ToolName, req ToolRequest) []retrieval.RetrievalResult{

	var merged []retrieval.RetrievalResult
	seen := make(map[string]bool)

	for _, name := range plan {

		//get the tool from the tools map
		tool, ok := e.tools[name]
		if !ok {
			e.logger.Warn("agent_tool not_registered", "tool", name)
			continue 
		}

		e.logger.Info("agent_tool_started", "tool", name, "repo_id", req.RepoID)


		//Execute the tool
		result, err := tool.Execute(ctx, req)
		if err != nil{
			e.logger.Error("agent_tool_failed", "tool", name, "error", err)
			continue 
		}

		e.logger.Info("agent_tool_completed", "tool", name, "results", len(result.Results))

		for _, r := range result.Results {

			//Make a key for deduplication
			key := r.Symbol + "|" + r.FilePath
			if seen[key] {
				continue 
			}

			seen[key] = true

			merged = append(merged, r)
		}
	}

	return merged 
}
