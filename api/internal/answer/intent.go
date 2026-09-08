package answer

import "strings"

// reingestKeywords are checked to decide whether a question implies the
// user wants the repository's current/latest state, independent of what
// else the question needs to answer it. Ported from
// api/internal/agent/reingest_intent.go - unrelated to tool selection,
// it just needed a new home once the agent package was removed.
var reingestKeywords = []string{
	"latest", "recent changes", "recently changed", "up to date", "up-to-date",
	"did anything change", "has this changed", "new commits", "newest version",
	"current state", "most recent",
}

// architectureKeywords are checked to decide whether a question is asking
// about the repository's overall structure rather than a specific symbol.
// Ported verbatim from api/internal/agent/planner.go's architectureKeywords.
var architectureKeywords = []string{
	"architecture", "entrypoint", "entry point", "component", "overview",
	"structure of the repo", "repository structure", "statistics", "high level",
	"high-level", "what languages",
}

func containsAny(query string, words []string) bool {
	for _, w := range words {
		if strings.Contains(query, w) {
			return true
		}
	}
	return false
}

// WantsReingestion reports whether a question implies the user wants the
// repository re-synced against its remote before/while being answered.
// Deliberately deterministic, not LLM-classified - the phrasing patterns
// here are mechanical enough that keyword matching is reliable.
func WantsReingestion(query string) bool {
	return containsAny(strings.ToLower(query), reingestKeywords)
}

// WantsArchitectureOverview reports whether a question is asking about the
// repository's overall structure (entrypoints, components, statistics,
// languages) rather than a specific symbol's behavior.
func WantsArchitectureOverview(query string) bool {
	return containsAny(strings.ToLower(query), architectureKeywords)
}
