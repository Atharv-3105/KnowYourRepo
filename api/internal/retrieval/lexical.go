package retrieval

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

const maxLexicalCandidates = 5
const maxLexicalResults = 5

// maxLexicalSnippetLines caps how much source readSymbolSource will return
// for one symbol - token-budget hygiene, matching the same reasoning
// applied to the overview-generation prompt elsewhere in this codebase.
const maxLexicalSnippetLines = 60

// readSymbolSource reads a symbol's real source lines from its file on
// disk (FilePath is already the absolute on-disk clone path - see the
// codebase-wide convention documented in frontend/src/lib/path.ts). Returns
// ("", false) on any failure (file missing, line range out of bounds after
// the file changed on disk, etc.) - callers fall back to the location-stub
// text rather than failing the whole lookup over one unreadable file.
func readSymbolSource(filePath string, startLine, endLine int) (string, bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(content), "\n")

	if startLine < 1 || startLine > len(lines) {
		return "", false
	}
	if endLine < startLine {
		endLine = startLine
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if endLine-startLine+1 > maxLexicalSnippetLines {
		endLine = startLine + maxLexicalSnippetLines - 1
	}

	return strings.Join(lines[startLine-1:endLine], "\n"), true
}

// dottedIdentifierPattern matches qualified names like "asyncio.create_task"
// as a single token - tried before the plain pattern below so a dotted call
// is looked up whole first, since that's exactly how call_edges.callee_symbol
// stores it for calls to functions from an imported module.
var dottedIdentifierPattern = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+\b`)
var plainIdentifierPattern = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)

// lexicalStopWords are common English words that would otherwise pass the
// bare identifier pattern - filtered out so they never turn into wasted (if
// cheap) lookups or, worse, spurious substring matches against real code.
var lexicalStopWords = map[string]bool{
	"what": true, "does": true, "did": true, "do": true, "this": true, "that": true,
	"which": true, "the": true, "and": true, "for": true, "with": true, "how": true,
	"where": true, "when": true, "who": true, "why": true, "have": true, "has": true,
	"had": true, "here": true, "there": true, "about": true, "into": true, "call": true,
	"calls": true, "called": true, "calling": true, "function": true, "method": true,
	"class": true, "repo": true, "repository": true, "code": true, "file": true,
	"line": true, "give": true, "tell": true, "explain": true, "describe": true,
	"show": true, "you": true, "your": true, "its": true, "was": true, "were": true,
	"are": true, "is": true, "of": true, "on": true, "to": true, "me": true, "recently": true,
}

// extractCandidateIdentifiers pulls identifier-looking tokens out of a raw
// natural-language question, dotted-qualified names first. It deliberately
// tries every plausible candidate (not just one "best guess") since a cheap
// indexed lookup per candidate costs far less than silently grounding on
// nothing - a false-positive candidate just returns zero rows.
func extractCandidateIdentifiers(query string) []string {
	seen := make(map[string]bool)
	var candidates []string

	add := func(tok string) {
		if len(candidates) >= maxLexicalCandidates {
			return
		}
		lower := strings.ToLower(tok)
		if len(tok) < 3 || seen[lower] || lexicalStopWords[lower] {
			return
		}
		seen[lower] = true
		candidates = append(candidates, tok)
	}

	for _, m := range dottedIdentifierPattern.FindAllString(query, -1) {
		add(m)
	}
	for _, m := range plainIdentifierPattern.FindAllString(query, -1) {
		add(m)
	}

	return candidates
}

// LexicalSearch grounds a question directly against the repo's structural
// data (symbol table + call graph) by looking up identifier-like tokens
// extracted from the raw query, exactly/by substring - no embedding call,
// so it works even when the embedding provider is unavailable, and it
// catches calls to symbols that were never locally defined (stdlib/external
// functions like "asyncio.create_task") that semantic search's ranking can
// simply miss for a plausible, correctly-phrased question. Meant to run
// unconditionally alongside Search, not as a replacement for it - see
// answer.Service.Answer.
func (r *HybridRetriever) LexicalSearch(ctx context.Context, repoID, query string) ([]RetrievalResult, error) {

	candidates := extractCandidateIdentifiers(query)
	if len(candidates) == 0 {
		return nil, nil
	}

	var results []RetrievalResult
	seen := make(map[string]bool)

	for _, candidate := range candidates {
		if len(results) >= maxLexicalResults {
			break
		}

		syms, err := r.store.ListSymbols(ctx, repoID, candidate)
		if err != nil {
			r.logger.Warn("lexical_symbol_lookup_failed", "candidate", candidate, "error", err)
		}

		// ListSymbols returns matches in file/line order, not relevance
		// order - an exact match on the searched identifier (e.g.
		// "HandleWord") is what the question actually asked about and must
		// survive the maxLexicalResults cap ahead of a merely-substring
		// match (e.g. "TestRoom_HandleWord_EmptyWordRejected"). Stable sort
		// preserves the original file/line order as the tiebreak within
		// each group.
		sort.SliceStable(syms, func(i, j int) bool {
			iExact := strings.EqualFold(syms[i].Name, candidate)
			jExact := strings.EqualFold(syms[j].Name, candidate)
			return iExact && !jExact
		})

		for _, sym := range syms {
			if len(results) >= maxLexicalResults {
				break
			}

			key := sym.Name + "|" + sym.FilePath
			if seen[key] {
				continue
			}
			seen[key] = true

			document := fmt.Sprintf("%s is a %s defined at %s, lines %d-%d.", sym.Name, sym.Type, sym.FilePath, sym.StartLine, sym.EndLine)
			if source, ok := readSymbolSource(sym.FilePath, sym.StartLine, sym.EndLine); ok {
				document = source
			}

			results = append(results, RetrievalResult{
				Symbol:   sym.Name,
				FilePath: sym.FilePath,
				Document: document,
				Metadata: map[string]interface{}{"start_line": float64(sym.StartLine), "end_line": float64(sym.EndLine)},
				Origin:   "lexical",
			})
		}

		if len(results) >= maxLexicalResults {
			break
		}

		edges, err := r.store.SearchCallEdgesByCalleeSubstring(ctx, repoID, candidate)
		if err != nil {
			r.logger.Warn("lexical_call_lookup_failed", "candidate", candidate, "error", err)
			continue
		}

		for callee, group := range groupCallEdgesByCallee(edges) {
			if len(results) >= maxLexicalResults {
				break
			}

			key := callee + "|"
			if seen[key] {
				continue
			}
			seen[key] = true

			var lines []string
			var graphEdges []GraphEdge
			for _, e := range group {
				lines = append(lines, fmt.Sprintf("%s calls %s (in %s)", e.CallerSymbol, e.CalleeSymbol, e.CallerFilePath))
				graphEdges = append(graphEdges, GraphEdge{Caller: e.CallerSymbol, Callee: e.CalleeSymbol})
			}

			results = append(results, RetrievalResult{
				Symbol:   callee,
				Document: strings.Join(lines, "\n"),
				Edges:    graphEdges,
				Origin:   "lexical",
			})
		}
	}

	r.logger.Info("lexical_search_complete", "query", query, "candidates", len(candidates), "results", len(results))

	return results, nil
}

func groupCallEdgesByCallee(edges []store.CallEdge) map[string][]store.CallEdge {
	grouped := make(map[string][]store.CallEdge)
	for _, e := range edges {
		grouped[e.CalleeSymbol] = append(grouped[e.CalleeSymbol], e)
	}
	return grouped
}
