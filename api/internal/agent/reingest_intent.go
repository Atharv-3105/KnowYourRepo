package agent

import "strings"

// freshnessKeywords are checked to decide whether a question implies the
// user wants the repository's current/latest state, independent of which
// tools get selected to answer it.
var reingestKeywords = []string{
	"latest", "recent changes", "recently changed", "up to date", "up-to-date",
	"did anything change", "has this changed", "new commits", "newest version",
	"current state", "most recent",
}

// WantsReingestion reports whether a question implies the user wants the
// repository re-synced against its remote before/while being answered.
// Deliberately deterministic, not LLM-classified - the phrasing patterns
// here are mechanical enough that keyword matching is reliable, and it
// avoids adding a second, LLM-dependent path for what's really a binary
// signal.
func WantsReingestion(query string) bool {
	return containsAny(strings.ToLower(query), reingestKeywords)
}