// This file computes a suggested "where to start reading" path through a
// repo's call graph - deterministic, no LLM involved, so it's free and
// always in sync with the current index. See the design spec's Reading
// path section for the reasoning behind this vs. an LLM-generated order.
package architecture

import (
	"fmt"
	"sort"

	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type ReadingStep struct {
	Symbol   string `json:"symbol"`
	FilePath string `json:"file_path"`
	Reason   string `json:"reason"`
}

const maxReadingPathSteps = 12

// BuildReadingPath ranks symbols by how central they are to the call graph:
// BFS outward from every detected entrypoint (depth ascending), broken by
// distinct-caller count (descending) within each depth. With zero
// entrypoints (a real case - some repos have none detected), falls back to
// ranking every symbol seen as a caller by caller count, with no BFS anchor.
func BuildReadingPath(entrypoints []EntryPoint, callEdges []store.ArchitectureCallEdge) []ReadingStep {
	fileOf := make(map[string]string)
	for _, ep := range entrypoints {
		fileOf[ep.Name] = ep.FilePath
	}
	for _, e := range callEdges {
		if _, ok := fileOf[e.CallerSymbol]; !ok {
			fileOf[e.CallerSymbol] = e.CallerFilePath
		}
	}

	// Distinct-caller count per callee symbol, used as a centrality proxy -
	// a symbol called from many places is more "core" than one called once.
	callerCount := make(map[string]int)
	callersSeen := make(map[string]map[string]struct{})
	for _, e := range callEdges {
		if callersSeen[e.CalleeeSymbol] == nil {
			callersSeen[e.CalleeeSymbol] = make(map[string]struct{})
		}
		if _, dup := callersSeen[e.CalleeeSymbol][e.CallerSymbol]; !dup {
			callersSeen[e.CalleeeSymbol][e.CallerSymbol] = struct{}{}
			callerCount[e.CalleeeSymbol]++
		}
	}

	adjacency := make(map[string][]string)
	for _, e := range callEdges {
		adjacency[e.CallerSymbol] = append(adjacency[e.CallerSymbol], e.CalleeeSymbol)
	}

	type discovered struct {
		symbol string
		depth  int
	}

	visited := make(map[string]bool)
	var order []discovered

	if len(entrypoints) > 0 {
		var queue []discovered
		for _, ep := range entrypoints {
			if !visited[ep.Name] {
				visited[ep.Name] = true
				queue = append(queue, discovered{ep.Name, 0})
			}
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			order = append(order, cur)
			for _, callee := range adjacency[cur.symbol] {
				if !visited[callee] {
					visited[callee] = true
					queue = append(queue, discovered{callee, cur.depth + 1})
				}
			}
		}
	} else {
		// No entrypoints detected - rank every symbol seen as a caller,
		// no BFS anchor to depth-order them, so depth is uniform.
		for symbol := range fileOf {
			order = append(order, discovered{symbol, 1})
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		if order[i].depth != order[j].depth {
			return order[i].depth < order[j].depth
		}
		return callerCount[order[i].symbol] > callerCount[order[j].symbol]
	})

	entrypointSet := make(map[string]bool, len(entrypoints))
	for _, ep := range entrypoints {
		entrypointSet[ep.Name] = true
	}

	steps := make([]ReadingStep, 0, maxReadingPathSteps)
	for _, d := range order {
		if len(steps) >= maxReadingPathSteps {
			break
		}

		reason := fmt.Sprintf("Called by %d function(s) in the core flow", callerCount[d.symbol])
		if entrypointSet[d.symbol] {
			reason = "Entrypoint"
		}

		steps = append(steps, ReadingStep{
			Symbol:   d.symbol,
			FilePath: fileOf[d.symbol], // empty for symbols never seen as a caller (e.g. stdlib/external calls) - same convention as GraphNode.file_path
			Reason:   reason,
		})
	}

	return steps
}
