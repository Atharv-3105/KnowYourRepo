package store

import (
	"context"
	"fmt"
)

func(s *Store) GetOutgoingCalls(ctx context.Context,filePath string, callerSymbol string) ([]CallEdge, error){

	query := `
	SELECT DISTINCT
		caller_symbol,
		caller_file_path,
		callee_symbol
	FROM call_edges
	WHERE caller_symbol = ?
	AND caller_file_path = ?
	`

	rows, err := s.db.QueryContext(ctx, query, callerSymbol, filePath)

	if err != nil {
		return nil, err 
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.CallerSymbol, &edge.CallerFilePath, &edge.CalleeSymbol)

		if err != nil {
			fmt.Println("OUTGOING SCAN ERROR:", err)
			continue
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

func(s *Store) GetIncomingCalls(ctx context.Context, callee string) ([]CallEdge, error){

	query := `
	SELECT
		caller_symbol,
		callee_symbol
	FROM call_edges
	WHERE callee_symbol = ?
	`

	rows, err := s.db.QueryContext(ctx, query, callee)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var edges []CallEdge

	for rows.Next() {

		var edge CallEdge

		err := rows.Scan(&edge.CallerSymbol, &edge.CalleeSymbol)

		if err != nil {
			continue	
		}

		edges = append(edges, edge)
	}

	return edges, nil
}