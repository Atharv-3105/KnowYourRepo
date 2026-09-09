package store

import (
	"context"
	"fmt"
)

// maxLexicalCallEdges bounds a single substring lookup so a very generic
// candidate token (a short, common word that still slipped past the
// caller's stopword filter) can't pull back an unbounded scan's worth of
// rows for one repo.
const maxLexicalCallEdges = 20

// SearchCallEdgesByCalleeSubstring finds call edges whose callee name
// contains substr, case-insensitively. Unlike GetIncomingCalls (exact
// match only), this also catches calls to symbols that were never locally
// defined - callee_symbol records the literal call-target text regardless
// of whether it resolves to a local definition, so an external/stdlib call
// like "asyncio.create_task" is findable here even though it will never
// appear in the `symbols` table at all.
func (s *Store) SearchCallEdgesByCalleeSubstring(ctx context.Context, repoID, substr string) ([]CallEdge, error) {

	query := `
	SELECT DISTINCT
		repo_id,
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE repo_id = $1
	AND callee_symbol ILIKE '%' || $2 || '%'
	LIMIT ` + fmt.Sprint(maxLexicalCallEdges)

	rows, err := s.db.QueryContext(ctx, query, repoID, substr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {
		var edge CallEdge

		if err := rows.Scan(&edge.RepoID, &edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeSymbol); err != nil {
			return nil, err
		}

		edges = append(edges, edge)
	}

	return edges, rows.Err()
}

func(s *Store) GetOutgoingCalls(ctx context.Context, repoID, filePath, callerSymbol string) ([]CallEdge, error){

	query := `
	SELECT DISTINCT
		repo_id,
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE repo_id = $1
	AND caller_symbol = $2
	AND caller_file_path = $3
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, callerSymbol, filePath)

	if err != nil {
		return nil, err 
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.RepoID, &edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeSymbol)

		if err != nil {
			fmt.Println("OUTGOING SCAN ERROR:", err)
			continue
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

func(s *Store) GetIncomingCalls(ctx context.Context, repoID, callee string) ([]CallEdge, error){

	query := `
	SELECT DISTINCT
		repo_id,
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE repo_id = $1
	AND callee_symbol = $2
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, callee)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.RepoID, &edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeSymbol)

		if err != nil {
			continue	
		}

		edges = append(edges, edge)
	}

	return edges, nil
}



func(s *Store) GetOutgoingCallsBySymbol(ctx context.Context, repoID, callerSymbol string) ([]CallEdge, error) {

	query := `
	SELECT DISTINCT
		repo_id,
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE repo_id = $1
	AND caller_symbol = $2
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, callerSymbol)
	if err != nil {
		return nil, err 
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next(){

		var edge CallEdge 

		err := rows.Scan(&edge.RepoID, &edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeSymbol)

		if err != nil {
			continue 
		}

		edges = append(edges, edge)
	}

	return edges, nil 
}

