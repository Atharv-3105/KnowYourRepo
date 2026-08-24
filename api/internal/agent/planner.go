package agent 

import "strings"

//Planner will be a deterministic query classifier, It will decide which tools are relevant to a question using keyword heuristics, 
//Will be updated with an LLM-driven planner later without changing the executor

type Planner struct{}

func NewPlanner() *Planner{
	return &Planner{}
}

//Keyword maps for deterministic tool calling
var architectureKeywords = []string{
	"architecture", "entrypoint", "entry point", "component", "overview",
	"structure of the repo", "repository structure", "statistics", "high level",
	"high-level", "what languages",
}

var graphKeywords = []string{
	"who calls", "calls into", "callers of", "caller of", "what calls",
	"incoming call", "outgoing call", "call graph", "who uses", "depends on",
	"dependency", "dependencies", "breaks if", "eventually call", "eventually calls",
}

var followUpKeywords = []string{
	"it further", "that further", "explain that", "explain it", "what about",
	"compare it", "compared to", "more about", "earlier", "previously",
	"we discussed", "you said", "go on",
}

// semanticHintPhrases are checked as whole phrases (not single words) to
// avoid the planner over-triggering on common English words like "how" or
// "what" that also appear inside follow-up questions.
var semanticHintPhrases = []string{
	"how is", "how are", "how does", "how do", "what does", "what is",
	"what are", "where is", "where are", "describe the", "implementation of",
	"how it works", "how they work",
}

//Plan returns the ordered, de-duplicated list of tools the executor should run for a given query.
//Semantic-Search is the default grounding tool: it is included whenever no other tool matched
func(p *Planner) Plan(query string, history string) []ToolName{
	q := strings.ToLower(query)

	var plan []ToolName 

	//Check for architecture words present in the Query
	if containsAny(q, architectureKeywords){
		plan = append(plan, ToolArchitecture)
	}

	//Check for graph words present in the Query
	if containsAny(q, graphKeywords){
		plan = append(plan, ToolGraph)
	}

	//Check for memory words present in the Query and history is not null
	if strings.TrimSpace(history) != "" && containsAny(q, followUpKeywords){
		plan = append(plan, ToolMemory)
	}

	//Check for semantic words present in the Query or no other tool selected
	if len(plan) == 0 || containsAny(q, semanticHintPhrases){
		plan = append(plan, ToolSemantic)
	}

	return dedupeToolNames(plan)
}


func containsAny(query string, words []string) bool {
	for _, n := range words{
		if strings.Contains(query, n){
			return true
		}
	}

	return false 
}

func dedupeToolNames(names []ToolName) []ToolName{

	//Act as a Set having toolName elements
	seen := make(map[ToolName]bool, len(names))
	var out []ToolName

	for _, n := range names {
		if seen[n] {
			continue 
		}

		seen[n] = true 
		out = append(out, n)
	}

	return out 
}