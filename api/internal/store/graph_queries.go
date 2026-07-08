package store

import (
	"context"
	"fmt"
)

func(s *Store) GetOutgoingCalls(ctx context.Context, repoID, filePath, callerSymbol string) ([]CallEdge, error){

	query := `
	SELECT DISTINCT
		repo_id,
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE repo_id = ?
	AND caller_symbol = ?
	AND caller_file_path = ?
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
	WHERE repo_id = ?
	AND callee_symbol = ?
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