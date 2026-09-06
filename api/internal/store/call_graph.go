package store

import (
	"context"
	"fmt"
)

// CallGraphEdge is one hop discovered while traversing outward from a root symbol.
// CallerFilePath comes straight from call_edges (every caller row already carries
// its own file path from ingestion) - there is no equivalent callee_file_path column,
// so a symbol's file location is only known when that symbol itself shows up as a
// caller somewhere else in the traversal (see GetBoundedCallGraph's caller in
// graph_bounded.go for how that's reconciled).
type CallGraphEdge struct {
	Caller         string
	Callee         string
	CallerFilePath string
	Depth          int
}

// GetBoundedCallGraph walks the call_edges table outward from rootSymbol (what does
// rootSymbol call, and what do those calls call, and so on), scoped to a single repo,
// capped at maxDepth hops and maxEdges rows. It's a recursive CTE with an explicit
// visited-path check to stay correct in the presence of cycles (mutual recursion,
// e.g. a handler that re-enqueues itself) - Postgres will happily recurse forever
// on a cyclic graph without one.
func (s *Store) GetBoundedCallGraph(ctx context.Context, repoID, rootSymbol string, maxDepth, maxEdges int) ([]CallGraphEdge, error) {

	query := `
	WITH RECURSIVE call_graph AS (
		SELECT
			caller_symbol,
			callee_symbol,
			caller_file_path,
			1 AS depth,
			ARRAY[caller_symbol] AS visited_path
		FROM call_edges
		WHERE repo_id = $1 AND caller_symbol = $2

		UNION ALL

		SELECT
			ce.caller_symbol,
			ce.callee_symbol,
			ce.caller_file_path,
			cg.depth + 1,
			cg.visited_path || ce.caller_symbol
		FROM call_edges ce
		JOIN call_graph cg ON ce.caller_symbol = cg.callee_symbol
		WHERE ce.repo_id = $1
		  AND cg.depth < $3
		  AND NOT ce.caller_symbol = ANY(cg.visited_path)
	)
	SELECT DISTINCT caller_symbol, callee_symbol, caller_file_path, depth
	FROM call_graph
	ORDER BY depth
	LIMIT $4`

	rows, err := s.db.QueryContext(ctx, query, repoID, rootSymbol, maxDepth, maxEdges)
	if err != nil {
		return nil, fmt.Errorf("failed to query bounded call graph: %w", err)
	}
	defer rows.Close()

	var edges []CallGraphEdge

	for rows.Next() {

		var e CallGraphEdge

		if err := rows.Scan(&e.Caller, &e.Callee, &e.CallerFilePath, &e.Depth); err != nil {
			return nil, fmt.Errorf("failed to scan call graph edge: %w", err)
		}

		edges = append(edges, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate call graph edges: %w", err)
	}

	return edges, nil
}
