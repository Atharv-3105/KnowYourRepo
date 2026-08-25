package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/agent"
	"github.com/atharva-3105/KnowYourRepo/internal/retrieval"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type GraphTool struct {
	store *store.Store
}

func NewGraphTool(store *store.Store) *GraphTool{
	return &GraphTool{
		store: store, 
	}
}

func(t *GraphTool) Name() agent.ToolName {
	return agent.ToolGraph
}


var identifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

//A dictionary of common-words which should be skipped
var commonWords = map[string]bool{
	"who": true, "what": true, "calls": true, "call": true, "does": true, "the": true,
	"is": true, "are": true, "how": true, "when": true, "where": true, "into": true,
	"function": true, "method": true, "this": true, "that": true, "which": true,
	"depends": true, "depend": true, "dependency": true, "dependencies": true,
}

//extractSymbol picks the most likely function/symbol name mentioned in a natural-language query asked by user,
//using a heuristic: prefer identifiers that look like code over common English words and prefer the last word in the user query as the last word is generally the 
//symbol user is asking(Ex: "Who calls CleanUpRoom")
func extractSymbol(query string)string {

	candidates := identifierPattern.FindAllString(query, -1)

	var best string 

	for _, c := range candidates {
		if commonWords[strings.ToLower(c)] {
			continue
		} 
		if looksLikeIdentifier(c) {
			best = c
		}
	}

	return best 
}

func looksLikeIdentifier(s string) bool {
	if strings.Contains(s, "_"){
		return true 
	}

	hasUpper, hasLower := false, false

	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true 
		}
		if r >= 'a' && r <= 'z' {
			hasLower = true 
		}
	}

	return hasUpper && hasLower && len(s) > 3
}

func(t *GraphTool) Execute(ctx context.Context,req agent.ToolRequest) (agent.ToolResult, error){

	symbol := extractSymbol(req.Query)
	if symbol == "" {
		return agent.ToolResult{
			Tool: agent.ToolGraph,
		}, nil 
	}

	incoming, err := t.store.GetIncomingCalls(ctx, req.RepoID, symbol)
	if err != nil {
		return agent.ToolResult{}, err 
	}

	outgoing, err := t.store.GetOutgoingCallsBySymbol(ctx, req.RepoID, symbol)
	if err != nil {
		return agent.ToolResult{}, err 
	}

	if len(incoming) == 0 && len(outgoing) == 0{
		return agent.ToolResult{Tool: agent.ToolGraph}, nil 
	}

	var edges []retrieval.GraphEdge
	var lines []string 

	for _, e := range incoming {
		edges = append(edges, retrieval.GraphEdge{Caller: e.CallerSymbol, Callee: e.CalleeSymbol})

		lines = append(lines, fmt.Sprintf("%s calls %s (in %s)", e.CallerSymbol, e.CalleeSymbol, e.CallerFilePath))
	}

	for _, e := range outgoing {
		edges = append(edges, retrieval.GraphEdge{Caller: e.CallerSymbol, Callee: e.CalleeSymbol})

		lines = append(lines, fmt.Sprintf("%s calls %s (in %s)", e.CallerSymbol, e.CalleeSymbol, e.CallerFilePath))
	}

	result := retrieval.RetrievalResult{
		Symbol: symbol,
		Document: strings.Join(lines, "\n"),
		Edges: edges,
	}

	return agent.ToolResult{Tool:agent.ToolGraph, Results: []retrieval.RetrievalResult{result}}, nil 

}