package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/agent"
	"github.com/atharva-3105/KnowYourRepo/internal/architecture"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
)

type ArchitectureTool struct {
	service  *architecture.Service 
}

func NewArchitectureTool(service *architecture.Service) *ArchitectureTool {
	return &ArchitectureTool{
		service: service,
	}
}

func(t *ArchitectureTool) Name() agent.ToolName {
	return agent.ToolArchitecture
}

const maxArchiectureListItems = 15

func(t *ArchitectureTool) Execute(ctx context.Context, req agent.ToolRequest) (agent.ToolResult, error) {

	summary, err := t.service.BuildSummary(ctx, req.RepoID)
	if err != nil{
		return agent.ToolResult{}, err 
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Repository statistics: %d files, %d symbols, %d call edges. \n", summary.Statistics.FileCount, summary.Statistics.SymbolCount, summary.Statistics.CallEdges)

	fmt.Fprintf(&b, "Languages: %s\n", strings.Join(summary.Languages, ", "))

	b.WriteString("EntryPoints: \n")
	for i, ep := range summary.EntryPoints {
		if i >= maxArchiectureListItems {
			break 
		}

		fmt.Fprintf(&b, "- %s (%s) in %s\n", ep.Name, ep.Type, ep.FilePath)
	}

	b.WriteString("Components:\n")
	for i, c := range summary.Components {
		if i >= maxArchiectureListItems{
			break 
		}

		fmt.Fprintf(&b, "- %s (%s) in %s\n", c.Name, c.Type, c.FilePath)
	}

	result := retrieval.RetrievalResult{
		Symbol: "architecture_overview",
		FilePath: req.RepoID,
		Document: b.String(),
	}


	return agent.ToolResult{
		Tool: agent.ToolArchitecture, 
		Results: []retrieval.RetrievalResult{result},
	}, nil 

}